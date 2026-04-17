package commands

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/ticket/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type DeleteTicketCommand struct {
	ID string
}

type DeleteTicketHandler struct {
	repo      domain.TicketRepository
	publisher events.EventPublisher
}

func NewDeleteTicketHandler(repo domain.TicketRepository, pub events.EventPublisher) *DeleteTicketHandler {
	return &DeleteTicketHandler{repo: repo, publisher: pub}
}

func (h *DeleteTicketHandler) Handle(ctx context.Context, cmd DeleteTicketCommand) error {
	_, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if err := h.repo.Delete(ctx, cmd.ID); err != nil {
		return err
	}
	h.publisher.Publish(ctx, events.TicketDeleted.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "ticket.deleted",
		AggregateID: cmd.ID,
		OccurredAt:  time.Now().UTC(),
	})
	return nil
}
