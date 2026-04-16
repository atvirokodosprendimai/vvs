package commands

import (
	"context"

	"github.com/vvs/isp/internal/modules/email/domain"
	"github.com/vvs/isp/internal/shared/events"

	"github.com/google/uuid"
	"time"
)

type MarkReadCommand struct {
	ThreadID string
}

type MarkReadHandler struct {
	tags      domain.EmailTagRepository
	publisher events.EventPublisher
}

func NewMarkReadHandler(tags domain.EmailTagRepository, pub events.EventPublisher) *MarkReadHandler {
	return &MarkReadHandler{tags: tags, publisher: pub}
}

func (h *MarkReadHandler) Handle(ctx context.Context, cmd MarkReadCommand) error {
	tag, err := h.tags.FindSystemTag(ctx, domain.TagUnread)
	if err != nil {
		return nil // unread tag missing — not an error
	}
	if err := h.tags.RemoveFromThread(ctx, cmd.ThreadID, tag.ID); err != nil {
		return err
	}
	h.publisher.Publish(ctx, "isp.email.read", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "email.read",
		AggregateID: cmd.ThreadID,
		OccurredAt:  time.Now().UTC(),
	})
	return nil
}
