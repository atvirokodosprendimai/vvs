package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/invoice/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type CreateInvoiceLineInput struct {
	ProductID   string
	ProductName string
	Description string
	Quantity    int
	UnitPrice   int64 // cents
}

type CreateInvoiceCommand struct {
	CustomerID   string
	CustomerName string
	IssueDate    time.Time
	DueDate      time.Time
	TaxRate      int
	Lines        []CreateInvoiceLineInput
}

type CreateInvoiceHandler struct {
	repo      domain.InvoiceRepository
	publisher events.EventPublisher
}

func NewCreateInvoiceHandler(repo domain.InvoiceRepository, pub events.EventPublisher) *CreateInvoiceHandler {
	return &CreateInvoiceHandler{repo: repo, publisher: pub}
}

func (h *CreateInvoiceHandler) Handle(ctx context.Context, cmd CreateInvoiceCommand) (*domain.Invoice, error) {
	number, err := h.repo.NextInvoiceNumber(ctx, cmd.IssueDate.Year())
	if err != nil {
		return nil, err
	}

	invoice, err := domain.NewInvoice(number, cmd.CustomerID, cmd.CustomerName, cmd.IssueDate, cmd.DueDate)
	if err != nil {
		return nil, err
	}

	if cmd.TaxRate > 0 {
		invoice.TaxRate = cmd.TaxRate
	}

	for _, line := range cmd.Lines {
		if err := invoice.AddLine(line.ProductID, line.ProductName, line.Description, line.Quantity, shareddomain.EUR(line.UnitPrice)); err != nil {
			return nil, err
		}
	}

	if err := h.repo.Save(ctx, invoice); err != nil {
		return nil, err
	}

	data, _ := json.Marshal(domainToReadModel(invoice))

	h.publisher.Publish(ctx, "isp.invoice.created", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "invoice.created",
		AggregateID: invoice.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return invoice, nil
}
