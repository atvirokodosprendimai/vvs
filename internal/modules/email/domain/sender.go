package domain

import "context"

// EmailSender is the port for sending outbound email.
type EmailSender interface {
	Send(ctx context.Context, account *EmailAccount, to, subject, body, inReplyTo, references string) error
}

// EmailFolderAppender is the port for appending raw RFC 2822 messages to an IMAP folder.
type EmailFolderAppender interface {
	AppendToFolder(ctx context.Context, account *EmailAccount, folder string, raw []byte) error
}
