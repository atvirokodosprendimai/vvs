package commands

import (
	"context"
	"fmt"
)

// InvoiceMarker is a port for marking invoices as paid.
type InvoiceMarker interface {
	MarkPaid(ctx context.Context, invoiceID string) error
}

// ConfirmImportCommand carries the invoice IDs the user selected to mark as paid.
type ConfirmImportCommand struct {
	InvoiceIDs []string
}

// ConfirmImportResult reports how many invoices were successfully marked paid.
type ConfirmImportResult struct {
	MarkedCount int
	Errors      []string
}

// ConfirmImportHandler marks the selected invoices as paid.
type ConfirmImportHandler struct {
	invoices InvoiceMarker
}

func NewConfirmImportHandler(invoices InvoiceMarker) *ConfirmImportHandler {
	return &ConfirmImportHandler{invoices: invoices}
}

// Handle marks each invoice in the command as paid and returns a summary.
func (h *ConfirmImportHandler) Handle(ctx context.Context, cmd ConfirmImportCommand) (ConfirmImportResult, error) {
	if len(cmd.InvoiceIDs) == 0 {
		return ConfirmImportResult{}, fmt.Errorf("no invoices selected")
	}

	var result ConfirmImportResult
	for _, id := range cmd.InvoiceIDs {
		if id == "" {
			continue
		}
		if err := h.invoices.MarkPaid(ctx, id); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", id, err))
		} else {
			result.MarkedCount++
		}
	}
	return result, nil
}
