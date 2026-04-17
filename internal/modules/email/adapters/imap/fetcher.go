package imap

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"strings"
	"time"

	imaplib "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/charset"
	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/modules/email/adapters/persistence"
	"github.com/vvs/isp/internal/modules/email/domain"
	"github.com/vvs/isp/internal/shared/events"
)

// Repos bundles all repositories and the shared DB needed by the fetcher.
type Repos struct {
	DB          *gormsqlite.DB
	Accounts    *persistence.GormEmailAccountRepository
	Folders     *persistence.GormEmailFolderRepository
	Threads     *persistence.GormEmailThreadRepository
	Messages    *persistence.GormEmailMessageRepository
	Attachments *persistence.GormEmailAttachmentRepository
	Tags        *persistence.GormEmailTagRepository
	EncKey      []byte // AES-256 key for decrypting stored passwords (nil = plaintext)
}

// NewIDFunc generates a new unique string ID.
type NewIDFunc func() string

// Fetch connects to the IMAP server, syncs all enabled folders for the account,
// and updates per-folder LastUID cursors.
func Fetch(ctx context.Context, account *domain.EmailAccount, repos Repos, newID NewIDFunc, pub events.EventPublisher) error {
	slog.Debug("imap: connecting", "account", account.Name, "host", account.Host, "port", account.Port)
	c, err := dial(account)
	if err != nil {
		return fmt.Errorf("imap dial: %w", err)
	}
	defer c.Close()

	password, err := decryptPassword(repos.EncKey, account.PasswordEnc)
	if err != nil {
		return fmt.Errorf("imap decrypt password: %w", err)
	}
	if err := c.Login(account.Username, string(password)).Wait(); err != nil {
		return fmt.Errorf("imap login: %w", err)
	}
	slog.Debug("imap: logged in", "account", account.Name, "user", account.Username)

	// Load configured folders. If none exist yet, auto-discover from server.
	folders, err := repos.Folders.ListForAccount(ctx, account.ID)
	if err != nil {
		return fmt.Errorf("imap: list folders: %w", err)
	}
	if len(folders) == 0 {
		slog.Info("imap: no folders configured, auto-discovering", "account", account.Name)
		discovered, discErr := DiscoverFolders(ctx, account, repos, newID)
		if discErr != nil {
			slog.Error("imap: auto-discover failed, falling back to configured folder", "account", account.Name, "err", discErr)
			seed := &domain.EmailFolder{
				ID:        newID(),
				AccountID: account.ID,
				Name:      account.Folder,
				LastUID:   account.LastUID,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			}
			if err := repos.Folders.Save(ctx, seed); err != nil {
				slog.Error("imap: seed folder", "account", account.Name, "err", err)
			}
			folders = []*domain.EmailFolder{seed}
		} else {
			folders = discovered
		}
	}

	totalFetched := 0
	for _, folder := range folders {
		if !folder.Enabled {
			continue
		}
		n, err := fetchFolder(ctx, c, account, folder, repos, newID)
		if err != nil {
			slog.Error("imap: folder sync failed", "account", account.Name, "folder", folder.Name, "err", err)
			continue
		}
		totalFetched += n
	}

	// Mark account synced (status + timestamp) if everything went fine.
	account.MarkSynced(account.LastUID) // keep LastUID unchanged; per-folder cursors are in Folders table
	if err := repos.Accounts.Save(ctx, account); err != nil {
		return fmt.Errorf("save account: %w", err)
	}

	if totalFetched > 0 {
		slog.Info("imap: sync complete", "account", account.Name, "fetched", totalFetched)
		if pub != nil {
			_ = pub.Publish(ctx, events.EmailSynced.String(), events.DomainEvent{
				Type:        "email.synced",
				AggregateID: account.ID,
				OccurredAt:  time.Now().UTC(),
				Data:        []byte(fmt.Sprintf(`{"count":%d}`, totalFetched)),
			})
		}
	} else {
		slog.Debug("imap: no new messages fetched", "account", account.Name)
	}
	return nil
}

