package commands

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/ticket/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type UpdateTicketCommand struct {
	ID       string
	Subject  string
	Body     string
	Priority string
}

type UpdateTicketHandler struct {
	repo      domain.TicketRepository
	publisher events.EventPublisher
}

func NewUpdateTicketHandler(repo domain.TicketRepository, pub events.EventPublisher) *UpdateTicketHandler {
	return &UpdateTicketHandler{repo: repo, publisher: pub}
}

func (h *UpdateTicketHandler) Handle(ctx context.Context, cmd UpdateTicketCommand) error {
	tk, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if cmd.Subject == "" {
		return domain.ErrSubjectRequired
	}
	tk.Subject = cmd.Subject
	tk.Body = cmd.Body
	if cmd.Priority != "" {
		tk.Priority = cmd.Priority
	}
	tk.UpdatedAt = time.Now().UTC()
	if err := h.repo.Save(ctx, tk); err != nil {
		return err
	}
	h.publisher.Publish(ctx, events.TicketUpdated.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "ticket.updated",
		AggregateID: tk.ID,
		OccurredAt:  tk.UpdatedAt,
	})
	return nil
}
