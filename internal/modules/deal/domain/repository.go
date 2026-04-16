package domain

import "context"

// DealRepository is the port for deal persistence.
type DealRepository interface {
	Save(ctx context.Context, deal *Deal) error
	FindByID(ctx context.Context, id string) (*Deal, error)
	ListForCustomer(ctx context.Context, customerID string) ([]*Deal, error)
	Delete(ctx context.Context, id string) error
}
