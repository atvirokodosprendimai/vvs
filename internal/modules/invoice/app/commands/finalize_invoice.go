package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/invoice/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type FinalizeInvoiceCommand struct {
	ID string
}

type FinalizeInvoiceHandler struct {
	repo      domain.InvoiceRepository
	publisher events.EventPublisher
}

func NewFinalizeInvoiceHandler(repo domain.InvoiceRepository, pub events.EventPublisher) *FinalizeInvoiceHandler {
	return &FinalizeInvoiceHandler{repo: repo, publisher: pub}
}

func (h *FinalizeInvoiceHandler) Handle(ctx context.Context, cmd FinalizeInvoiceCommand) error {
	invoice, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}

	if err := invoice.Finalize(); err != nil {
		return err
	}

	if err := h.repo.Save(ctx, invoice); err != nil {
		return err
	}

	data, _ := json.Marshal(map[string]string{
		"id":     invoice.ID,
		"number": invoice.InvoiceNumber,
	})

	h.publisher.Publish(ctx, "isp.invoice.finalized", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "invoice.finalized",
		AggregateID: invoice.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return nil
}
