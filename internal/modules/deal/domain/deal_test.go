package domain

import (
	"errors"
	"testing"
)

func validDeal(t *testing.T) *Deal {
	t.Helper()
	d, err := NewDeal("id-1", "cust-1", "Big ISP Contract", 100000, "EUR", "some notes")
	if err != nil {
		t.Fatalf("unexpected error building valid deal: %v", err)
	}
	return d
}

func TestNewDeal(t *testing.T) {
	t.Run("valid creation", func(t *testing.T) {
		d, err := NewDeal("id-1", "cust-1", "Big Contract", 50000, "EUR", "notes")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if d.ID != "id-1" {
			t.Errorf("ID: want %q, got %q", "id-1", d.ID)
		}
		if d.CustomerID != "cust-1" {
			t.Errorf("CustomerID: want %q, got %q", "cust-1", d.CustomerID)
		}
		if d.Title != "Big Contract" {
			t.Errorf("Title: want %q, got %q", "Big Contract", d.Title)
		}
		if d.Value != 50000 {
			t.Errorf("Value: want 50000, got %d", d.Value)
		}
		if d.Currency != "EUR" {
			t.Errorf("Currency: want %q, got %q", "EUR", d.Currency)
		}
		if d.Stage != StageNew {
			t.Errorf("Stage: want %q, got %q", StageNew, d.Stage)
		}
	})

	t.Run("empty title returns ErrTitleRequired", func(t *testing.T) {
		_, err := NewDeal("id-1", "cust-1", "", 0, "EUR", "")
		if !errors.Is(err, ErrTitleRequired) {
			t.Errorf("expected ErrTitleRequired, got %v", err)
		}
	})

	t.Run("empty currency defaults to EUR", func(t *testing.T) {
		d, err := NewDeal("id-1", "cust-1", "Title", 0, "", "")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if d.Currency != "EUR" {
			t.Errorf("Currency: want %q, got %q", "EUR", d.Currency)
		}
	})
}

