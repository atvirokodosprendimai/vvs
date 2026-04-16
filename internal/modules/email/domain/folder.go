package domain

import (
	"errors"
	"time"
)

var ErrFolderNotFound = errors.New("email: folder not found")

// EmailFolder tracks sync state for one IMAP mailbox folder within an account.
type EmailFolder struct {
	ID        string
	AccountID string
	Name      string // IMAP mailbox name, e.g. "INBOX", "Sent"
	LastUID   uint32 // incremental sync cursor (per-folder — IMAP UIDs are mailbox-scoped)
	Enabled   bool   // false = skip during sync
	CreatedAt time.Time
}
