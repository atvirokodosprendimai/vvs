package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/invoice/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type MarkPaidCommand struct {
	InvoiceID string
}

type MarkPaidHandler struct {
	repo      domain.InvoiceRepository
	publisher events.EventPublisher
}

func NewMarkPaidHandler(repo domain.InvoiceRepository, pub events.EventPublisher) *MarkPaidHandler {
	return &MarkPaidHandler{repo: repo, publisher: pub}
}

func (h *MarkPaidHandler) Handle(ctx context.Context, cmd MarkPaidCommand) (*domain.Invoice, error) {
	inv, err := h.repo.FindByID(ctx, cmd.InvoiceID)
	if err != nil {
		return nil, err
	}

	if err := inv.MarkPaid(); err != nil {
		return nil, err
	}

	if err := h.repo.Save(ctx, inv); err != nil {
		return nil, err
	}

	data, _ := json.Marshal(inv)
	h.publisher.Publish(ctx, events.InvoicePaid.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "invoice.paid",
		AggregateID: inv.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return inv, nil
}
