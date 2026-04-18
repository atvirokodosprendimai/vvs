package domain

import (
	"errors"
	"time"
)

// Status values for a Service.
const (
	StatusActive    = "active"
	StatusSuspended = "suspended"
	StatusCancelled = "cancelled"
)

var ErrNotFound = errors.New("service not found")
var ErrInvalidTransition = errors.New("invalid status transition")

// Service represents a product/service assigned to a customer.
type Service struct {
	ID              string
	CustomerID      string
	ProductID       string
	ProductName     string // snapshot at assign time
	PriceAmount     int64  // cents
	Currency        string
	StartDate       time.Time
	Status          string
	BillingCycle    string     // snapshot from product: "monthly", "quarterly", "yearly"
	NextBillingDate *time.Time // nil for legacy rows pre-migration
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func NewService(id, customerID, productID, productName string, priceAmount int64, currency string, startDate time.Time, billingCycle string) (*Service, error) {
	if customerID == "" {
		return nil, errors.New("customer id required")
	}
	if productID == "" {
		return nil, errors.New("product id required")
	}
	if productName == "" {
		return nil, errors.New("product name required")
	}
	if currency == "" {
		currency = "EUR"
	}
	next := advanceDate(startDate, billingCycle)
	now := time.Now().UTC()
	return &Service{
		ID:              id,
		CustomerID:      customerID,
		ProductID:       productID,
		ProductName:     productName,
		PriceAmount:     priceAmount,
		Currency:        currency,
		StartDate:       startDate,
		Status:          StatusActive,
		BillingCycle:    billingCycle,
		NextBillingDate: &next,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

func (s *Service) AdvanceNextBillingDate() {
	if s.NextBillingDate == nil {
		return
	}
	next := advanceDate(*s.NextBillingDate, s.BillingCycle)
	s.NextBillingDate = &next
	s.UpdatedAt = time.Now().UTC()
}

func advanceDate(from time.Time, cycle string) time.Time {
	switch cycle {
	case "quarterly":
		return from.AddDate(0, 3, 0)
	case "yearly":
		return from.AddDate(1, 0, 0)
	default: // monthly
		return from.AddDate(0, 1, 0)
	}
}

func (s *Service) Suspend() error {
	if s.Status != StatusActive {
		return ErrInvalidTransition
	}
	s.Status = StatusSuspended
	s.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *Service) Reactivate() error {
	if s.Status != StatusSuspended {
		return ErrInvalidTransition
	}
	s.Status = StatusActive
	s.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *Service) Cancel() error {
	if s.Status == StatusCancelled {
		return ErrInvalidTransition
	}
	s.Status = StatusCancelled
	s.UpdatedAt = time.Now().UTC()
	return nil
}
