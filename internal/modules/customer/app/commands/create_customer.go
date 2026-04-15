package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/customer/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type CreateCustomerCommand struct {
	CompanyName string
	ContactName string
	Email       string
	Phone       string
}

type CreateCustomerHandler struct {
	repo      domain.CustomerRepository
	publisher events.EventPublisher
}

func NewCreateCustomerHandler(repo domain.CustomerRepository, pub events.EventPublisher) *CreateCustomerHandler {
	return &CreateCustomerHandler{repo: repo, publisher: pub}
}

func (h *CreateCustomerHandler) Handle(ctx context.Context, cmd CreateCustomerCommand) (*domain.Customer, error) {
	code, err := h.repo.NextCode(ctx)
	if err != nil {
		return nil, err
	}

	customer, err := domain.NewCustomer(code, cmd.CompanyName, cmd.ContactName, cmd.Email, cmd.Phone)
	if err != nil {
		return nil, err
	}

	if err := h.repo.Save(ctx, customer); err != nil {
		return nil, err
	}

	data, _ := json.Marshal(domainToReadModel(customer))

	h.publisher.Publish(ctx, "isp.customer.created", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "customer.created",
		AggregateID: customer.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return customer, nil
}
