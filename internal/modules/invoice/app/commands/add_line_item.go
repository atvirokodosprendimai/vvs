package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/invoice/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type AddLineItemCommand struct {
	InvoiceID      string
	ProductID      string
	ProductName    string
	Description    string
	Quantity       int
	UnitPriceGross int64
	VATRate        int
}

type AddLineItemHandler struct {
	repo      domain.InvoiceRepository
	publisher events.EventPublisher
}

func NewAddLineItemHandler(repo domain.InvoiceRepository, pub events.EventPublisher) *AddLineItemHandler {
	return &AddLineItemHandler{repo: repo, publisher: pub}
}

func (h *AddLineItemHandler) Handle(ctx context.Context, cmd AddLineItemCommand) (*domain.Invoice, error) {
	inv, err := h.repo.FindByID(ctx, cmd.InvoiceID)
	if err != nil {
		return nil, err
	}

	item := domain.LineItem{
		ID:             uuid.Must(uuid.NewV7()).String(),
		ProductID:      cmd.ProductID,
		ProductName:    cmd.ProductName,
		Description:    cmd.Description,
		Quantity:       cmd.Quantity,
		UnitPriceGross: cmd.UnitPriceGross,
		VATRate:        cmd.VATRate,
	}

	if err := inv.AddLineItem(item); err != nil {
		return nil, err
	}
	inv.Recalculate()

	if err := h.repo.Save(ctx, inv); err != nil {
		return nil, err
	}

	data, _ := json.Marshal(inv)
	h.publisher.Publish(ctx, events.InvoiceUpdated.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "invoice.updated",
		AggregateID: inv.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return inv, nil
}
