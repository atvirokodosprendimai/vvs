package domain

import "context"

// InvoiceRepository is the port for invoice persistence.
type InvoiceRepository interface {
	Save(ctx context.Context, invoice *Invoice) error
	FindByID(ctx context.Context, id string) (*Invoice, error)
	ListByCustomer(ctx context.Context, customerID string) ([]*Invoice, error)
	ListAll(ctx context.Context) ([]*Invoice, error)
	NextCode(ctx context.Context) (string, error) // INV-001, INV-002...
	FindByCode(ctx context.Context, code string) (*Invoice, error)
	ListOverdue(ctx context.Context) ([]*Invoice, error) // finalized + past due date
}
