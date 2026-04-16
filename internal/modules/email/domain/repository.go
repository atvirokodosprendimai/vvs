package domain

import (
	"context"
	"time"
)

// EmailAccountRepository is the port for account persistence.
type EmailAccountRepository interface {
	Save(ctx context.Context, a *EmailAccount) error
	FindByID(ctx context.Context, id string) (*EmailAccount, error)
	ListActive(ctx context.Context) ([]*EmailAccount, error)
	List(ctx context.Context) ([]*EmailAccount, error)
	Delete(ctx context.Context, id string) error
}

// EmailThreadRepository is the port for thread persistence.
type EmailThreadRepository interface {
	Save(ctx context.Context, t *EmailThread) error
	FindByID(ctx context.Context, id string) (*EmailThread, error)
	// FindByMessageID returns the thread containing a message with the given RFC 2822 Message-ID.
	FindByMessageID(ctx context.Context, accountID, messageID string) (*EmailThread, error)
	// FindBySubject returns a thread with matching normalized subject, or ErrThreadNotFound.
	FindBySubject(ctx context.Context, accountID, normalizedSubject string) (*EmailThread, error)
	ListAll(ctx context.Context) ([]*EmailThread, error)
	ListForAccount(ctx context.Context, accountID string) ([]*EmailThread, error)
	ListForCustomer(ctx context.Context, customerID string) ([]*EmailThread, error)
}

// EmailMessageRepository is the port for message persistence.
type EmailMessageRepository interface {
	Save(ctx context.Context, m *EmailMessage) error
	FindByUID(ctx context.Context, accountID, folder string, uid uint32) (*EmailMessage, error)
	FindByMessageID(ctx context.Context, accountID, messageID string) (*EmailMessage, error)
	ListForThread(ctx context.Context, threadID string) ([]*EmailMessage, error)
}

// AttachmentSearchRow is a single result from an attachment search.
type AttachmentSearchRow struct {
	ID            string
	Filename      string
	MIMEType      string
	Size          int64
	ThreadID      string
	ThreadSubject string
	FromAddr      string
	ReceivedAt    time.Time
}

// EmailAttachmentRepository is the port for attachment persistence.
type EmailAttachmentRepository interface {
	Save(ctx context.Context, a *EmailAttachment) error
	FindByID(ctx context.Context, id string) (*EmailAttachment, error)
	ListForMessage(ctx context.Context, messageID string) ([]*EmailAttachment, error)
	// SearchByFilename returns attachments whose filename contains query, scoped to accountID.
	SearchByFilename(ctx context.Context, accountID, query string) ([]*AttachmentSearchRow, error)
}

// EmailFolderRepository is the port for per-account folder sync state.
type EmailFolderRepository interface {
	Save(ctx context.Context, f *EmailFolder) error
	ListForAccount(ctx context.Context, accountID string) ([]*EmailFolder, error)
	FindByAccountAndName(ctx context.Context, accountID, name string) (*EmailFolder, error)
	// ListThreadIDsWithFolder returns thread IDs that have at least one message in the given folder.
	ListThreadIDsWithFolder(ctx context.Context, accountID, folder string) ([]string, error)
}

// EmailTagRepository is the port for tag persistence.
type EmailTagRepository interface {
	Save(ctx context.Context, t *EmailTag) error
	FindByID(ctx context.Context, id string) (*EmailTag, error)
	ListAll(ctx context.Context) ([]*EmailTag, error)
	ListForThread(ctx context.Context, threadID string) ([]*EmailTag, error)
	ApplyToThread(ctx context.Context, tt EmailThreadTag) error
	RemoveFromThread(ctx context.Context, threadID, tagID string) error
	// FindSystemTag returns the system tag with the given name (creates if missing).
	FindSystemTag(ctx context.Context, name string) (*EmailTag, error)
}
