package commands

import (
	"context"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/ticket/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type OpenTicketCommand struct {
	CustomerID string
	Subject    string
	Body       string
	Priority   string
}

type OpenTicketHandler struct {
	repo      domain.TicketRepository
	publisher events.EventPublisher
}

func NewOpenTicketHandler(repo domain.TicketRepository, pub events.EventPublisher) *OpenTicketHandler {
	return &OpenTicketHandler{repo: repo, publisher: pub}
}

func (h *OpenTicketHandler) Handle(ctx context.Context, cmd OpenTicketCommand) (*domain.Ticket, error) {
	tk, err := domain.NewTicket(
		uuid.Must(uuid.NewV7()).String(),
		cmd.CustomerID,
		cmd.Subject,
		cmd.Body,
		cmd.Priority,
	)
	if err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, tk); err != nil {
		return nil, err
	}
	h.publisher.Publish(ctx, events.TicketOpened.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "ticket.opened",
		AggregateID: tk.ID,
		OccurredAt:  tk.CreatedAt,
	})
	return tk, nil
}
