package domain

import (
	"testing"
	"time"

	"github.com/vvs/isp/internal/shared/domain"
)

func TestNewPayment(t *testing.T) {
	p := NewPayment(
		domain.EUR(15000),
		"INV-001",
		"Acme Corp",
		"DE89370400440532013000",
		time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		"batch-1",
	)

	if p.Amount.Amount != 15000 {
		t.Errorf("expected amount 15000, got %d", p.Amount.Amount)
	}
	if p.Reference != "INV-001" {
		t.Errorf("expected reference INV-001, got %s", p.Reference)
	}
	if p.PayerName != "Acme Corp" {
		t.Errorf("expected payer name Acme Corp, got %s", p.PayerName)
	}
	if p.Status != StatusImported {
		t.Errorf("expected status imported, got %s", p.Status)
	}
	if p.ImportBatchID != "batch-1" {
		t.Errorf("expected batch batch-1, got %s", p.ImportBatchID)
	}
}

func TestNewPayment_EmptyReference(t *testing.T) {
	p := NewPayment(
		domain.EUR(5000),
		"",
		"Acme Corp",
		"DE89370400440532013000",
		time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		"batch-1",
	)

	if p.Status != StatusUnmatched {
		t.Errorf("expected unmatched status for empty reference, got %s", p.Status)
	}
}

func TestMatch(t *testing.T) {
	p := NewPayment(
		domain.EUR(15000),
		"INV-001",
		"Acme Corp",
		"DE89370400440532013000",
		time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		"batch-1",
	)

	err := p.Match("inv-123", "cust-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status != StatusManuallyMatched {
		t.Errorf("expected manually_matched, got %s", p.Status)
	}
	if *p.InvoiceID != "inv-123" {
		t.Errorf("expected invoice ID inv-123, got %s", *p.InvoiceID)
	}
	if *p.CustomerID != "cust-456" {
		t.Errorf("expected customer ID cust-456, got %s", *p.CustomerID)
	}

	// Cannot match again
	err = p.Match("inv-999", "cust-999")
	if err != ErrAlreadyMatched {
		t.Errorf("expected ErrAlreadyMatched, got %v", err)
	}
}

func TestMatch_Validation(t *testing.T) {
	p := NewPayment(domain.EUR(5000), "REF", "Payer", "IBAN", time.Now(), "b1")

	if err := p.Match("", "cust-1"); err != ErrInvoiceIDRequired {
		t.Errorf("expected ErrInvoiceIDRequired, got %v", err)
	}
	if err := p.Match("inv-1", ""); err != ErrCustomerIDRequired {
		t.Errorf("expected ErrCustomerIDRequired, got %v", err)
	}
}

func TestUnmatch(t *testing.T) {
	p := NewPayment(domain.EUR(5000), "REF", "Payer", "IBAN", time.Now(), "b1")
	p.Match("inv-1", "cust-1")

	err := p.Unmatch()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status != StatusUnmatched {
		t.Errorf("expected unmatched, got %s", p.Status)
	}
	if p.InvoiceID != nil {
		t.Error("expected nil InvoiceID")
	}
	if p.CustomerID != nil {
		t.Error("expected nil CustomerID")
	}

	// Cannot unmatch again
	err = p.Unmatch()
	if err != ErrNotMatched {
		t.Errorf("expected ErrNotMatched, got %v", err)
	}
}
