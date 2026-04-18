package domain

import (
	"context"
	"time"
)

// ServiceRepository is the port for service persistence.
type ServiceRepository interface {
	Save(ctx context.Context, s *Service) error
	FindByID(ctx context.Context, id string) (*Service, error)
	ListForCustomer(ctx context.Context, customerID string) ([]*Service, error)
	ListForProduct(ctx context.Context, productID string) ([]*Service, error)
	// ListDueForBilling returns active services whose NextBillingDate <= asOf.
	ListDueForBilling(ctx context.Context, asOf time.Time) ([]*Service, error)
}
