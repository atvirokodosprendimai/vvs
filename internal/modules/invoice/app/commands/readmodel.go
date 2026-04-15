package commands

import (
	"github.com/vvs/isp/internal/modules/invoice/app/queries"
	"github.com/vvs/isp/internal/modules/invoice/domain"
)

// domainToReadModel maps a domain Invoice to InvoiceReadModel for use as NATS event payload.
func domainToReadModel(inv *domain.Invoice) queries.InvoiceReadModel {
	return queries.InvoiceReadModel{
		ID:               inv.ID,
		InvoiceNumber:    inv.InvoiceNumber,
		CustomerID:       inv.CustomerID,
		CustomerName:     inv.CustomerName,
		SubtotalAmount:   inv.Subtotal.Amount,
		SubtotalCurrency: string(inv.Subtotal.Currency),
		TaxRate:          inv.TaxRate,
		TaxAmount:        inv.TaxAmount.Amount,
		TaxCurrency:      string(inv.TaxAmount.Currency),
		TotalAmount:      inv.Total.Amount,
		TotalCurrency:    string(inv.Total.Currency),
		Status:           string(inv.Status),
		IssueDate:        inv.IssueDate,
		DueDate:          inv.DueDate,
		PaidDate:         inv.PaidDate,
		RecurringID:      inv.RecurringID,
		CreatedAt:        inv.CreatedAt,
		UpdatedAt:        inv.UpdatedAt,
	}
}
