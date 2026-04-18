package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/invoice/domain"
	"github.com/vvs/isp/internal/shared/events"
)

// ServiceInfo holds the minimal data needed from an active service to create a line item.
type ServiceInfo struct {
	ID          string
	ProductID   string
	ProductName string
	PriceAmount int64 // cents
}

// ActiveServiceLister is a port for querying active services for a customer.
type ActiveServiceLister interface {
	ListActiveForCustomer(ctx context.Context, customerID string) ([]ServiceInfo, error)
}

type GenerateFromSubscriptionsCommand struct {
	CustomerID   string
	CustomerName string
	CustomerCode string
	DefaultVATRate int
}

type GenerateFromSubscriptionsHandler struct {
	repo      domain.InvoiceRepository
	publisher events.EventPublisher
	services  ActiveServiceLister
}

func NewGenerateFromSubscriptionsHandler(
	repo domain.InvoiceRepository,
	pub events.EventPublisher,
	services ActiveServiceLister,
) *GenerateFromSubscriptionsHandler {
	return &GenerateFromSubscriptionsHandler{repo: repo, publisher: pub, services: services}
}

// GenerateFromServicesCommand creates an invoice from a pre-provided list of services.
// Used by the billing cron action to bill only services due for renewal.
type GenerateFromServicesCommand struct {
	CustomerID     string
	CustomerName   string
	CustomerCode   string
	DefaultVATRate int
	Services       []ServiceInfo
}

type GenerateFromServicesHandler struct {
	repo      domain.InvoiceRepository
	publisher events.EventPublisher
}

func NewGenerateFromServicesHandler(repo domain.InvoiceRepository, pub events.EventPublisher) *GenerateFromServicesHandler {
	return &GenerateFromServicesHandler{repo: repo, publisher: pub}
}

func (h *GenerateFromServicesHandler) Handle(ctx context.Context, cmd GenerateFromServicesCommand) (*domain.Invoice, error) {
	if len(cmd.Services) == 0 {
		return nil, nil
	}

	code, err := h.repo.NextCode(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	id := uuid.Must(uuid.NewV7()).String()
	inv := domain.NewInvoice(id, cmd.CustomerID, cmd.CustomerName, cmd.CustomerCode, code)
	inv.IssueDate = now
	inv.DueDate = now.AddDate(0, 0, 30)

	vatRate := cmd.DefaultVATRate
	if vatRate <= 0 {
		vatRate = 21
	}
	for _, svc := range cmd.Services {
		item := domain.LineItem{
			ID:             uuid.Must(uuid.NewV7()).String(),
			ProductID:      svc.ProductID,
			ProductName:    svc.ProductName,
			Description:    svc.ProductName,
			Quantity:       1,
			UnitPriceGross: svc.PriceAmount,
			VATRate:        vatRate,
		}
		if err := inv.AddLineItem(item); err != nil {
			return nil, err
		}
	}
	inv.Recalculate()

	if err := h.repo.Save(ctx, inv); err != nil {
		return nil, err
	}

	data, _ := json.Marshal(inv)
	h.publisher.Publish(ctx, events.InvoiceCreated.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "invoice.created",
		AggregateID: inv.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return inv, nil
}

func (h *GenerateFromSubscriptionsHandler) Handle(ctx context.Context, cmd GenerateFromSubscriptionsCommand) (*domain.Invoice, error) {
	active, err := h.services.ListActiveForCustomer(ctx, cmd.CustomerID)
	if err != nil {
		return nil, err
	}

	code, err := h.repo.NextCode(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	id := uuid.Must(uuid.NewV7()).String()
	inv := domain.NewInvoice(id, cmd.CustomerID, cmd.CustomerName, cmd.CustomerCode, code)
	inv.IssueDate = now
	inv.DueDate = now.AddDate(0, 0, 30)

	vatRate := cmd.DefaultVATRate
	if vatRate <= 0 {
		vatRate = 21
	}
	for _, svc := range active {
		item := domain.LineItem{
			ID:             uuid.Must(uuid.NewV7()).String(),
			ProductID:      svc.ProductID,
			ProductName:    svc.ProductName,
			Description:    svc.ProductName,
			Quantity:       1,
			UnitPriceGross: svc.PriceAmount,
			VATRate:        vatRate,
		}
		if err := inv.AddLineItem(item); err != nil {
			return nil, err
		}
	}
	inv.Recalculate()

	if err := h.repo.Save(ctx, inv); err != nil {
		return nil, err
	}

	data, _ := json.Marshal(inv)
	h.publisher.Publish(ctx, events.InvoiceCreated.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "invoice.created",
		AggregateID: inv.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return inv, nil
}
