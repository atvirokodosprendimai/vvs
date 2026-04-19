package commands

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	smtpadapter "github.com/atvirokodosprendimai/vvs/internal/modules/email/adapters/smtp"
	"github.com/atvirokodosprendimai/vvs/internal/modules/email/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

type ComposeEmailCommand struct {
	AccountID string
	To        string
	Subject   string
	Body      string
}

type ComposeEmailHandler struct {
	accounts domain.EmailAccountRepository
	threads  domain.EmailThreadRepository
	messages domain.EmailMessageRepository
	sender   domain.EmailSender
	appender domain.EmailFolderAppender
	pub      events.EventPublisher
}

func NewComposeEmailHandler(
	accounts domain.EmailAccountRepository,
	threads domain.EmailThreadRepository,
	messages domain.EmailMessageRepository,
	sender domain.EmailSender,
	appender domain.EmailFolderAppender,
	pub events.EventPublisher,
) *ComposeEmailHandler {
	return &ComposeEmailHandler{
		accounts: accounts,
		threads:  threads,
		messages: messages,
		sender:   sender,
		appender: appender,
		pub:      pub,
	}
}

func (h *ComposeEmailHandler) Handle(ctx context.Context, cmd ComposeEmailCommand) error {
	if cmd.To == "" {
		return fmt.Errorf("compose: recipient (To) is required")
	}
	if cmd.Body == "" {
		return fmt.Errorf("compose: body is required")
	}

	account, err := h.accounts.FindByID(ctx, cmd.AccountID)
	if err != nil {
		return fmt.Errorf("compose: load account: %w", err)
	}

	msgID, raw := smtpadapter.BuildMessage(account, cmd.To, cmd.Subject, cmd.Body, "", "")

	if err := h.sender.Send(ctx, account, cmd.To, cmd.Subject, cmd.Body, "", ""); err != nil {
		return fmt.Errorf("compose: send: %w", err)
	}

	now := time.Now().UTC()

	thread := &domain.EmailThread{
		ID:            uuid.Must(uuid.NewV7()).String(),
		AccountID:     cmd.AccountID,
		Subject:       cmd.Subject,
		MessageCount:  1,
		LastMessageAt: now,
		CreatedAt:     now,
	}
	if err := h.threads.Save(ctx, thread); err != nil {
		return fmt.Errorf("compose: save thread: %w", err)
	}

	msg := &domain.EmailMessage{
		ID:         uuid.Must(uuid.NewV7()).String(),
		AccountID:  cmd.AccountID,
		ThreadID:   thread.ID,
		MessageID:  msgID,
		UID:        0,
		Folder:     "",
		Direction:  "out",
		Subject:    cmd.Subject,
		FromAddr:   account.Username,
		ToAddrs:    cmd.To,
		TextBody:   cmd.Body,
		ReceivedAt: now,
		FetchedAt:  now,
	}
	if err := h.messages.Save(ctx, msg); err != nil {
		slog.Error("compose: store sent message", "err", err)
	}

	if err := h.appender.AppendToFolder(ctx, account, account.EffectiveSentFolder(), raw); err != nil {
		slog.Error("compose: imap append", "err", err)
	}

	h.pub.Publish(ctx, events.EmailThreadUpdated.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "email.composed", AggregateID: thread.ID,
	})
	return nil
}
