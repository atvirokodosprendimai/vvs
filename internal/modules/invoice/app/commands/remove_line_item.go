package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/invoice/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type RemoveLineItemCommand struct {
	InvoiceID  string
	LineItemID string
}

type RemoveLineItemHandler struct {
	repo      domain.InvoiceRepository
	publisher events.EventPublisher
}

func NewRemoveLineItemHandler(repo domain.InvoiceRepository, pub events.EventPublisher) *RemoveLineItemHandler {
	return &RemoveLineItemHandler{repo: repo, publisher: pub}
}

func (h *RemoveLineItemHandler) Handle(ctx context.Context, cmd RemoveLineItemCommand) (*domain.Invoice, error) {
	inv, err := h.repo.FindByID(ctx, cmd.InvoiceID)
	if err != nil {
		return nil, err
	}

	if err := inv.RemoveLineItem(cmd.LineItemID); err != nil {
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
