package commands

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/email/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type SendReplyCommand struct {
	ThreadID string
	Body     string
}

type SendReplyHandler struct {
	threads  domain.EmailThreadRepository
	messages domain.EmailMessageRepository
	accounts domain.EmailAccountRepository
	sender   domain.EmailSender
	pub      events.EventPublisher
}

func NewSendReplyHandler(
	threads domain.EmailThreadRepository,
	messages domain.EmailMessageRepository,
	accounts domain.EmailAccountRepository,
	sender domain.EmailSender,
	pub events.EventPublisher,
) *SendReplyHandler {
	return &SendReplyHandler{threads: threads, messages: messages, accounts: accounts, sender: sender, pub: pub}
}

func (h *SendReplyHandler) Handle(ctx context.Context, cmd SendReplyCommand) error {
	if cmd.Body == "" {
		return fmt.Errorf("reply body is empty")
	}

	thread, err := h.threads.FindByID(ctx, cmd.ThreadID)
	if err != nil {
		return err
	}

	msgs, err := h.messages.ListForThread(ctx, cmd.ThreadID)
	if err != nil {
		return err
	}

	// Find the last inbound message for reply headers.
	var lastIn *domain.EmailMessage
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Direction != "out" {
			lastIn = msgs[i]
			break
		}
	}

	var to, inReplyTo, references string
	if lastIn != nil {
		to = lastIn.FromAddr
		inReplyTo = lastIn.MessageID
		refs := lastIn.References
		if lastIn.MessageID != "" {
			if refs != "" {
				refs += " "
			}
			refs += lastIn.MessageID
		}
		references = refs
	}

	account, err := h.accounts.FindByID(ctx, thread.AccountID)
	if err != nil {
		return err
	}

	subject := "Re: " + domain.NormalizeSubject(thread.Subject)

	if err := h.sender.Send(ctx, account, to, subject, cmd.Body, inReplyTo, references); err != nil {
		return fmt.Errorf("send reply: %w", err)
	}

	// Store sent message in thread.
	now := time.Now().UTC()
	sentMsg := &domain.EmailMessage{
		ID:         uuid.Must(uuid.NewV7()).String(),
		AccountID:  thread.AccountID,
		ThreadID:   thread.ID,
		UID:        0,
		Direction:  "out",
		Subject:    subject,
		FromAddr:   account.Username,
		ToAddrs:    to,
		TextBody:   cmd.Body,
		ReceivedAt: now,
		FetchedAt:  now,
	}
	if err := h.messages.Save(ctx, sentMsg); err != nil {
		slog.Error("email: store sent message", "err", err)
	} else {
		thread.MessageCount++
		thread.LastMessageAt = now
		if err := h.threads.Save(ctx, thread); err != nil {
			slog.Error("email: update thread after reply", "err", err)
		}
	}

	h.pub.Publish(ctx, "isp.email.thread_updated", events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "email.reply_sent", AggregateID: thread.ID,
	})
	return nil
}
