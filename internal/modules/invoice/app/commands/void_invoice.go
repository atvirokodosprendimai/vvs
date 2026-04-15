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
	ID string
}

type VoidInvoiceHandler struct {
	repo      domain.InvoiceRepository
	publisher events.EventPublisher
}

func NewVoidInvoiceHandler(repo domain.InvoiceRepository, pub events.EventPublisher) *VoidInvoiceHandler {
	return &VoidInvoiceHandler{repo: repo, publisher: pub}
}

func (h *VoidInvoiceHandler) Handle(ctx context.Context, cmd VoidInvoiceCommand) error {
	invoice, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}

	if err := invoice.Void(); err != nil {
		return err
	}

	if err := h.repo.Save(ctx, invoice); err != nil {
		return err
	}

	data, _ := json.Marshal(domainToReadModel(invoice))

	h.publisher.Publish(ctx, "isp.invoice.voided", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "invoice.voided",
		AggregateID: invoice.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return nil
}
