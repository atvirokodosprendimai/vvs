package domain

import (
	"context"
	"time"

	shareddomain "github.com/vvs/isp/internal/shared/domain"
)

type RecurringFilter struct {
	Search     string
	Status     string
	CustomerID string
}

type RecurringInvoiceRepository interface {
	Save(ctx context.Context, invoice *RecurringInvoice) error
	FindByID(ctx context.Context, id string) (*RecurringInvoice, error)
	FindByCustomer(ctx context.Context, customerID string) ([]*RecurringInvoice, error)
	FindDueForGeneration(ctx context.Context, asOf time.Time) ([]*RecurringInvoice, error)
	FindAll(ctx context.Context, filter RecurringFilter, page shareddomain.Pagination) ([]*RecurringInvoice, int64, error)
	Delete(ctx context.Context, id string) error
}
