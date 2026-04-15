package domain

import (
	"errors"
	"testing"
	"time"
)

func validService(t *testing.T) *Service {
	t.Helper()
	s, err := NewService("id-1", "cust-1", "prod-1", "Fiber 100", 2999, "EUR", time.Now())
	if err != nil {
		t.Fatalf("unexpected error building valid service: %v", err)
	}
	return s
}

func TestNewService(t *testing.T) {
	t.Run("valid creation", func(t *testing.T) {
		start := time.Now()
		s, err := NewService("id-1", "cust-1", "prod-1", "Fiber 100", 2999, "EUR", start)
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
	})

	t.Run("missing customerID error", func(t *testing.T) {
		_, err := NewService("id-1", "", "prod-1", "Fiber 100", 0, "EUR", time.Now())
		if err == nil {
			t.Fatal("expected error for empty customerID, got nil")
		}
	})

	t.Run("missing productID error", func(t *testing.T) {
		_, err := NewService("id-1", "cust-1", "", "Fiber 100", 0, "EUR", time.Now())
		if err == nil {
			t.Fatal("expected error for empty productID, got nil")
		}
	})

	t.Run("missing productName error", func(t *testing.T) {
		_, err := NewService("id-1", "cust-1", "prod-1", "", 0, "EUR", time.Now())
		if err == nil {
			t.Fatal("expected error for empty productName, got nil")
		}
	})

	t.Run("default currency EUR", func(t *testing.T) {
		s, err := NewService("id-1", "cust-1", "prod-1", "Fiber 100", 2999, "", time.Now())
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
