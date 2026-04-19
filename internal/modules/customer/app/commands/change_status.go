package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/customer/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

const (
	ActionQualify  = "qualify"
	ActionConvert  = "convert"
	ActionSuspend  = "suspend"
	ActionActivate = "activate"
	ActionChurn    = "churn"
)

type ChangeCustomerStatusCommand struct {
	ID     string
	Action string
}

type ChangeCustomerStatusHandler struct {
	repo      domain.CustomerRepository
	publisher events.EventPublisher
}

func NewChangeCustomerStatusHandler(repo domain.CustomerRepository, pub events.EventPublisher) *ChangeCustomerStatusHandler {
	return &ChangeCustomerStatusHandler{repo: repo, publisher: pub}
}

func (h *ChangeCustomerStatusHandler) Handle(ctx context.Context, cmd ChangeCustomerStatusCommand) error {
	customer, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}

	var action string
	switch cmd.Action {
	case ActionQualify:
		err = customer.Qualify()
		action = "qualified"
	case ActionConvert:
		err = customer.Convert()
		action = "converted"
	case ActionSuspend:
		err = customer.Suspend()
		action = "suspended"
	case ActionActivate:
		err = customer.Activate()
		action = "activated"
	case ActionChurn:
		err = customer.Churn()
		action = "churned"
	default:
		return domain.ErrInvalidTransition
	}
	if err != nil {
		return err
	}

	if err := h.repo.Save(ctx, customer); err != nil {
		return err
	}

	data, _ := json.Marshal(domainToReadModel(customer))
	h.publisher.Publish(ctx, events.CustomerStatusChanged.Format(action), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "customer." + action,
		AggregateID: customer.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})
	return nil
}
