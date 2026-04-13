package domain

import (
	"testing"
	"time"

	"github.com/vvs/isp/internal/shared/domain"
)

func TestNewInvoice(t *testing.T) {
	issue := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	due := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)

	t.Run("valid invoice", func(t *testing.T) {
		inv, err := NewInvoice("INV-2026-00001", "cust-1", "Acme Corp", issue, due)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if inv.InvoiceNumber != "INV-2026-00001" {
			t.Errorf("expected INV-2026-00001, got %s", inv.InvoiceNumber)
		}
		if inv.Status != StatusDraft {
			t.Errorf("expected draft status, got %s", inv.Status)
		}
		if inv.TaxRate != 21 {
			t.Errorf("expected tax rate 21, got %d", inv.TaxRate)
		}
	})

	t.Run("empty number", func(t *testing.T) {
		_, err := NewInvoice("", "cust-1", "Acme Corp", issue, due)
		if err != ErrInvoiceNumberRequired {
			t.Errorf("expected ErrInvoiceNumberRequired, got %v", err)
		}
	})

	t.Run("empty customer ID", func(t *testing.T) {
		_, err := NewInvoice("INV-2026-00001", "", "Acme Corp", issue, due)
		if err != ErrCustomerIDRequired {
			t.Errorf("expected ErrCustomerIDRequired, got %v", err)
		}
	})

	t.Run("empty customer name", func(t *testing.T) {
		_, err := NewInvoice("INV-2026-00001", "cust-1", "", issue, due)
		if err != ErrCustomerNameRequired {
			t.Errorf("expected ErrCustomerNameRequired, got %v", err)
		}
	})

	t.Run("due date before issue date", func(t *testing.T) {
		_, err := NewInvoice("INV-2026-00001", "cust-1", "Acme Corp", due, issue)
		if err != ErrInvalidDueDate {
			t.Errorf("expected ErrInvalidDueDate, got %v", err)
		}
	})
}

func TestAddLine(t *testing.T) {
	inv := newTestInvoice(t)

	t.Run("add line recalculates", func(t *testing.T) {
		err := inv.AddLine("prod-1", "Internet 100Mbps", "Monthly subscription", 1, domain.EUR(5000))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(inv.Lines) != 1 {
			t.Fatalf("expected 1 line, got %d", len(inv.Lines))
		}
		if inv.Lines[0].Total.Amount != 5000 {
			t.Errorf("expected line total 5000, got %d", inv.Lines[0].Total.Amount)
		}
		if inv.Subtotal.Amount != 5000 {
			t.Errorf("expected subtotal 5000, got %d", inv.Subtotal.Amount)
		}
		// 5000 * 21 / 100 = 1050
		if inv.TaxAmount.Amount != 1050 {
			t.Errorf("expected tax 1050, got %d", inv.TaxAmount.Amount)
		}
		if inv.Total.Amount != 6050 {
			t.Errorf("expected total 6050, got %d", inv.Total.Amount)
		}
	})

	t.Run("add multiple lines", func(t *testing.T) {
		err := inv.AddLine("prod-2", "Router", "Equipment rental", 2, domain.EUR(1000))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(inv.Lines) != 2 {
			t.Fatalf("expected 2 lines, got %d", len(inv.Lines))
		}
		// 5000 + 2000 = 7000
		if inv.Subtotal.Amount != 7000 {
			t.Errorf("expected subtotal 7000, got %d", inv.Subtotal.Amount)
		}
	})

	t.Run("invalid quantity", func(t *testing.T) {
		err := inv.AddLine("prod-3", "Test", "Test", 0, domain.EUR(1000))
		if err != ErrInvalidQuantity {
			t.Errorf("expected ErrInvalidQuantity, got %v", err)
		}
	})
}

