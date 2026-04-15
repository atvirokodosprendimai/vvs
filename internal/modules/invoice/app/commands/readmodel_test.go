package commands

import (
	"testing"
	"time"

	"github.com/vvs/isp/internal/modules/invoice/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
)

func TestDomainToReadModel_MapsFieldsCorrectly(t *testing.T) {
	issue := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	due := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)

	inv, err := domain.NewInvoice("INV-2026-00001", "cust-1", "Acme Corp", issue, due)
	if err != nil {
		t.Fatalf("NewInvoice: %v", err)
	}
	inv.TaxRate = 21
	inv.AddLine("p1", "Internet 100M", "Monthly", 1, shareddomain.EUR(2999))

	rm := domainToReadModel(inv)

	if rm.ID != inv.ID {
		t.Errorf("ID: want %q; got %q", inv.ID, rm.ID)
	}
	if rm.InvoiceNumber != "INV-2026-00001" {
		t.Errorf("InvoiceNumber: want INV-2026-00001; got %q", rm.InvoiceNumber)
	}
	if rm.CustomerID != "cust-1" {
		t.Errorf("CustomerID: want cust-1; got %q", rm.CustomerID)
	}
	if rm.CustomerName != "Acme Corp" {
		t.Errorf("CustomerName: want Acme Corp; got %q", rm.CustomerName)
	}
	if rm.SubtotalAmount != inv.Subtotal.Amount {
		t.Errorf("SubtotalAmount: want %d; got %d", inv.Subtotal.Amount, rm.SubtotalAmount)
	}
	if rm.TaxRate != 21 {
		t.Errorf("TaxRate: want 21; got %d", rm.TaxRate)
	}
	if rm.TotalAmount != inv.Total.Amount {
		t.Errorf("TotalAmount: want %d; got %d", inv.Total.Amount, rm.TotalAmount)
	}
	if rm.TotalAmount == 0 {
		t.Error("TotalAmount must not be 0 after adding a line")
	}
	if rm.Status != string(domain.StatusDraft) {
		t.Errorf("Status: want draft; got %q", rm.Status)
	}
	if !rm.IssueDate.Equal(issue) {
		t.Errorf("IssueDate: want %v; got %v", issue, rm.IssueDate)
	}
	if !rm.DueDate.Equal(due) {
		t.Errorf("DueDate: want %v; got %v", due, rm.DueDate)
	}
}
