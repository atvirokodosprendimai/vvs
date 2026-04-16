package domain

import (
	"errors"
	"testing"
)

func validTicket(t *testing.T) *Ticket {
	t.Helper()
	tk, err := NewTicket("id-1", "cust-1", "Internet is down", "No connectivity since morning", PriorityNormal)
	if err != nil {
		t.Fatalf("unexpected error building valid ticket: %v", err)
	}
	return tk
}

func TestNewTicket(t *testing.T) {
	t.Run("valid creation", func(t *testing.T) {
		tk, err := NewTicket("id-1", "cust-1", "Subject", "Body", PriorityHigh)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if tk.ID != "id-1" {
			t.Errorf("ID: want %q, got %q", "id-1", tk.ID)
		}
		if tk.CustomerID != "cust-1" {
			t.Errorf("CustomerID: want %q, got %q", "cust-1", tk.CustomerID)
		}
		if tk.Subject != "Subject" {
			t.Errorf("Subject: want %q, got %q", "Subject", tk.Subject)
		}
		if tk.Status != StatusOpen {
			t.Errorf("Status: want %q, got %q", StatusOpen, tk.Status)
		}
		if tk.Priority != PriorityHigh {
			t.Errorf("Priority: want %q, got %q", PriorityHigh, tk.Priority)
		}
	})

	t.Run("empty customerID returns error", func(t *testing.T) {
		_, err := NewTicket("id-1", "", "Subject", "", PriorityNormal)
		if err == nil {
			t.Fatal("expected error for empty customerID, got nil")
		}
	})

	t.Run("empty subject returns ErrSubjectRequired", func(t *testing.T) {
		_, err := NewTicket("id-1", "cust-1", "", "", PriorityNormal)
		if !errors.Is(err, ErrSubjectRequired) {
			t.Errorf("expected ErrSubjectRequired, got %v", err)
		}
	})

	t.Run("empty priority defaults to normal", func(t *testing.T) {
		tk, err := NewTicket("id-1", "cust-1", "Subject", "", "")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if tk.Priority != PriorityNormal {
			t.Errorf("Priority: want %q, got %q", PriorityNormal, tk.Priority)
		}
	})
}

func TestStartWork(t *testing.T) {
	t.Run("open to in_progress succeeds", func(t *testing.T) {
		tk := validTicket(t)
		if err := tk.StartWork(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if tk.Status != StatusInProgress {
			t.Errorf("Status: want %q, got %q", StatusInProgress, tk.Status)
		}
	})

	t.Run("in_progress to in_progress returns ErrInvalidTransition", func(t *testing.T) {
		tk := validTicket(t)
		_ = tk.StartWork()
		err := tk.StartWork()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("resolved to in_progress returns ErrInvalidTransition", func(t *testing.T) {
		tk := validTicket(t)
		_ = tk.StartWork()
		_ = tk.Resolve()
		err := tk.StartWork()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("closed to in_progress returns ErrInvalidTransition", func(t *testing.T) {
		tk := validTicket(t)
		_ = tk.Close()
		err := tk.StartWork()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})
}

func TestResolve(t *testing.T) {
	t.Run("in_progress to resolved succeeds", func(t *testing.T) {
		tk := validTicket(t)
		_ = tk.StartWork()
		if err := tk.Resolve(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if tk.Status != StatusResolved {
			t.Errorf("Status: want %q, got %q", StatusResolved, tk.Status)
		}
	})

	t.Run("open to resolved returns ErrInvalidTransition", func(t *testing.T) {
		tk := validTicket(t)
		err := tk.Resolve()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("resolved to resolved returns ErrInvalidTransition", func(t *testing.T) {
		tk := validTicket(t)
		_ = tk.StartWork()
		_ = tk.Resolve()
		err := tk.Resolve()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})
}

func TestClose(t *testing.T) {
	t.Run("open to closed succeeds", func(t *testing.T) {
		tk := validTicket(t)
		if err := tk.Close(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if tk.Status != StatusClosed {
			t.Errorf("Status: want %q, got %q", StatusClosed, tk.Status)
		}
	})

	t.Run("in_progress to closed succeeds", func(t *testing.T) {
		tk := validTicket(t)
		_ = tk.StartWork()
		if err := tk.Close(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if tk.Status != StatusClosed {
			t.Errorf("Status: want %q, got %q", StatusClosed, tk.Status)
		}
	})

	t.Run("resolved to closed succeeds", func(t *testing.T) {
		tk := validTicket(t)
		_ = tk.StartWork()
		_ = tk.Resolve()
		if err := tk.Close(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if tk.Status != StatusClosed {
			t.Errorf("Status: want %q, got %q", StatusClosed, tk.Status)
		}
	})

	t.Run("closed to closed returns ErrAlreadyClosed", func(t *testing.T) {
		tk := validTicket(t)
		_ = tk.Close()
		err := tk.Close()
		if !errors.Is(err, ErrAlreadyClosed) {
			t.Errorf("expected ErrAlreadyClosed, got %v", err)
		}
	})
}

func TestReopen(t *testing.T) {
	t.Run("resolved to open succeeds", func(t *testing.T) {
		tk := validTicket(t)
		_ = tk.StartWork()
		_ = tk.Resolve()
		if err := tk.Reopen(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if tk.Status != StatusOpen {
			t.Errorf("Status: want %q, got %q", StatusOpen, tk.Status)
		}
	})

	t.Run("closed to open succeeds", func(t *testing.T) {
		tk := validTicket(t)
		_ = tk.Close()
		if err := tk.Reopen(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if tk.Status != StatusOpen {
			t.Errorf("Status: want %q, got %q", StatusOpen, tk.Status)
		}
	})

	t.Run("open to open returns ErrInvalidTransition", func(t *testing.T) {
		tk := validTicket(t)
		err := tk.Reopen()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("in_progress to open returns ErrInvalidTransition", func(t *testing.T) {
		tk := validTicket(t)
		_ = tk.StartWork()
		err := tk.Reopen()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})
}
