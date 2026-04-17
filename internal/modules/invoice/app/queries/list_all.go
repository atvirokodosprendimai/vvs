package queries

import (
	"context"
	"fmt"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
)

type ListAllInvoicesQuery struct {
	Status string // optional filter
}

type ListAllInvoicesHandler struct {
	db *gormsqlite.DB
}

func NewListAllInvoicesHandler(db *gormsqlite.DB) *ListAllInvoicesHandler {
	return &ListAllInvoicesHandler{db: db}
}

func (h *ListAllInvoicesHandler) Handle(ctx context.Context, q ListAllInvoicesQuery) ([]InvoiceReadModel, error) {
	var rows []invoiceRow
	err := h.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		query := tx.Preload("LineItems")
		if q.Status != "" {
			query = query.Where("status = ?", q.Status)
		}
		return query.Order("created_at DESC").Find(&rows).Error
	})
	if err != nil {
		return nil, fmt.Errorf("list all invoices: %w", err)
	}
	out := make([]InvoiceReadModel, len(rows))
	for i := range rows {
		out[i] = rows[i].toReadModel()
	}
	return out, nil
}
