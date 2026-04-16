package commands

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/ticket/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type AddCommentCommand struct {
	TicketID string
	Body     string
	AuthorID string
}

type AddCommentHandler struct {
	repo      domain.TicketRepository
	publisher events.EventPublisher
}

func NewAddCommentHandler(repo domain.TicketRepository, pub events.EventPublisher) *AddCommentHandler {
	return &AddCommentHandler{repo: repo, publisher: pub}
}

func (h *AddCommentHandler) Handle(ctx context.Context, cmd AddCommentCommand) (*domain.TicketComment, error) {
	if cmd.Body == "" {
		return nil, errors.New("comment body is required")
	}
	// Ensure the ticket exists.
	_, err := h.repo.FindByID(ctx, cmd.TicketID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	comment := &domain.TicketComment{
		ID:        uuid.Must(uuid.NewV7()).String(),
		TicketID:  cmd.TicketID,
		Body:      cmd.Body,
		AuthorID:  cmd.AuthorID,
		CreatedAt: now,
	}
	if err := h.repo.SaveComment(ctx, comment); err != nil {
		return nil, err
	}
	h.publisher.Publish(ctx, "isp.ticket.comment_added", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "ticket.comment_added",
		AggregateID: cmd.TicketID,
		OccurredAt:  now,
	})
	return comment, nil
}
