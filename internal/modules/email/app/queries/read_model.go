package queries

import "time"

// ThreadReadModel is the flattened view for the thread list.
type ThreadReadModel struct {
	ID                   string
	AccountID            string
	AccountName          string
	Subject              string
	ParticipantAddresses string
	CustomerID           string
	MessageCount         int
	LastMessageAt        time.Time
	PreviewText          string // first ~120 chars of last message body
	Tags                 []TagReadModel
	Unread               bool
}

// ThreadDetailReadModel is a thread with all its messages and attachments.
type ThreadDetailReadModel struct {
	ThreadReadModel
	Messages []MessageReadModel
}

// MessageReadModel is a single message in a thread detail view.
type MessageReadModel struct {
	ID          string
	ThreadID    string
	MessageID   string
	Direction   string // "in" | "out"
	Subject     string
	FromAddr    string
	FromName    string
	ToAddrs     string
	TextBody    string
	HTMLBody    string
	ReceivedAt  time.Time
	Attachments []AttachmentReadModel
}

// AttachmentReadModel is metadata for a message attachment.
type AttachmentReadModel struct {
	ID       string
	Filename string
	MIMEType string
	Size     int64
}

// AccountReadModel is the view model for an email account.
type AccountReadModel struct {
	ID         string
	Name       string
	Host       string
	Port       int
	Username   string
	TLS        string
	Folder     string
	Status     string
	LastError  string
	LastSyncAt time.Time
	SMTPHost   string
	SMTPPort   int
	SMTPTLS    string
}

// FolderReadModel is the view model for a per-account IMAP folder.
type FolderReadModel struct {
	ID        string
	Name      string
	LastUID   uint32
	Enabled   bool
}

// TagReadModel is the view model for a tag.
type TagReadModel struct {
	ID     string
	Name   string
	Color  string
	System bool
}

// AttachmentSearchRow is a single result from an attachment filename search.
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

// DefaultPageSize is the number of threads per page.
const DefaultPageSize = 50

// ThreadListResult is the paginated thread list response.
type ThreadListResult struct {
	Threads  []ThreadReadModel
	Total    int
	Page     int
	PageSize int
}
