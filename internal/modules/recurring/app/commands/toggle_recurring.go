package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/recurring/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type ToggleRecurringCommand struct {
	ID string
}

type ToggleRecurringHandler struct {
	repo      domain.RecurringInvoiceRepository
	publisher events.EventPublisher
}

func NewToggleRecurringHandler(repo domain.RecurringInvoiceRepository, pub events.EventPublisher) *ToggleRecurringHandler {
	return &ToggleRecurringHandler{repo: repo, publisher: pub}
}

func (h *ToggleRecurringHandler) Handle(ctx context.Context, cmd ToggleRecurringCommand) error {
	invoice, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}

	var eventType string
	var subject string

	if invoice.Status == domain.StatusActive {
		if err := invoice.Pause(); err != nil {
			return err
		}
		eventType = "recurring.paused"
		subject = "isp.recurring.paused"
	} else {
		if err := invoice.Resume(); err != nil {
			return err
		}
		eventType = "recurring.resumed"
		subject = "isp.recurring.resumed"
	}

	if err := h.repo.Save(ctx, invoice); err != nil {
		return err
	}

	data, _ := json.Marshal(map[string]string{
		"id":     invoice.ID,
		"status": string(invoice.Status),
	})

	h.publisher.Publish(ctx, subject, events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        eventType,
		AggregateID: invoice.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return nil
}
