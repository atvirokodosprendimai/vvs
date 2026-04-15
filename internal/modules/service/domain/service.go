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
	ID          string
	CustomerID  string
	ProductID   string
	ProductName string // snapshot at assign time
	PriceAmount int64  // cents
	Currency    string
	StartDate   time.Time
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewService(id, customerID, productID, productName string, priceAmount int64, currency string, startDate time.Time) (*Service, error) {
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
	now := time.Now().UTC()
	return &Service{
		ID:          id,
		CustomerID:  customerID,
		ProductID:   productID,
		ProductName: productName,
		PriceAmount: priceAmount,
		Currency:    currency,
		StartDate:   startDate,
		Status:      StatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
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
