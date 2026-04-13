package domain

import (
	"testing"
	"time"

	"github.com/vvs/isp/internal/shared/domain"
)

func TestNewRecurringInvoice(t *testing.T) {
	ri, err := NewRecurringInvoice("cust-1", "Acme Corp", "monthly", 15)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ri.CustomerID != "cust-1" {
		t.Errorf("expected customer ID cust-1, got %s", ri.CustomerID)
	}
	if ri.CustomerName != "Acme Corp" {
		t.Errorf("expected customer name Acme Corp, got %s", ri.CustomerName)
	}
	if ri.Schedule.Frequency != FrequencyMonthly {
		t.Errorf("expected monthly frequency, got %s", ri.Schedule.Frequency)
	}
	if ri.Schedule.DayOfMonth != 15 {
		t.Errorf("expected day 15, got %d", ri.Schedule.DayOfMonth)
	}
	if ri.Status != StatusActive {
		t.Errorf("expected active status, got %s", ri.Status)
	}
}

func TestNewRecurringInvoice_Validation(t *testing.T) {
	tests := []struct {
		name       string
		custID     string
		custName   string
		frequency  string
		dayOfMonth int
		wantErr    error
	}{
		{"empty customer ID", "", "Acme", "monthly", 1, ErrCustomerIDRequired},
		{"empty customer name", "c1", "", "monthly", 1, ErrCustomerNameRequired},
		{"invalid frequency", "c1", "Acme", "weekly", 1, ErrInvalidFrequency},
		{"day too low", "c1", "Acme", "monthly", 0, ErrInvalidDayOfMonth},
		{"day too high", "c1", "Acme", "monthly", 29, ErrInvalidDayOfMonth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRecurringInvoice(tt.custID, tt.custName, tt.frequency, tt.dayOfMonth)
			if err != tt.wantErr {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestAddLine(t *testing.T) {
	ri, _ := NewRecurringInvoice("cust-1", "Acme Corp", "monthly", 1)
	err := ri.AddLine("prod-1", "Internet 100Mbps", "Monthly internet", 1, domain.EUR(5000))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ri.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(ri.Lines))
	}
	if ri.Lines[0].ProductName != "Internet 100Mbps" {
		t.Errorf("expected product name Internet 100Mbps, got %s", ri.Lines[0].ProductName)
	}
}

func TestAddLine_InvalidQuantity(t *testing.T) {
	ri, _ := NewRecurringInvoice("cust-1", "Acme Corp", "monthly", 1)
	err := ri.AddLine("prod-1", "Internet", "desc", 0, domain.EUR(5000))
	if err != ErrInvalidQuantity {
		t.Errorf("expected ErrInvalidQuantity, got %v", err)
	}
}

func TestPauseResume(t *testing.T) {
	ri, _ := NewRecurringInvoice("cust-1", "Acme Corp", "monthly", 1)

	if err := ri.Pause(); err != nil {
		t.Fatalf("unexpected error on pause: %v", err)
	}
	if ri.Status != StatusPaused {
		t.Errorf("expected paused, got %s", ri.Status)
	}

	if err := ri.Pause(); err != ErrAlreadyPaused {
		t.Errorf("expected ErrAlreadyPaused, got %v", err)
	}

	if err := ri.Resume(); err != nil {
		t.Fatalf("unexpected error on resume: %v", err)
	}
	if ri.Status != StatusActive {
		t.Errorf("expected active, got %s", ri.Status)
	}

	if err := ri.Resume(); err != ErrAlreadyActive {
		t.Errorf("expected ErrAlreadyActive, got %v", err)
	}
}

func TestCancel(t *testing.T) {
	ri, _ := NewRecurringInvoice("cust-1", "Acme Corp", "monthly", 1)

	if err := ri.Cancel(); err != nil {
		t.Fatalf("unexpected error on cancel: %v", err)
	}
	if ri.Status != StatusCancelled {
		t.Errorf("expected cancelled, got %s", ri.Status)
	}

	if err := ri.Pause(); err != ErrAlreadyCancelled {
		t.Errorf("expected ErrAlreadyCancelled on pause, got %v", err)
	}
	if err := ri.Resume(); err != ErrAlreadyCancelled {
		t.Errorf("expected ErrAlreadyCancelled on resume, got %v", err)
	}
}

func TestIsDue(t *testing.T) {
	ri, _ := NewRecurringInvoice("cust-1", "Acme Corp", "monthly", 1)
	ri.NextRunDate = time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	if !ri.IsDue(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("should be due on the run date")
	}
	if !ri.IsDue(time.Date(2025, 6, 2, 0, 0, 0, 0, time.UTC)) {
		t.Error("should be due after the run date")
	}
	if ri.IsDue(time.Date(2025, 5, 31, 0, 0, 0, 0, time.UTC)) {
		t.Error("should not be due before the run date")
	}

	ri.Status = StatusPaused
	if ri.IsDue(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("paused invoice should not be due")
	}
}

func TestTotal(t *testing.T) {
	ri, _ := NewRecurringInvoice("cust-1", "Acme Corp", "monthly", 1)
	ri.AddLine("", "Internet", "", 1, domain.EUR(5000))
	ri.AddLine("", "Support", "", 2, domain.EUR(1500))

	total := ri.Total()
	if total.Amount != 8000 {
		t.Errorf("expected total 8000, got %d", total.Amount)
	}
}
