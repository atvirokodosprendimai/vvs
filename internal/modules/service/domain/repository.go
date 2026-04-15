package domain

import "context"

// ServiceRepository is the port for service persistence.
type ServiceRepository interface {
	Save(ctx context.Context, s *Service) error
	FindByID(ctx context.Context, id string) (*Service, error)
	ListForCustomer(ctx context.Context, customerID string) ([]*Service, error)
	ListForProduct(ctx context.Context, productID string) ([]*Service, error)
}
