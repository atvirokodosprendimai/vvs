package commands

import (
	"context"
	"errors"
	"fmt"

	invoicecommands "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/app/commands"
	invoicedomain "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/domain"
	"github.com/atvirokodosprendimai/vvs/internal/modules/payment/domain"
)

// InvoiceLookup is a port for finding invoices from the payment module.
type InvoiceLookup interface {
	FindByCode(ctx context.Context, code string) (*invoicedomain.Invoice, error)
}

// InvoiceMarker is a port for marking invoices as paid.
type InvoiceMarker interface {
	Handle(ctx context.Context, cmd invoicecommands.MarkPaidCommand) (*invoicedomain.Invoice, error)
}

// PreviewImportCommand parses CSV bytes and returns match results without applying anything.
type PreviewImportCommand struct {
	CSVData []byte
}

// PreviewImportHandler parses a CSV and returns matched MatchResults.
type PreviewImportHandler struct {
	lookup InvoiceLookup
}

func NewPreviewImportHandler(lookup InvoiceLookup) *PreviewImportHandler {
	return &PreviewImportHandler{lookup: lookup}
}

func (h *PreviewImportHandler) Handle(ctx context.Context, cmd PreviewImportCommand) ([]domain.MatchResult, error) {
	entries, err := domain.ParseCSV(cmd.CSVData)
	if err != nil {
		return nil, err
	}

	// Extract all invoice codes referenced in the CSV (deduplicated).
	codeSet := map[string]struct{}{}
	for _, e := range entries {
		if c := domain.ExtractInvoiceCode(e.Reference); c != "" {
			codeSet[c] = struct{}{}
		}
		if c := domain.ExtractInvoiceCode(e.Description); c != "" {
			codeSet[c] = struct{}{}
		}
	}

	// Batch-fetch the referenced invoices.
	var refs []domain.InvoiceRef
	for code := range codeSet {
		inv, err := h.lookup.FindByCode(ctx, code)
		if err != nil {
			if errors.Is(err, invoicedomain.ErrInvoiceNotFound) {
				continue // not found — matcher will mark as unmatched
			}
			return nil, fmt.Errorf("lookup invoice %s: %w", code, err)
		}
		refs = append(refs, domain.InvoiceRef{
			ID:           inv.ID,
			Code:         inv.Code,
			CustomerCode: inv.CustomerCode,
			Amount:       inv.TotalAmount,
			Status:       string(inv.Status),
		})
	}

	return domain.MatchPayments(entries, refs), nil
}

// ConfirmImportCommand marks the given invoice IDs as paid.
type ConfirmImportCommand struct {
	InvoiceIDs []string
}

// ConfirmImportResult reports which invoices were marked paid and which failed.
type ConfirmImportResult struct {
	MarkedPaid []string
	Errors     []string
}

// ConfirmImportHandler applies confirmed matches by calling MarkPaid on each invoice.
type ConfirmImportHandler struct {
	marker InvoiceMarker
}

func NewConfirmImportHandler(marker InvoiceMarker) *ConfirmImportHandler {
	return &ConfirmImportHandler{marker: marker}
}

func (h *ConfirmImportHandler) Handle(ctx context.Context, cmd ConfirmImportCommand) (ConfirmImportResult, error) {
	var result ConfirmImportResult
	for _, id := range cmd.InvoiceIDs {
		if _, err := h.marker.Handle(ctx, invoicecommands.MarkPaidCommand{InvoiceID: id}); err != nil {
			result.Errors = append(result.Errors, id+": "+err.Error())
			continue
		}
		result.MarkedPaid = append(result.MarkedPaid, id)
	}
	return result, nil
}
