package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/ticket/domain"
	"github.com/vvs/isp/internal/shared/events"
)

// ChangeTicketStatusCommand changes a ticket's status via a named action.
// Action must be one of: start, resolve, close, reopen.
type ChangeTicketStatusCommand struct {
	ID     string
	Action string // start | resolve | close | reopen
}

type ChangeTicketStatusHandler struct {
	repo      domain.TicketRepository
	publisher events.EventPublisher
}

func NewChangeTicketStatusHandler(repo domain.TicketRepository, pub events.EventPublisher) *ChangeTicketStatusHandler {
	return &ChangeTicketStatusHandler{repo: repo, publisher: pub}
}

func (h *ChangeTicketStatusHandler) Handle(ctx context.Context, cmd ChangeTicketStatusCommand) error {
	tk, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}

	switch cmd.Action {
	case "start":
		err = tk.StartWork()
	case "resolve":
		err = tk.Resolve()
	case "close":
		err = tk.Close()
	case "reopen":
		err = tk.Reopen()
	default:
		return fmt.Errorf("unknown action %q: must be one of start, resolve, close, reopen", cmd.Action)
	}
	if err != nil {
		return err
	}

	if err := h.repo.Save(ctx, tk); err != nil {
		return err
	}
	h.publisher.Publish(ctx, "isp.ticket.status_changed", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "ticket.status_changed",
		AggregateID: tk.ID,
		OccurredAt:  time.Now().UTC(),
	})
	return nil
}
