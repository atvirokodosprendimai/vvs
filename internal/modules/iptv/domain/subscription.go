package domain

import (
	"context"
	"errors"
	"time"
)

const (
	SubscriptionActive    = "active"
	SubscriptionSuspended = "suspended"
	SubscriptionCancelled = "cancelled"
)

var ErrInvalidSubscriptionTransition = errors.New("iptv: invalid subscription status transition")

// Subscription links a customer to an IPTV package.
type Subscription struct {
	ID         string
	CustomerID string
	PackageID  string
	Status     string
	StartsAt   time.Time
	EndsAt     *time.Time // nil = open-ended
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewSubscription(id, customerID, packageID string, startsAt time.Time) (*Subscription, error) {
	if customerID == "" {
		return nil, errors.New("iptv: customer id required")
	}
	if packageID == "" {
		return nil, errors.New("iptv: package id required")
	}
	now := time.Now().UTC()
	return &Subscription{
		ID:         id,
		CustomerID: customerID,
		PackageID:  packageID,
		Status:     SubscriptionActive,
		StartsAt:   startsAt,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

func (s *Subscription) Suspend() error {
	if s.Status != SubscriptionActive {
		return ErrInvalidSubscriptionTransition
	}
	s.Status = SubscriptionSuspended
	s.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *Subscription) Reactivate() error {
	if s.Status != SubscriptionSuspended {
		return ErrInvalidSubscriptionTransition
	}
	s.Status = SubscriptionActive
	s.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *Subscription) Cancel() error {
	if s.Status == SubscriptionCancelled {
		return ErrInvalidSubscriptionTransition
	}
	now := time.Now().UTC()
	s.Status = SubscriptionCancelled
	s.EndsAt = &now
	s.UpdatedAt = now
	return nil
}

// SubscriptionRepository is the port for subscription persistence.
type SubscriptionRepository interface {
	Save(ctx context.Context, s *Subscription) error
	FindByID(ctx context.Context, id string) (*Subscription, error)
	ListForCustomer(ctx context.Context, customerID string) ([]*Subscription, error)
	ListActive(ctx context.Context) ([]*Subscription, error)
	Delete(ctx context.Context, id string) error
}