// fetchFolder syncs a single IMAP folder and returns the number of new messages fetched.
func fetchFolder(
	ctx context.Context,
	c *imapclient.Client,
	account *domain.EmailAccount,
	folder *domain.EmailFolder,
	repos Repos,
	newID NewIDFunc,
) (int, error) {
	if _, err := c.Select(folder.Name, &imaplib.SelectOptions{ReadOnly: true}).Wait(); err != nil {
		return 0, fmt.Errorf("imap select %q: %w", folder.Name, err)
	}

	criteria := &imaplib.SearchCriteria{}
	if folder.LastUID > 0 {
		var uidSet imaplib.UIDSet
		uidSet.AddRange(imaplib.UID(folder.LastUID+1), 0) // 0 = *
		criteria.UID = []imaplib.UIDSet{uidSet}
	}

	searchData, err := c.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return 0, fmt.Errorf("imap search: %w", err)
	}

	uidSet, ok := searchData.All.(imaplib.UIDSet)
	if !ok {
		slog.Debug("imap: no new messages", "account", account.Name, "folder", folder.Name)
		return 0, nil
	}
	allUIDs, valid := uidSet.Nums()
	if !valid || len(allUIDs) == 0 {
		slog.Debug("imap: no new messages", "account", account.Name, "folder", folder.Name)
		return 0, nil
	}

	slog.Info("imap: fetching new messages", "account", account.Name, "folder", folder.Name, "count", len(allUIDs), "last_uid", folder.LastUID)

	const batchSize = 50
	var maxUID uint32
	fetched := 0

	for i := 0; i < len(allUIDs); i += batchSize {
		end := i + batchSize
		if end > len(allUIDs) {
			end = len(allUIDs)
		}
		batch := allUIDs[i:end]

		var fetchSet imaplib.UIDSet
		for _, uid := range batch {
			fetchSet.AddNum(uid)
			if uint32(uid) > maxUID {
				maxUID = uint32(uid)
			}
		}

		fetchCmd := c.Fetch(fetchSet, &imaplib.FetchOptions{
			UID:          true,
			Flags:        true, // fetch current \Seen state before body access
			Envelope:     true,
			InternalDate: true,
			BodySection:  []*imaplib.FetchItemBodySection{{Peek: true}}, // BODY.PEEK[] — avoids \Seen on compliant servers
		})

		// Track UIDs that were NOT already \Seen — only those need flag restoration.
		var unreadUIDs imaplib.UIDSet
		unreadCount := 0

		for {
			msg := fetchCmd.Next()
			if msg == nil {
				break
			}
			buf, err := msg.Collect()
			if err != nil {
				slog.Error("imap: fetch collect", "account", account.Name, "folder", folder.Name, "seq", msg.SeqNum, "err", err)
				continue
			}

			wasSeen := false
			for _, f := range buf.Flags {
				if f == imaplib.FlagSeen {
					wasSeen = true
					break
				}
			}
			if !wasSeen {
				unreadUIDs.AddNum(buf.UID)
				unreadCount++
			}

			slog.Debug("imap: processing message", "account", account.Name, "folder", folder.Name, "uid", buf.UID, "seen", wasSeen)
			if err := processMessage(ctx, account, folder.Name, buf, wasSeen, repos, newID); err != nil {
				slog.Error("imap: process message", "account", account.Name, "folder", folder.Name, "uid", buf.UID, "err", err)
			}
			fetched++
		}
		if err := fetchCmd.Close(); err != nil {
			return fetched, fmt.Errorf("imap fetch close: %w", err)
		}

		// Restore \Seen=false only on messages that were unread before our fetch.
		// This undoes any accidental \Seen set by non-compliant servers, without
		// touching messages the user had already read in their external client.
		if unreadCount > 0 {
			storeCmd := c.Store(unreadUIDs, &imaplib.StoreFlags{
				Op:     imaplib.StoreFlagsDel,
				Silent: true,
				Flags:  []imaplib.Flag{imaplib.FlagSeen},
			}, nil)
			if err := storeCmd.Close(); err != nil {
				slog.Debug("imap: restore unread flags", "folder", folder.Name, "err", err)
			}
		}
	}

	if maxUID > folder.LastUID {
		folder.LastUID = maxUID
		if err := repos.Folders.Save(ctx, folder); err != nil {
			slog.Error("imap: save folder last_uid", "account", account.Name, "folder", folder.Name, "err", err)
		}
	}

	return fetched, nil
}

