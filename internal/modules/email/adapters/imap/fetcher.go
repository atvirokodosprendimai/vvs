package imap

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
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
}

// NewIDFunc generates a new unique string ID.
type NewIDFunc func() string

// Fetch connects to the IMAP server for the given account, fetches new messages
// (UID > account.LastUID), stores them, and updates the account sync state.
func Fetch(ctx context.Context, account *domain.EmailAccount, repos Repos, newID NewIDFunc, pub events.EventPublisher) error {
	c, err := dial(account)
	if err != nil {
		return fmt.Errorf("imap dial: %w", err)
	}
	defer c.Close()

	if err := c.Login(account.Username, string(account.PasswordEnc)).Wait(); err != nil {
		return fmt.Errorf("imap login: %w", err)
	}

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
		return nil
	}
	allUIDs, valid := uidSet.Nums()
	if !valid || len(allUIDs) == 0 {
		return nil
	}

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
			BodySection:  []*imaplib.FetchItemBodySection{{}}, // full body
		})

		for {
			msg := fetchCmd.Next()
			if msg == nil {
				break
			}
			buf, err := msg.Collect()
			if err != nil {
				log.Printf("imap fetch collect seq=%v: %v", msg.SeqNum, err)
				continue
			}
			if err := processMessage(ctx, account, buf, repos, newID); err != nil {
				log.Printf("imap process message uid=%v: %v", buf.UID, err)
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

	if fetched > 0 && pub != nil {
		_ = pub.Publish(ctx, "isp.email.synced", events.DomainEvent{
			Type:        "email.synced",
			AggregateID: account.ID,
			OccurredAt:  time.Now().UTC(),
			Data:        []byte(fmt.Sprintf(`{"count":%d}`, fetched)),
		})
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
	env := buf.Envelope

	// Build domain message.
	msg := &domain.EmailMessage{
		ID:         newID(),
		AccountID:  account.ID,
		UID:        uint32(buf.UID),
		Folder:     account.Folder,
		MessageID:  env.MessageID,
		InReplyTo:  strings.Join(env.InReplyTo, " "),
		Subject:    env.Subject,
		FromAddr:   addrString(env.From),
		FromName:   addrName(env.From),
		ToAddrs:    addrListString(env.To),
		ReceivedAt: buf.InternalDate,
		FetchedAt:  time.Now().UTC(),
	}
	if msg.ReceivedAt.IsZero() {
		msg.ReceivedAt = env.Date
	}

	// Extract text/html from fetched body bytes.
	for _, section := range buf.BodySection {
		if len(section.Bytes) == 0 {
			continue
		}
		text, html := extractTextHTML(section.Bytes)
		if text != "" {
			msg.TextBody = text
		}
		if html != "" {
			msg.HTMLBody = html
		}
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

	// Update thread stats and participants.
	if err := persistence.UpdateThreadStats(ctx, repos.DB, threadID); err != nil {
		log.Printf("update thread stats %s: %v", threadID, err)
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
