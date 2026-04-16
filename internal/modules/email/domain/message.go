package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrMessageNotFound = errors.New("email: message not found")
	ErrThreadNotFound  = errors.New("email: thread not found")
)

// EmailThread groups related messages by References/In-Reply-To or subject.
type EmailThread struct {
	ID                   string
	AccountID            string
	Subject              string
	ParticipantAddresses string // comma-joined unique From/To across messages
	CustomerID           string // nullable — linked customer
	MessageCount         int
	LastMessageAt        time.Time
	CreatedAt            time.Time
}

// EmailMessage is a single fetched IMAP message.
type EmailMessage struct {
	ID         string
	AccountID  string
	ThreadID   string
	UID        uint32 // IMAP UID (folder-scoped)
	Folder     string
	MessageID  string // RFC 2822 Message-ID header value
	References string // space-joined References header
	InReplyTo  string
	Subject    string
	FromAddr   string
	FromName   string
	ToAddrs    string // comma-joined To addresses
	TextBody   string // plain text part, decoded to UTF-8
	HTMLBody   string // HTML part, decoded to UTF-8
	ReceivedAt time.Time
	FetchedAt  time.Time
}

// EmailAttachment is a MIME attachment part of a message.
type EmailAttachment struct {
	ID        string
	MessageID string
	Filename  string
	MIMEType  string
	Size      int64
	Data      []byte // nil if size > threshold (not downloaded)
	CreatedAt time.Time
}

// NormalizeSubject strips Re:/Fwd: prefixes for thread matching.
func NormalizeSubject(s string) string {
	s = strings.TrimSpace(s)
	for {
		lower := strings.ToLower(s)
		switch {
		case strings.HasPrefix(lower, "re: "):
			s = strings.TrimSpace(s[4:])
		case strings.HasPrefix(lower, "fwd: "):
			s = strings.TrimSpace(s[5:])
		case strings.HasPrefix(lower, "fw: "):
			s = strings.TrimSpace(s[4:])
		default:
			return s
		}
	}
}

// ReferenceIDs returns individual Message-IDs from the References header.
func (m *EmailMessage) ReferenceIDs() []string {
	if m.References == "" {
		return nil
	}
	parts := strings.Fields(m.References)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
