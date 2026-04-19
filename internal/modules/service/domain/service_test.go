package domain

import (
	"errors"
	"testing"
	"time"
)

func validService(t *testing.T) *Service {
	t.Helper()
	s, err := NewService("id-1", "cust-1", "prod-1", "Fiber 100", 2999, "EUR", time.Now(), "monthly")
	if err != nil {
		t.Fatalf("unexpected error building valid service: %v", err)
	}
	return s
}

func TestNewService(t *testing.T) {
	t.Run("valid creation", func(t *testing.T) {
		start := time.Now()
		s, err := NewService("id-1", "cust-1", "prod-1", "Fiber 100", 2999, "EUR", start, "monthly")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if s.ID != "id-1" {
			t.Errorf("ID: want %q, got %q", "id-1", s.ID)
		}
		if s.CustomerID != "cust-1" {
			t.Errorf("CustomerID: want %q, got %q", "cust-1", s.CustomerID)
		}
		if s.ProductID != "prod-1" {
			t.Errorf("ProductID: want %q, got %q", "prod-1", s.ProductID)
		}
		if s.ProductName != "Fiber 100" {
			t.Errorf("ProductName: want %q, got %q", "Fiber 100", s.ProductName)
		}
		if s.PriceAmount != 2999 {
			t.Errorf("PriceAmount: want 2999, got %d", s.PriceAmount)
		}
		if s.Currency != "EUR" {
			t.Errorf("Currency: want %q, got %q", "EUR", s.Currency)
		}
		if s.Status != StatusActive {
			t.Errorf("Status: want %q, got %q", StatusActive, s.Status)
		}
		if s.BillingCycle != "monthly" {
			t.Errorf("BillingCycle: want %q, got %q", "monthly", s.BillingCycle)
		}
		if s.NextBillingDate == nil {
			t.Fatal("NextBillingDate: want non-nil, got nil")
		}
		want := start.AddDate(0, 1, 0)
		if !s.NextBillingDate.Equal(want) {
			t.Errorf("NextBillingDate: want %v, got %v", want, *s.NextBillingDate)
		}
	})

	t.Run("missing customerID error", func(t *testing.T) {
		_, err := NewService("id-1", "", "prod-1", "Fiber 100", 0, "EUR", time.Now(), "monthly")
		if err == nil {
			t.Fatal("expected error for empty customerID, got nil")
		}
	})

	t.Run("missing productID error", func(t *testing.T) {
		_, err := NewService("id-1", "cust-1", "", "Fiber 100", 0, "EUR", time.Now(), "monthly")
		if err == nil {
			t.Fatal("expected error for empty productID, got nil")
		}
	})

	t.Run("missing productName error", func(t *testing.T) {
		_, err := NewService("id-1", "cust-1", "prod-1", "", 0, "EUR", time.Now(), "monthly")
		if err == nil {
			t.Fatal("expected error for empty productName, got nil")
		}
	})

	t.Run("default currency EUR", func(t *testing.T) {
		s, err := NewService("id-1", "cust-1", "prod-1", "Fiber 100", 2999, "", time.Now(), "monthly")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if s.Currency != "EUR" {
			t.Errorf("Currency: want %q, got %q", "EUR", s.Currency)
		}
	})
}

func TestSuspend(t *testing.T) {
	t.Run("active to suspended succeeds", func(t *testing.T) {
		s := validService(t)
		if err := s.Suspend(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if s.Status != StatusSuspended {
			t.Errorf("Status: want %q, got %q", StatusSuspended, s.Status)
		}
	})

	t.Run("suspended to suspended returns ErrInvalidTransition", func(t *testing.T) {
		s := validService(t)
		_ = s.Suspend()
		err := s.Suspend()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("cancelled to suspended returns ErrInvalidTransition", func(t *testing.T) {
		s := validService(t)
		_ = s.Cancel()
		err := s.Suspend()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})
}

func TestReactivate(t *testing.T) {
	t.Run("suspended to active succeeds", func(t *testing.T) {
		s := validService(t)
		_ = s.Suspend()
		if err := s.Reactivate(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if s.Status != StatusActive {
			t.Errorf("Status: want %q, got %q", StatusActive, s.Status)
		}
	})

	t.Run("active to active returns ErrInvalidTransition", func(t *testing.T) {
		s := validService(t)
		err := s.Reactivate()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})
}

func TestCancel(t *testing.T) {
	t.Run("active to cancelled succeeds", func(t *testing.T) {
		s := validService(t)
		if err := s.Cancel(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if s.Status != StatusCancelled {
			t.Errorf("Status: want %q, got %q", StatusCancelled, s.Status)
		}
	})

	t.Run("suspended to cancelled succeeds", func(t *testing.T) {
		s := validService(t)
		_ = s.Suspend()
		if err := s.Cancel(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if s.Status != StatusCancelled {
			t.Errorf("Status: want %q, got %q", StatusCancelled, s.Status)
		}
	})

	t.Run("cancelled to cancelled returns ErrInvalidTransition", func(t *testing.T) {
		s := validService(t)
		_ = s.Cancel()
		err := s.Cancel()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})
}

func TestBillingCycle(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("quarterly NextBillingDate is +3 months", func(t *testing.T) {
		s, err := NewService("id-1", "cust-1", "prod-1", "VoIP", 999, "EUR", base, "quarterly")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := base.AddDate(0, 3, 0)
		if s.NextBillingDate == nil || !s.NextBillingDate.Equal(want) {
			t.Errorf("NextBillingDate: want %v, got %v", want, s.NextBillingDate)
		}
	})

	t.Run("yearly NextBillingDate is +1 year", func(t *testing.T) {
		s, err := NewService("id-1", "cust-1", "prod-1", "Hosting", 4999, "EUR", base, "yearly")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := base.AddDate(1, 0, 0)
		if s.NextBillingDate == nil || !s.NextBillingDate.Equal(want) {
			t.Errorf("NextBillingDate: want %v, got %v", want, s.NextBillingDate)
		}
	})
}

func TestAdvanceNextBillingDate(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("monthly advances by 1 month", func(t *testing.T) {
		s, _ := NewService("id-1", "cust-1", "prod-1", "Fiber", 2999, "EUR", base, "monthly")
		s.AdvanceNextBillingDate()
		want := base.AddDate(0, 2, 0) // start+1mo → advance again → start+2mo
		if s.NextBillingDate == nil || !s.NextBillingDate.Equal(want) {
			t.Errorf("want %v, got %v", want, s.NextBillingDate)
		}
	})

	t.Run("quarterly advances by 3 months", func(t *testing.T) {
		s, _ := NewService("id-1", "cust-1", "prod-1", "VoIP", 999, "EUR", base, "quarterly")
		s.AdvanceNextBillingDate()
		want := base.AddDate(0, 6, 0)
		if s.NextBillingDate == nil || !s.NextBillingDate.Equal(want) {
			t.Errorf("want %v, got %v", want, s.NextBillingDate)
		}
	})

	t.Run("nil NextBillingDate is a no-op", func(t *testing.T) {
		s := &Service{BillingCycle: "monthly", NextBillingDate: nil}
		s.AdvanceNextBillingDate() // must not panic
		if s.NextBillingDate != nil {
			t.Error("want nil, still nil after no-op")
		}
	})
}
