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

type RecurringLineInput struct {
	ProductID   string
	ProductName string
	Description string
	Quantity    int
	UnitPrice   int64
	Currency    string
}

type CreateRecurringCommand struct {
	CustomerID   string
	CustomerName string
	Frequency    string
	DayOfMonth   int
	Lines        []RecurringLineInput
}

type CreateRecurringHandler struct {
	repo      domain.RecurringInvoiceRepository
	publisher events.EventPublisher
}

func NewCreateRecurringHandler(repo domain.RecurringInvoiceRepository, pub events.EventPublisher) *CreateRecurringHandler {
	return &CreateRecurringHandler{repo: repo, publisher: pub}
}

func (h *CreateRecurringHandler) Handle(ctx context.Context, cmd CreateRecurringCommand) (*domain.RecurringInvoice, error) {
	invoice, err := domain.NewRecurringInvoice(cmd.CustomerID, cmd.CustomerName, cmd.Frequency, cmd.DayOfMonth)
	if err != nil {
		return nil, err
	}

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
			return nil, err
		}
	}

	if err := h.repo.Save(ctx, invoice); err != nil {
		return nil, err
	}

	data, _ := json.Marshal(domainToReadModel(invoice))

	h.publisher.Publish(ctx, "isp.recurring.created", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "recurring.created",
		AggregateID: invoice.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return invoice, nil
}
