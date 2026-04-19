package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/vvs/isp/internal/modules/payment/app/commands"
	"github.com/vvs/isp/internal/modules/payment/domain"
	invoicecommands "github.com/vvs/isp/internal/modules/invoice/app/commands"
	invoicedomain "github.com/vvs/isp/internal/modules/invoice/domain"
)

// ── mocks ────────────────────────────────────────────────────────────

type mockLookup struct {
	invoices map[string]*invoicedomain.Invoice
}

func (m *mockLookup) FindByCode(_ context.Context, code string) (*invoicedomain.Invoice, error) {
	inv, ok := m.invoices[code]
	if !ok {
		return nil, invoicedomain.ErrInvoiceNotFound
	}
	return inv, nil
}

type mockMarker struct {
	results map[string]*invoicedomain.Invoice
	errs    map[string]error
}

func (m *mockMarker) Handle(_ context.Context, cmd invoicecommands.MarkPaidCommand) (*invoicedomain.Invoice, error) {
	if err, ok := m.errs[cmd.InvoiceID]; ok {
		return nil, err
	}
	inv, ok := m.results[cmd.InvoiceID]
	if !ok {
		return nil, invoicedomain.ErrInvoiceNotFound
	}
	return inv, nil
}

// ── PreviewImportHandler ─────────────────────────────────────────────

func TestPreviewImportHandler_MatchFound(t *testing.T) {
	csv := []byte("Date;Beneficiary;IBAN;Amount;Currency;Reference;Description\n" +
		"2026-04-01;UAB Klientas;LT123;100.00;EUR;INV-001;Payment\n")
	lookup := &mockLookup{invoices: map[string]*invoicedomain.Invoice{
		"INV-001": {ID: "id1", Code: "INV-001", TotalAmount: 10000, Status: invoicedomain.StatusFinalized},
	}}
	h := commands.NewPreviewImportHandler(lookup)
	results, err := h.Handle(context.Background(), commands.PreviewImportCommand{CSVData: csv})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if results[0].Confidence != domain.ConfidenceExact {
		t.Errorf("want exact, got %s", results[0].Confidence)
	}
}

func TestPreviewImportHandler_NotFound(t *testing.T) {
	csv := []byte("Date;Beneficiary;IBAN;Amount;Currency;Reference;Description\n" +
		"2026-04-01;UAB Klientas;LT123;100.00;EUR;INV-999;Payment\n")
	lookup := &mockLookup{invoices: map[string]*invoicedomain.Invoice{}}
	h := commands.NewPreviewImportHandler(lookup)
	results, err := h.Handle(context.Background(), commands.PreviewImportCommand{CSVData: csv})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Confidence != domain.ConfidenceUnmatched {
		t.Errorf("want unmatched, got %s", results[0].Confidence)
	}
}

func TestPreviewImportHandler_NoCodeInCSV(t *testing.T) {
	csv := []byte("Date;Beneficiary;IBAN;Amount;Currency;Reference;Description\n" +
		"2026-04-01;UAB Klientas;LT123;100.00;EUR;wire transfer;Monthly payment\n")
	lookup := &mockLookup{invoices: map[string]*invoicedomain.Invoice{}}
	h := commands.NewPreviewImportHandler(lookup)
	results, err := h.Handle(context.Background(), commands.PreviewImportCommand{CSVData: csv})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Confidence != domain.ConfidenceUnmatched {
		t.Errorf("want unmatched, got %s", results[0].Confidence)
	}
}

// ── ConfirmImportHandler ─────────────────────────────────────────────

func TestConfirmImportHandler_AllSuccess(t *testing.T) {
	paid := &invoicedomain.Invoice{ID: "id1", Status: invoicedomain.StatusPaid}
	marker := &mockMarker{
		results: map[string]*invoicedomain.Invoice{"id1": paid, "id2": paid},
		errs:    map[string]error{},
	}
	h := commands.NewConfirmImportHandler(marker)
	res, err := h.Handle(context.Background(), commands.ConfirmImportCommand{InvoiceIDs: []string{"id1", "id2"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.MarkedPaid) != 2 {
		t.Errorf("want 2 marked paid, got %d", len(res.MarkedPaid))
	}
	if len(res.Errors) != 0 {
		t.Errorf("want 0 errors, got %d", len(res.Errors))
	}
}

func TestConfirmImportHandler_PartialError(t *testing.T) {
	paid := &invoicedomain.Invoice{ID: "id1", Status: invoicedomain.StatusPaid}
	marker := &mockMarker{
		results: map[string]*invoicedomain.Invoice{"id1": paid},
		errs:    map[string]error{"id-bad": errors.New("invalid transition")},
	}
	h := commands.NewConfirmImportHandler(marker)
	res, _ := h.Handle(context.Background(), commands.ConfirmImportCommand{InvoiceIDs: []string{"id1", "id-bad"}})
	if len(res.MarkedPaid) != 1 {
		t.Errorf("want 1 marked paid, got %d", len(res.MarkedPaid))
	}
	if len(res.Errors) != 1 {
		t.Errorf("want 1 error, got %d", len(res.Errors))
	}
}

func TestConfirmImportHandler_Empty(t *testing.T) {
	marker := &mockMarker{results: map[string]*invoicedomain.Invoice{}, errs: map[string]error{}}
	h := commands.NewConfirmImportHandler(marker)
	res, err := h.Handle(context.Background(), commands.ConfirmImportCommand{InvoiceIDs: []string{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.MarkedPaid) != 0 || len(res.Errors) != 0 {
		t.Errorf("want empty result, got paid=%d errs=%d", len(res.MarkedPaid), len(res.Errors))
	}
}
