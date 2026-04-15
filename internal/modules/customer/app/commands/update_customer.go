package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/customer/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type UpdateCustomerCommand struct {
	ID          string
	CompanyName string
	ContactName string
	Email       string
	Phone       string
	Street      string
	City        string
	PostalCode  string
	Country     string
	TaxID       string
	Notes       string
	RouterID    string // empty = clear provisioning
	IPAddress   string
	MACAddress  string
}

type UpdateCustomerHandler struct {
	repo      domain.CustomerRepository
	publisher events.EventPublisher
}

func NewUpdateCustomerHandler(repo domain.CustomerRepository, pub events.EventPublisher) *UpdateCustomerHandler {
	return &UpdateCustomerHandler{repo: repo, publisher: pub}
}

func (h *UpdateCustomerHandler) Handle(ctx context.Context, cmd UpdateCustomerCommand) error {
	customer, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}

	if err := customer.Update(
		cmd.CompanyName, cmd.ContactName, cmd.Email, cmd.Phone,
		cmd.Street, cmd.City, cmd.PostalCode, cmd.Country, cmd.TaxID, cmd.Notes,
	); err != nil {
		return err
	}

	customer.SetNetworkInfo(cmd.RouterID, cmd.IPAddress, cmd.MACAddress)

	if err := h.repo.Save(ctx, customer); err != nil {
		return err
	}

	data, _ := json.Marshal(domainToReadModel(customer))

	h.publisher.Publish(ctx, "isp.customer.updated", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "customer.updated",
		AggregateID: customer.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return nil
}