func TestRemoveLine(t *testing.T) {
	inv := newTestInvoice(t)
	inv.AddLine("prod-1", "Internet", "Monthly", 1, domain.EUR(5000))
	inv.AddLine("prod-2", "Router", "Equipment", 1, domain.EUR(2000))
	lineID := inv.Lines[0].ID

	t.Run("remove existing line", func(t *testing.T) {
		err := inv.RemoveLine(lineID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(inv.Lines) != 1 {
			t.Fatalf("expected 1 line, got %d", len(inv.Lines))
		}
		if inv.Subtotal.Amount != 2000 {
			t.Errorf("expected subtotal 2000, got %d", inv.Subtotal.Amount)
		}
	})

	t.Run("remove non-existent line", func(t *testing.T) {
		err := inv.RemoveLine("non-existent")
		if err != ErrLineNotFound {
			t.Errorf("expected ErrLineNotFound, got %v", err)
		}
	})
}

func TestRecalculate(t *testing.T) {
	inv := newTestInvoice(t)
	inv.AddLine("prod-1", "Internet", "Monthly", 3, domain.EUR(10000))

	// 3 * 10000 = 30000 subtotal
	if inv.Subtotal.Amount != 30000 {
		t.Errorf("expected subtotal 30000, got %d", inv.Subtotal.Amount)
	}
	// 30000 * 21 / 100 = 6300
	if inv.TaxAmount.Amount != 6300 {
		t.Errorf("expected tax 6300, got %d", inv.TaxAmount.Amount)
	}
	// 30000 + 6300 = 36300
	if inv.Total.Amount != 36300 {
		t.Errorf("expected total 36300, got %d", inv.Total.Amount)
	}
}

func TestFinalize(t *testing.T) {
	t.Run("finalize from draft", func(t *testing.T) {
		inv := newTestInvoice(t)
		err := inv.Finalize()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if inv.Status != StatusFinalized {
			t.Errorf("expected finalized, got %s", inv.Status)
		}
	})

	t.Run("finalize from non-draft", func(t *testing.T) {
		inv := newTestInvoice(t)
		inv.Finalize()
		err := inv.Finalize()
		if err != ErrInvoiceNotDraft {
			t.Errorf("expected ErrInvoiceNotDraft, got %v", err)
		}
	})
}

func TestMarkPaid(t *testing.T) {
	paidDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	t.Run("mark paid from finalized", func(t *testing.T) {
		inv := newTestInvoice(t)
		inv.Finalize()
		err := inv.MarkPaid(paidDate)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if inv.Status != StatusPaid {
			t.Errorf("expected paid, got %s", inv.Status)
		}
		if inv.PaidDate == nil || !inv.PaidDate.Equal(paidDate) {
			t.Errorf("expected paid date %v, got %v", paidDate, inv.PaidDate)
		}
	})

	t.Run("mark paid from draft", func(t *testing.T) {
		inv := newTestInvoice(t)
		err := inv.MarkPaid(paidDate)
		if err != ErrCannotMarkPaid {
			t.Errorf("expected ErrCannotMarkPaid, got %v", err)
		}
	})
}

func TestVoid(t *testing.T) {
	t.Run("void from draft", func(t *testing.T) {
		inv := newTestInvoice(t)
		err := inv.Void()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if inv.Status != StatusVoid {
			t.Errorf("expected void, got %s", inv.Status)
		}
	})

	t.Run("void from finalized", func(t *testing.T) {
		inv := newTestInvoice(t)
		inv.Finalize()
		err := inv.Void()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if inv.Status != StatusVoid {
			t.Errorf("expected void, got %s", inv.Status)
		}
	})

	t.Run("void from paid", func(t *testing.T) {
		inv := newTestInvoice(t)
		inv.Finalize()
		inv.MarkPaid(time.Now())
		err := inv.Void()
		if err != ErrCannotVoid {
			t.Errorf("expected ErrCannotVoid, got %v", err)
		}
	})
}

func newTestInvoice(t *testing.T) *Invoice {
	t.Helper()
	issue := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	due := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	inv, err := NewInvoice("INV-2026-00001", "cust-1", "Acme Corp", issue, due)
	if err != nil {
		t.Fatalf("failed to create test invoice: %v", err)
	}
	return inv
}
