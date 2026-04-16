package domain

import "context"

type ContactRepository interface {
	Save(ctx context.Context, c *Contact) error
	FindByID(ctx context.Context, id string) (*Contact, error)
	ListForCustomer(ctx context.Context, customerID string) ([]*Contact, error)
	Delete(ctx context.Context, id string) error
}
