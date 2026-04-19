package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/invoice/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

type FinalizeInvoiceCommand struct {
	InvoiceID string
}

type FinalizeInvoiceHandler struct {
	repo      domain.InvoiceRepository
	publisher events.EventPublisher
}

func NewFinalizeInvoiceHandler(repo domain.InvoiceRepository, pub events.EventPublisher) *FinalizeInvoiceHandler {
	return &FinalizeInvoiceHandler{repo: repo, publisher: pub}
}

func (h *FinalizeInvoiceHandler) Handle(ctx context.Context, cmd FinalizeInvoiceCommand) (*domain.Invoice, error) {
	inv, err := h.repo.FindByID(ctx, cmd.InvoiceID)
	if err != nil {
		return nil, err
	}

	if err := inv.Finalize(); err != nil {
		return nil, err
	}

	if err := h.repo.Save(ctx, inv); err != nil {
		return nil, err
	}

	data, _ := json.Marshal(inv)
	h.publisher.Publish(ctx, events.InvoiceFinalized.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "invoice.finalized",
		AggregateID: inv.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return inv, nil
}
