package commands

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/email/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

type LinkCustomerCommand struct {
	ThreadID   string
	CustomerID string // empty = unlink
}

type LinkCustomerHandler struct {
	threads   domain.EmailThreadRepository
	publisher events.EventPublisher
}

func NewLinkCustomerHandler(threads domain.EmailThreadRepository, pub events.EventPublisher) *LinkCustomerHandler {
	return &LinkCustomerHandler{threads: threads, publisher: pub}
}

func (h *LinkCustomerHandler) Handle(ctx context.Context, cmd LinkCustomerCommand) error {
	t, err := h.threads.FindByID(ctx, cmd.ThreadID)
	if err != nil {
		return err
	}
	t.CustomerID = cmd.CustomerID
	if err := h.threads.Save(ctx, t); err != nil {
		return err
	}
	h.publisher.Publish(ctx, events.EmailCustomerLinked.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "email.customer_linked",
		AggregateID: cmd.ThreadID, OccurredAt: time.Now().UTC(),
	})
	return nil
}