// DiscoverFolders connects to IMAP, lists all mailboxes, and upserts them into the
// email_account_folders table (enabled=true if new, preserves existing enabled state).
func DiscoverFolders(ctx context.Context, account *domain.EmailAccount, repos Repos, newID NewIDFunc) ([]*domain.EmailFolder, error) {
	c, err := dial(account)
	if err != nil {
		return nil, fmt.Errorf("imap dial: %w", err)
	}
	defer c.Close()

	password, err := decryptPassword(repos.EncKey, account.PasswordEnc)
	if err != nil {
		return nil, fmt.Errorf("imap decrypt password: %w", err)
	}
	if err := c.Login(account.Username, string(password)).Wait(); err != nil {
		return nil, fmt.Errorf("imap login: %w", err)
	}

	listCmd := c.List("", "*", nil)
	var names []string
	for {
		mb := listCmd.Next()
		if mb == nil {
			break
		}
		names = append(names, mb.Mailbox)
	}
	if err := listCmd.Close(); err != nil {
		return nil, fmt.Errorf("imap list close: %w", err)
	}

	var result []*domain.EmailFolder
	for _, name := range names {
		existing, err := repos.Folders.FindByAccountAndName(ctx, account.ID, name)
		if err == nil {
			result = append(result, existing)
			continue
		}
		f := &domain.EmailFolder{
			ID:        newID(),
			AccountID: account.ID,
			Name:      name,
			LastUID:   0,
			Enabled:   true,
			CreatedAt: time.Now().UTC(),
		}
		if err := repos.Folders.Save(ctx, f); err != nil {
			slog.Error("imap: save discovered folder", "name", name, "err", err)
			continue
		}
		result = append(result, f)
	}
	slog.Info("imap: discovered folders", "account", account.Name, "count", len(result))
	return result, nil
}

