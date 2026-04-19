package domain

import (
	"testing"
	"time"
)

func newTestSubscription(t *testing.T) *Subscription {
	t.Helper()
	s, err := NewSubscription("sub-1", "cust-1", "pkg-1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return s
}

func TestNewSubscription_Active(t *testing.T) {
	s := newTestSubscription(t)
	if s.Status != SubscriptionActive {
		t.Errorf("status: want %q, got %q", SubscriptionActive, s.Status)
	}
}

func TestNewSubscription_MissingCustomer(t *testing.T) {
	_, err := NewSubscription("sub-1", "", "pkg-1", time.Now())
	if err == nil {
		t.Error("expected error for missing customer id")
	}
}

func TestNewSubscription_MissingPackage(t *testing.T) {
	_, err := NewSubscription("sub-1", "cust-1", "", time.Now())
	if err == nil {
		t.Error("expected error for missing package id")
	}
}

func TestSubscription_SuspendAndReactivate(t *testing.T) {
	s := newTestSubscription(t)

	if err := s.Suspend(); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if s.Status != SubscriptionSuspended {
		t.Errorf("status after suspend: want %q, got %q", SubscriptionSuspended, s.Status)
	}

	if err := s.Reactivate(); err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	if s.Status != SubscriptionActive {
		t.Errorf("status after reactivate: want %q, got %q", SubscriptionActive, s.Status)
	}
}

func TestSubscription_SuspendAlreadySuspended(t *testing.T) {
	s := newTestSubscription(t)
	_ = s.Suspend()
	if err := s.Suspend(); err != ErrInvalidSubscriptionTransition {
		t.Errorf("want ErrInvalidSubscriptionTransition, got %v", err)
	}
}

func TestSubscription_ReactivateNotSuspended(t *testing.T) {
	s := newTestSubscription(t)
	if err := s.Reactivate(); err != ErrInvalidSubscriptionTransition {
		t.Errorf("want ErrInvalidSubscriptionTransition, got %v", err)
	}
}

func TestSubscription_Cancel(t *testing.T) {
	s := newTestSubscription(t)
	before := time.Now()
	if err := s.Cancel(); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if s.Status != SubscriptionCancelled {
		t.Errorf("status: want %q, got %q", SubscriptionCancelled, s.Status)
	}
	if s.EndsAt == nil {
		t.Fatal("EndsAt should be set on cancel")
	}
	if s.EndsAt.Before(before) {
		t.Error("EndsAt should be >= time before cancel")
	}
}

func TestSubscription_CancelAlreadyCancelled(t *testing.T) {
	s := newTestSubscription(t)
	_ = s.Cancel()
	if err := s.Cancel(); err != ErrInvalidSubscriptionTransition {
		t.Errorf("want ErrInvalidSubscriptionTransition, got %v", err)
	}
}

func TestSubscription_CancelFromSuspended(t *testing.T) {
	s := newTestSubscription(t)
	_ = s.Suspend()
	if err := s.Cancel(); err != nil {
		t.Fatalf("cancel from suspended: %v", err)
	}
	if s.Status != SubscriptionCancelled {
		t.Errorf("status: want %q, got %q", SubscriptionCancelled, s.Status)
	}
}
