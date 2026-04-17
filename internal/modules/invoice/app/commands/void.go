package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/invoice/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type VoidInvoiceCommand struct {
	InvoiceID string
}

type VoidInvoiceHandler struct {
	repo      domain.InvoiceRepository
	publisher events.EventPublisher
}

func NewVoidInvoiceHandler(repo domain.InvoiceRepository, pub events.EventPublisher) *VoidInvoiceHandler {
	return &VoidInvoiceHandler{repo: repo, publisher: pub}
}

func (h *VoidInvoiceHandler) Handle(ctx context.Context, cmd VoidInvoiceCommand) (*domain.Invoice, error) {
	inv, err := h.repo.FindByID(ctx, cmd.InvoiceID)
	if err != nil {
		return nil, err
	}

	if err := inv.Void(); err != nil {
		return nil, err
	}

	if err := h.repo.Save(ctx, inv); err != nil {
		return nil, err
	}

	data, _ := json.Marshal(inv)
	h.publisher.Publish(ctx, events.InvoiceVoided.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "invoice.voided",
		AggregateID: inv.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return inv, nil
}