func TestQualify(t *testing.T) {
	t.Run("new → qualified succeeds", func(t *testing.T) {
		d := validDeal(t)
		if err := d.Qualify(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if d.Stage != StageQualified {
			t.Errorf("Stage: want %q, got %q", StageQualified, d.Stage)
		}
	})

	t.Run("qualified → qualified returns error", func(t *testing.T) {
		d := validDeal(t)
		_ = d.Qualify()
		err := d.Qualify()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("won deal returns ErrAlreadyClosed", func(t *testing.T) {
		d := validDeal(t)
		_ = d.Win()
		err := d.Qualify()
		if !errors.Is(err, ErrAlreadyClosed) {
			t.Errorf("expected ErrAlreadyClosed, got %v", err)
		}
	})

	t.Run("lost deal returns ErrAlreadyClosed", func(t *testing.T) {
		d := validDeal(t)
		_ = d.Lose()
		err := d.Qualify()
		if !errors.Is(err, ErrAlreadyClosed) {
			t.Errorf("expected ErrAlreadyClosed, got %v", err)
		}
	})
}

func TestPropose(t *testing.T) {
	t.Run("qualified → proposal succeeds", func(t *testing.T) {
		d := validDeal(t)
		_ = d.Qualify()
		if err := d.Propose(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if d.Stage != StageProposal {
			t.Errorf("Stage: want %q, got %q", StageProposal, d.Stage)
		}
	})

	t.Run("new → proposal returns error (wrong stage)", func(t *testing.T) {
		d := validDeal(t)
		err := d.Propose()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("won deal returns ErrAlreadyClosed", func(t *testing.T) {
		d := validDeal(t)
		_ = d.Win()
		err := d.Propose()
		if !errors.Is(err, ErrAlreadyClosed) {
			t.Errorf("expected ErrAlreadyClosed, got %v", err)
		}
	})
}

func TestNegotiate(t *testing.T) {
	t.Run("proposal → negotiation succeeds", func(t *testing.T) {
		d := validDeal(t)
		_ = d.Qualify()
		_ = d.Propose()
		if err := d.Negotiate(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if d.Stage != StageNegotiation {
			t.Errorf("Stage: want %q, got %q", StageNegotiation, d.Stage)
		}
	})

	t.Run("qualified → negotiation returns error (wrong stage)", func(t *testing.T) {
		d := validDeal(t)
		_ = d.Qualify()
		err := d.Negotiate()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("won deal returns ErrAlreadyClosed", func(t *testing.T) {
		d := validDeal(t)
		_ = d.Win()
		err := d.Negotiate()
		if !errors.Is(err, ErrAlreadyClosed) {
			t.Errorf("expected ErrAlreadyClosed, got %v", err)
		}
	})
}

func TestWin(t *testing.T) {
	t.Run("new → won succeeds", func(t *testing.T) {
		d := validDeal(t)
		if err := d.Win(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if d.Stage != StageWon {
			t.Errorf("Stage: want %q, got %q", StageWon, d.Stage)
		}
	})

	t.Run("negotiation → won succeeds", func(t *testing.T) {
		d := validDeal(t)
		_ = d.Qualify()
		_ = d.Propose()
		_ = d.Negotiate()
		if err := d.Win(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if d.Stage != StageWon {
			t.Errorf("Stage: want %q, got %q", StageWon, d.Stage)
		}
	})

	t.Run("already won returns ErrAlreadyClosed", func(t *testing.T) {
		d := validDeal(t)
		_ = d.Win()
		err := d.Win()
		if !errors.Is(err, ErrAlreadyClosed) {
			t.Errorf("expected ErrAlreadyClosed, got %v", err)
		}
	})

	t.Run("already lost returns ErrAlreadyClosed", func(t *testing.T) {
		d := validDeal(t)
		_ = d.Lose()
		err := d.Win()
		if !errors.Is(err, ErrAlreadyClosed) {
			t.Errorf("expected ErrAlreadyClosed, got %v", err)
		}
	})
}

func TestLose(t *testing.T) {
	t.Run("new → lost succeeds", func(t *testing.T) {
		d := validDeal(t)
		if err := d.Lose(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if d.Stage != StageLost {
			t.Errorf("Stage: want %q, got %q", StageLost, d.Stage)
		}
	})

	t.Run("qualified → lost succeeds", func(t *testing.T) {
		d := validDeal(t)
		_ = d.Qualify()
		if err := d.Lose(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if d.Stage != StageLost {
			t.Errorf("Stage: want %q, got %q", StageLost, d.Stage)
		}
	})

	t.Run("already lost returns ErrAlreadyClosed", func(t *testing.T) {
		d := validDeal(t)
		_ = d.Lose()
		err := d.Lose()
		if !errors.Is(err, ErrAlreadyClosed) {
			t.Errorf("expected ErrAlreadyClosed, got %v", err)
		}
	})

	t.Run("already won returns ErrAlreadyClosed", func(t *testing.T) {
		d := validDeal(t)
		_ = d.Win()
		err := d.Lose()
		if !errors.Is(err, ErrAlreadyClosed) {
			t.Errorf("expected ErrAlreadyClosed, got %v", err)
		}
	})
}

func TestUpdate(t *testing.T) {
	t.Run("valid update succeeds", func(t *testing.T) {
		d := validDeal(t)
		if err := d.Update("New Title", 200000, "USD", "updated notes"); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if d.Title != "New Title" {
			t.Errorf("Title: want %q, got %q", "New Title", d.Title)
		}
		if d.Value != 200000 {
			t.Errorf("Value: want 200000, got %d", d.Value)
		}
		if d.Currency != "USD" {
			t.Errorf("Currency: want %q, got %q", "USD", d.Currency)
		}
	})

	t.Run("empty title returns ErrTitleRequired", func(t *testing.T) {
		d := validDeal(t)
		err := d.Update("", 0, "EUR", "")
		if !errors.Is(err, ErrTitleRequired) {
			t.Errorf("expected ErrTitleRequired, got %v", err)
		}
	})

	t.Run("empty currency defaults to EUR", func(t *testing.T) {
		d := validDeal(t)
		if err := d.Update("Title", 0, "", ""); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if d.Currency != "EUR" {
			t.Errorf("Currency: want %q, got %q", "EUR", d.Currency)
		}
	})
}
