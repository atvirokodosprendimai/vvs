package domain

import "context"

// EmailSender is the port for sending outbound email.
type EmailSender interface {
	Send(ctx context.Context, account *EmailAccount, to, subject, body, inReplyTo, references string) error
}
