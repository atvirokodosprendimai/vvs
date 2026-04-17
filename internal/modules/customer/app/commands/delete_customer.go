package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/customer/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type DeleteCustomerCommand struct {
	ID string
}

type DeleteCustomerHandler struct {
	repo      domain.CustomerRepository
	publisher events.EventPublisher
}

func NewDeleteCustomerHandler(repo domain.CustomerRepository, pub events.EventPublisher) *DeleteCustomerHandler {
	return &DeleteCustomerHandler{repo: repo, publisher: pub}
}

func (h *DeleteCustomerHandler) Handle(ctx context.Context, cmd DeleteCustomerCommand) error {
	customer, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}

	if err := h.repo.Delete(ctx, cmd.ID); err != nil {
		return err
	}

	data, _ := json.Marshal(map[string]string{
		"id":   customer.ID,
		"code": customer.Code.String(),
	})

	h.publisher.Publish(ctx, events.CustomerDeleted.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "customer.deleted",
		AggregateID: customer.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return nil
}
