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
	Threads     *persistence.GormEmailThreadRepository
	Messages    *persistence.GormEmailMessageRepository
	Attachments *persistence.GormEmailAttachmentRepository
	Tags        *persistence.GormEmailTagRepository
	EncKey      []byte // AES-256 key for decrypting stored passwords (nil = plaintext)
}

// NewIDFunc generates a new unique string ID.
type NewIDFunc func() string

// Fetch connects to the IMAP server for the given account, fetches new messages
// (UID > account.LastUID), stores them, and updates the account sync state.
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

	// EXAMINE = readonly select.
	if _, err := c.Select(account.Folder, &imaplib.SelectOptions{ReadOnly: true}).Wait(); err != nil {
		return fmt.Errorf("imap select %q: %w", account.Folder, err)
	}

	// Build UID search criteria for UIDs > lastUID.
	criteria := &imaplib.SearchCriteria{}
	if account.LastUID > 0 {
		var uidSet imaplib.UIDSet
		uidSet.AddRange(imaplib.UID(account.LastUID+1), 0) // 0 = * (all remaining)
		criteria.UID = []imaplib.UIDSet{uidSet}
	}

	searchData, err := c.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return fmt.Errorf("imap search: %w", err)
	}

	uidSet, ok := searchData.All.(imaplib.UIDSet)
	if !ok {
		slog.Debug("imap: no new messages", "account", account.Name)
		return nil
	}
	allUIDs, valid := uidSet.Nums()
	if !valid || len(allUIDs) == 0 {
		slog.Debug("imap: no new messages", "account", account.Name)
		return nil
	}

	slog.Info("imap: fetching new messages", "account", account.Name, "count", len(allUIDs), "last_uid", account.LastUID)

	// Fetch in batches of 50.
	const batchSize = 50
	var maxUID uint32
	fetched := 0

	for i := 0; i < len(allUIDs); i += batchSize {
		end := i + batchSize
		if end > len(allUIDs) {
			end = len(allUIDs)
		}
		batch := allUIDs[i:end]

		slog.Debug("imap: fetching batch", "account", account.Name, "batch_start", i+1, "batch_end", end, "total", len(allUIDs))

		var fetchSet imaplib.UIDSet
		for _, uid := range batch {
			fetchSet.AddNum(uid)
			if uint32(uid) > maxUID {
				maxUID = uint32(uid)
			}
		}

		fetchCmd := c.Fetch(fetchSet, &imaplib.FetchOptions{
			UID:          true,
			Envelope:     true,
			InternalDate: true,
			BodySection:  []*imaplib.FetchItemBodySection{{Peek: true}}, // BODY.PEEK[] — never sets \Seen
		})

		for {
			msg := fetchCmd.Next()
			if msg == nil {
				break
			}
			buf, err := msg.Collect()
			if err != nil {
				slog.Error("imap: fetch collect", "account", account.Name, "seq", msg.SeqNum, "err", err)
				continue
			}
			slog.Debug("imap: processing message", "account", account.Name, "uid", buf.UID)
			if err := processMessage(ctx, account, buf, repos, newID); err != nil {
				slog.Error("imap: process message", "account", account.Name, "uid", buf.UID, "err", err)
			}
			fetched++
		}
		if err := fetchCmd.Close(); err != nil {
			return fmt.Errorf("imap fetch close: %w", err)
		}
	}

	if maxUID > account.LastUID {
		account.MarkSynced(maxUID)
		if err := repos.Accounts.Save(ctx, account); err != nil {
			return fmt.Errorf("save account: %w", err)
		}
	}

	if fetched > 0 {
		slog.Info("imap: sync complete", "account", account.Name, "fetched", fetched, "max_uid", maxUID)
		if pub != nil {
			_ = pub.Publish(ctx, "isp.email.synced", events.DomainEvent{
				Type:        "email.synced",
				AggregateID: account.ID,
				OccurredAt:  time.Now().UTC(),
				Data:        []byte(fmt.Sprintf(`{"count":%d}`, fetched)),
			})
		}
	} else {
		slog.Debug("imap: no new messages fetched", "account", account.Name)
	}
	return nil
}

func processMessage(
	ctx context.Context,
	account *domain.EmailAccount,
	buf *imapclient.FetchMessageBuffer,
	repos Repos,
	newID NewIDFunc,
) error {
	if buf.Envelope == nil {
		return nil
	}

	// Skip if already stored — guards against LastUID desync re-processing the
	// same UIDs, which would re-apply the unread tag and undo MarkRead.
	if _, err := repos.Messages.FindByUID(ctx, account.ID, uint32(buf.UID)); err == nil {
		slog.Debug("imap: message already stored, skipping", "account", account.Name, "uid", buf.UID)
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
		Folder:     account.Folder,
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

	// Re-resolve the message ID: if the row was INSERT OR IGNORE'd (duplicate UID),
	// the existing row has a different ID — use that for attachments to avoid FK errors.
	if existing, err := repos.Messages.FindByUID(ctx, account.ID, msg.UID); err == nil {
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

	// Apply "unread" system tag to thread.
	if unreadTag, err := repos.Tags.FindSystemTag(ctx, domain.TagUnread); err == nil {
		_ = repos.Tags.ApplyToThread(ctx, domain.EmailThreadTag{ThreadID: threadID, TagID: unreadTag.ID})
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
