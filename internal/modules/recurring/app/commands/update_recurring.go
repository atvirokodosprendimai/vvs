package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/recurring/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type UpdateRecurringCommand struct {
	ID           string
	CustomerID   string
	CustomerName string
	Frequency    string
	DayOfMonth   int
	Lines        []RecurringLineInput
}

type UpdateRecurringHandler struct {
	repo      domain.RecurringInvoiceRepository
	publisher events.EventPublisher
}

func NewUpdateRecurringHandler(repo domain.RecurringInvoiceRepository, pub events.EventPublisher) *UpdateRecurringHandler {
	return &UpdateRecurringHandler{repo: repo, publisher: pub}
}

func (h *UpdateRecurringHandler) Handle(ctx context.Context, cmd UpdateRecurringCommand) error {
	invoice, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}

	schedule, err := domain.NewSchedule(cmd.Frequency, cmd.DayOfMonth)
	if err != nil {
		return err
	}

	invoice.CustomerID = cmd.CustomerID
	invoice.CustomerName = cmd.CustomerName
	invoice.Schedule = schedule
	invoice.Lines = nil
	invoice.UpdatedAt = time.Now().UTC()

	for _, line := range cmd.Lines {
		currency := line.Currency
		if currency == "" {
			currency = "EUR"
		}
		if err := invoice.AddLine(
			line.ProductID,
			line.ProductName,
			line.Description,
			line.Quantity,
			shareddomain.NewMoney(line.UnitPrice, currency),
		); err != nil {
			return err
		}
	}

	if err := h.repo.Save(ctx, invoice); err != nil {
		return err
	}

	data, _ := json.Marshal(map[string]string{
		"id":          invoice.ID,
		"customer_id": invoice.CustomerID,
	})

	h.publisher.Publish(ctx, "isp.recurring.updated", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "recurring.updated",
		AggregateID: invoice.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return nil
}
