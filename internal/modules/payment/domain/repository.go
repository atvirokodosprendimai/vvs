package domain

import (
	"context"

	shareddomain "github.com/vvs/isp/internal/shared/domain"
)

type PaymentFilter struct {
	Search     string
	Status     string
	CustomerID string
	InvoiceID  string
}

type PaymentRepository interface {
	Save(ctx context.Context, payment *Payment) error
	SaveBatch(ctx context.Context, payments []*Payment) error
	FindByID(ctx context.Context, id string) (*Payment, error)
	FindUnmatched(ctx context.Context) ([]*Payment, error)
	FindByInvoice(ctx context.Context, invoiceID string) ([]*Payment, error)
	FindAll(ctx context.Context, filter PaymentFilter, page shareddomain.Pagination) ([]*Payment, int64, error)
}
