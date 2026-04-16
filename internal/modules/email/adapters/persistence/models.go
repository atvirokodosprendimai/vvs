package persistence

import "time"

// --- GORM models ---

type accountModel struct {
	ID          string    `gorm:"primaryKey;column:id"`
	Name        string    `gorm:"column:name"`
	Host        string    `gorm:"column:host"`
	Port        int       `gorm:"column:port"`
	Username    string    `gorm:"column:username"`
	PasswordEnc []byte    `gorm:"column:password_enc"`
	TLS         string    `gorm:"column:tls"`
	Folder      string    `gorm:"column:folder"`
	Status      string    `gorm:"column:status"`
	LastError   string    `gorm:"column:last_error"`
	LastSyncAt  time.Time `gorm:"column:last_sync_at"`
	LastUID     uint32    `gorm:"column:last_uid"`
	SMTPHost    string    `gorm:"column:smtp_host"`
	SMTPPort    int       `gorm:"column:smtp_port"`
	SMTPTLS     string    `gorm:"column:smtp_tls"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (accountModel) TableName() string { return "email_accounts" }

type threadModel struct {
	ID                   string    `gorm:"primaryKey;column:id"`
	AccountID            string    `gorm:"column:account_id"`
	Subject              string    `gorm:"column:subject"`
	ParticipantAddresses string    `gorm:"column:participant_addresses"`
	CustomerID           string    `gorm:"column:customer_id"`
	MessageCount         int       `gorm:"column:message_count"`
	LastMessageAt        time.Time `gorm:"column:last_message_at"`
	CreatedAt            time.Time `gorm:"column:created_at"`
}

func (threadModel) TableName() string { return "email_threads" }

type messageModel struct {
	ID         string    `gorm:"primaryKey;column:id"`
	AccountID  string    `gorm:"column:account_id"`
	ThreadID   string    `gorm:"column:thread_id"`
	UID        uint32    `gorm:"column:uid"`
	Folder     string    `gorm:"column:folder"`
	Direction  string    `gorm:"column:direction"`
	MessageID  string    `gorm:"column:message_id"`
	References string    `gorm:"column:ref_ids"`
	InReplyTo  string    `gorm:"column:in_reply_to"`
	Subject    string    `gorm:"column:subject"`
	FromAddr   string    `gorm:"column:from_addr"`
	FromName   string    `gorm:"column:from_name"`
	ToAddrs    string    `gorm:"column:to_addrs"`
	TextBody   string    `gorm:"column:text_body"`
	HTMLBody   string    `gorm:"column:html_body"`
	ReceivedAt time.Time `gorm:"column:received_at"`
	FetchedAt  time.Time `gorm:"column:fetched_at"`
}

func (messageModel) TableName() string { return "email_messages" }

type attachmentModel struct {
	ID        string    `gorm:"primaryKey;column:id"`
	MessageID string    `gorm:"column:message_id"`
	Filename  string    `gorm:"column:filename"`
	MIMEType  string    `gorm:"column:mime_type"`
	Size      int64     `gorm:"column:size"`
	Data      []byte    `gorm:"column:data"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (attachmentModel) TableName() string { return "email_attachments" }

type tagModel struct {
	ID        string    `gorm:"primaryKey;column:id"`
	AccountID string    `gorm:"column:account_id"`
	Name      string    `gorm:"column:name"`
	Color     string    `gorm:"column:color"`
	System    int       `gorm:"column:system"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (tagModel) TableName() string { return "email_tags" }

type folderModel struct {
	ID        string    `gorm:"primaryKey;column:id"`
	AccountID string    `gorm:"column:account_id"`
	Name      string    `gorm:"column:name"`
	LastUID   uint32    `gorm:"column:last_uid"`
	Enabled   bool      `gorm:"column:enabled"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (folderModel) TableName() string { return "email_account_folders" }

type threadTagModel struct {
	ThreadID string `gorm:"column:thread_id"`
	TagID    string `gorm:"column:tag_id"`
}

func (threadTagModel) TableName() string { return "email_thread_tags" }
