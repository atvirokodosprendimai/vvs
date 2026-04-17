package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/invoice/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type CreateInvoiceCommand struct {
	CustomerID   string
	CustomerName string
	CustomerCode string
	IssueDate    time.Time
	DueDate      time.Time
	Notes        string
	LineItems    []LineItemInput
}

type LineItemInput struct {
	ProductID      string
	ProductName    string
	Description    string
	Quantity       int
	UnitPriceGross int64 // gross price per unit (includes VAT)
	VATRate        int   // percentage: 0, 5, 9, 21
}

type CreateInvoiceHandler struct {
	repo      domain.InvoiceRepository
	publisher events.EventPublisher
}

func NewCreateInvoiceHandler(repo domain.InvoiceRepository, pub events.EventPublisher) *CreateInvoiceHandler {
	return &CreateInvoiceHandler{repo: repo, publisher: pub}
}

func (h *CreateInvoiceHandler) Handle(ctx context.Context, cmd CreateInvoiceCommand) (*domain.Invoice, error) {
	code, err := h.repo.NextCode(ctx)
	if err != nil {
		return nil, err
	}

	id := uuid.Must(uuid.NewV7()).String()
	inv := domain.NewInvoice(id, cmd.CustomerID, cmd.CustomerName, cmd.CustomerCode, code)
	inv.IssueDate = cmd.IssueDate
	inv.DueDate = cmd.DueDate
	inv.Notes = cmd.Notes

	for _, li := range cmd.LineItems {
		item := domain.LineItem{
			ID:             uuid.Must(uuid.NewV7()).String(),
			ProductID:      li.ProductID,
			ProductName:    li.ProductName,
			Description:    li.Description,
			Quantity:       li.Quantity,
			UnitPriceGross: li.UnitPriceGross,
			VATRate:        li.VATRate,
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
