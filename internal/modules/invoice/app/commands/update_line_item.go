package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/invoice/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type UpdateLineItemCommand struct {
	InvoiceID      string
	LineItemID     string
	ProductName    string
	Description    string
	Quantity       int
	UnitPriceGross int64
	VATRate        int
}

type UpdateLineItemHandler struct {
	repo      domain.InvoiceRepository
	publisher events.EventPublisher
}

func NewUpdateLineItemHandler(repo domain.InvoiceRepository, pub events.EventPublisher) *UpdateLineItemHandler {
	return &UpdateLineItemHandler{repo: repo, publisher: pub}
}

func (h *UpdateLineItemHandler) Handle(ctx context.Context, cmd UpdateLineItemCommand) (*domain.Invoice, error) {
	inv, err := h.repo.FindByID(ctx, cmd.InvoiceID)
	if err != nil {
		return nil, err
	}

	if err := inv.UpdateLineItem(cmd.LineItemID, cmd.ProductName, cmd.Description, cmd.Quantity, cmd.UnitPriceGross, cmd.VATRate); err != nil {
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
