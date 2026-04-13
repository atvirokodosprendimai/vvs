package domain

import (
	"context"

	shareddomain "github.com/vvs/isp/internal/shared/domain"
)

type InvoiceFilter struct {
	Search     string
	Status     string
	CustomerID string
}

type InvoiceRepository interface {
	NextInvoiceNumber(ctx context.Context, year int) (string, error)
	Save(ctx context.Context, invoice *Invoice) error
	FindByID(ctx context.Context, id string) (*Invoice, error)
	FindByCustomer(ctx context.Context, customerID string) ([]*Invoice, error)
	FindOutstanding(ctx context.Context) ([]*Invoice, error)
	FindAll(ctx context.Context, filter InvoiceFilter, page shareddomain.Pagination) ([]*Invoice, int64, error)
	Delete(ctx context.Context, id string) error
}
