package domain

import (
	"errors"
	"time"
)

// Account status values.
const (
	AccountStatusActive = "active"
	AccountStatusPaused = "paused"
	AccountStatusError  = "error"
)

// TLS mode values.
const (
	TLSNone     = "none"
	TLSStartTLS = "starttls"
	TLSTLS      = "tls"
)

var (
	ErrAccountNotFound   = errors.New("email: account not found")
	ErrAccountNameEmpty  = errors.New("email: account name required")
	ErrAccountHostEmpty  = errors.New("email: host required")
	ErrAccountUserEmpty  = errors.New("email: username required")
)

// EmailAccount is the aggregate for an IMAP mail account.
type EmailAccount struct {
	ID          string
	Name        string
	Host        string
	Port        int
	Username    string
	PasswordEnc []byte // AES-256-GCM encrypted
	TLS         string // none | starttls | tls
	Folder      string // IMAP folder to watch, default "INBOX"
	Status      string // active | paused | error
	LastError   string
	LastSyncAt  time.Time
	LastUID     uint32 // last fetched IMAP UID for incremental sync
	// SMTP outbound settings
	SMTPHost   string // empty = use IMAP Host
	SMTPPort   int    // 0 = use 587
	SMTPTLS    string // none | starttls | tls (default "starttls")
	SentFolder string // IMAP folder for sent messages, default "Sent"
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewEmailAccount creates an EmailAccount with defaults.
func NewEmailAccount(id, name, host string, port int, username string, passwordEnc []byte, tls, folder string) (*EmailAccount, error) {
	if name == "" {
		return nil, ErrAccountNameEmpty
	}
	if host == "" {
		return nil, ErrAccountHostEmpty
	}
	if username == "" {
		return nil, ErrAccountUserEmpty
	}
	if folder == "" {
		folder = "INBOX"
	}
	if tls == "" {
		tls = TLSTLS
	}
	now := time.Now().UTC()
	return &EmailAccount{
		ID:          id,
		Name:        name,
		Host:        host,
		Port:        port,
		Username:    username,
		PasswordEnc: passwordEnc,
		TLS:         tls,
		Folder:      folder,
		Status:      AccountStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// EffectiveSentFolder returns SentFolder or "Sent" if unset.
func (a *EmailAccount) EffectiveSentFolder() string {
	if a.SentFolder != "" {
		return a.SentFolder
	}
	return "Sent"
}

// SetError marks the account as errored with the given message.
func (a *EmailAccount) SetError(msg string) {
	a.Status = AccountStatusError
	a.LastError = msg
	a.UpdatedAt = time.Now().UTC()
}

// MarkSynced records a successful sync up to the given UID.
func (a *EmailAccount) MarkSynced(lastUID uint32) {
	a.Status = AccountStatusActive
	a.LastError = ""
	a.LastUID = lastUID
	a.LastSyncAt = time.Now().UTC()
	a.UpdatedAt = a.LastSyncAt
}

// Pause pauses the account (no further sync).
func (a *EmailAccount) Pause() {
	a.Status = AccountStatusPaused
	a.UpdatedAt = time.Now().UTC()
}

// Resume resumes a paused or errored account.
func (a *EmailAccount) Resume() {
	a.Status = AccountStatusActive
	a.LastError = ""
	a.UpdatedAt = time.Now().UTC()
}
