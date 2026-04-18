package commands

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/vvs/isp/internal/modules/payment/domain"
)

// InvoiceLookup is a port for finding invoices by reference/number.
type InvoiceLookup interface {
	// FindByNumber returns invoice ID, number, and status for a given invoice number.
	// Returns ("", "", "", nil) when not found.
	FindByNumber(ctx context.Context, number string) (id, invoiceNumber, status string, err error)
}

// PreviewImportCommand carries the raw CSV bytes to preview.
type PreviewImportCommand struct {
	CSVData []byte
}

// PreviewImportHandler parses a SEPA CSV and matches rows against invoices.
type PreviewImportHandler struct {
	invoices InvoiceLookup
}

func NewPreviewImportHandler(invoices InvoiceLookup) *PreviewImportHandler {
	return &PreviewImportHandler{invoices: invoices}
}

// Handle parses the CSV and returns match results.
func (h *PreviewImportHandler) Handle(ctx context.Context, cmd PreviewImportCommand) ([]domain.MatchResult, error) {
	r := csv.NewReader(bytes.NewReader(cmd.CSVData))
	r.Comma = ';'
	r.TrimLeadingSpace = true

	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse csv: %w", err)
	}

	var results []domain.MatchResult
	for i, rec := range records {
		// Skip header row
		if i == 0 {
			continue
		}
		if len(rec) < 5 {
			continue
		}

		row := domain.CSVRow{
			Date:      strings.TrimSpace(rec[0]),
			Amount:    strings.TrimSpace(rec[1]),
			Reference: strings.TrimSpace(rec[2]),
			Payer:     strings.TrimSpace(rec[3]),
			IBAN:      strings.TrimSpace(rec[4]),
		}

		bookingDate, _ := time.Parse("2006-01-02", row.Date)

		result := domain.MatchResult{
			Row:         row,
			BookingDate: bookingDate,
		}

		// Attempt to match by reference (invoice number embedded in reference)
		invID, invNum, invStatus, err := h.invoices.FindByNumber(ctx, row.Reference)
		if err != nil || invID == "" {
			result.Confidence = domain.ConfidenceUnmatched
		} else {
			result.InvoiceID = invID
			result.InvoiceNumber = invNum
			result.InvoiceStatus = invStatus
			result.Confidence = domain.ConfidenceExact
		}

		results = append(results, result)
	}

	return results, nil
}
