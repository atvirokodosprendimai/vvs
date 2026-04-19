package queries

import (
	"context"
	"fmt"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
)

type ListInvoicesForCustomerQuery struct {
	CustomerID string
}

type ListInvoicesForCustomerHandler struct {
	db *gormsqlite.DB
}

func NewListInvoicesForCustomerHandler(db *gormsqlite.DB) *ListInvoicesForCustomerHandler {
	return &ListInvoicesForCustomerHandler{db: db}
}

func (h *ListInvoicesForCustomerHandler) Handle(ctx context.Context, q ListInvoicesForCustomerQuery) ([]InvoiceReadModel, error) {
	var rows []invoiceRow
	err := h.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Preload("LineItems").
			Where("customer_id = ?", q.CustomerID).
			Order("issue_date DESC").
			Find(&rows).Error
	})
	if err != nil {
		return nil, fmt.Errorf("list invoices for customer: %w", err)
	}
	out := make([]InvoiceReadModel, len(rows))
	for i := range rows {
		out[i] = rows[i].toReadModel()
	}
	return out, nil
}