func processMessage(
	ctx context.Context,
	account *domain.EmailAccount,
	folderName string,
	buf *imapclient.FetchMessageBuffer,
	wasSeen bool, // true if \Seen was already set on IMAP server before our fetch
	repos Repos,
	newID NewIDFunc,
) error {
	if buf.Envelope == nil {
		return nil
	}

	// Skip if already stored — guards against LastUID desync re-processing the
	// same UIDs, which would re-apply the unread tag and undo MarkRead.
	if _, err := repos.Messages.FindByUID(ctx, account.ID, folderName, uint32(buf.UID)); err == nil {
		slog.Debug("imap: message already stored, skipping", "account", account.Name, "folder", folderName, "uid", buf.UID)
		return nil
	}

	env := buf.Envelope

	// Parse body: text, html, references header, attachments.
	var parsed ParsedMessage
	for _, section := range buf.BodySection {
		if len(section.Bytes) > 0 {
			parsed = ParseMessage(section.Bytes)
			break
		}
	}

	// Build domain message.
	msg := &domain.EmailMessage{
		ID:         newID(),
		AccountID:  account.ID,
		UID:        uint32(buf.UID),
		Folder:     folderName,
		MessageID:  env.MessageID,
		References: parsed.References,
		InReplyTo:  strings.Join(env.InReplyTo, " "),
		Subject:    env.Subject,
		FromAddr:   addrString(env.From),
		FromName:   addrName(env.From),
		ToAddrs:    addrListString(env.To),
		TextBody:   parsed.Text,
		HTMLBody:   parsed.HTML,
		ReceivedAt: buf.InternalDate,
		FetchedAt:  time.Now().UTC(),
	}
	if msg.ReceivedAt.IsZero() {
		msg.ReceivedAt = env.Date
	}

	// Thread assignment.
	threadID, err := Assign(ctx, msg, repos.Threads, repos.Messages, newID)
	if err != nil {
		return fmt.Errorf("assign thread: %w", err)
	}
	msg.ThreadID = threadID

	if err := repos.Messages.Save(ctx, msg); err != nil {
		return fmt.Errorf("save message: %w", err)
	}

	// Re-resolve the message ID (covers any unlikely duplicate).
	if existing, err := repos.Messages.FindByUID(ctx, account.ID, folderName, msg.UID); err == nil {
		msg.ID = existing.ID
	}

	// Persist attachment records.
	for _, a := range parsed.Attachments {
		att := &domain.EmailAttachment{
			ID:        newID(),
			MessageID: msg.ID,
			Filename:  a.Filename,
			MIMEType:  a.MIMEType,
			Size:      int64(len(a.Data)),
			Data:      a.Data,
		}
		if err := repos.Attachments.Save(ctx, att); err != nil {
			slog.Error("imap: save attachment", "filename", a.Filename, "err", err)
		}
	}

	// Update thread stats and participants.
	if err := persistence.UpdateThreadStats(ctx, repos.DB, threadID); err != nil {
		slog.Error("imap: update thread stats", "thread_id", threadID, "err", err)
	}
	if msg.FromAddr != "" {
		_ = persistence.AddParticipant(ctx, repos.DB, threadID, msg.FromAddr)
	}

	// Apply "unread" tag only if the message was not already read on the IMAP server.
	// Respects \Seen flag set by external email clients.
	if !wasSeen {
		if unreadTag, err := repos.Tags.FindSystemTag(ctx, domain.TagUnread); err == nil {
			_ = repos.Tags.ApplyToThread(ctx, domain.EmailThreadTag{ThreadID: threadID, TagID: unreadTag.ID})
		}
	}

	return nil
}

func addrString(addrs []imaplib.Address) string {
	if len(addrs) == 0 {
		return ""
	}
	return addrs[0].Addr()
}

func addrName(addrs []imaplib.Address) string {
	if len(addrs) == 0 {
		return ""
	}
	return addrs[0].Name
}

func addrListString(addrs []imaplib.Address) string {
	parts := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if addr := a.Addr(); addr != "" {
			parts = append(parts, addr)
		}
	}
	return strings.Join(parts, ",")
}

// decryptPassword decrypts an AES-256-GCM ciphertext (nonce prepended).
// If key is empty, ciphertext is returned as-is (dev/plaintext mode).
func decryptPassword(key, ciphertext []byte) ([]byte, error) {
	if len(key) == 0 {
		return ciphertext, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("email: ciphertext too short")
	}
	return gcm.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], nil)
}

func dial(account *domain.EmailAccount) (*imapclient.Client, error) {
	opts := &imapclient.Options{
		WordDecoder: &mime.WordDecoder{CharsetReader: charset.Reader},
	}
	addr := fmt.Sprintf("%s:%d", account.Host, account.Port)
	switch account.TLS {
	case domain.TLSTLS:
		return imapclient.DialTLS(addr, opts)
	case domain.TLSStartTLS:
		return imapclient.DialStartTLS(addr, opts)
	case domain.TLSNone:
		return imapclient.DialInsecure(addr, opts)
	default:
		return imapclient.DialTLS(addr, &imapclient.Options{
			TLSConfig:   &tls.Config{ServerName: account.Host},
			WordDecoder: &mime.WordDecoder{CharsetReader: charset.Reader},
		})
	}
}
