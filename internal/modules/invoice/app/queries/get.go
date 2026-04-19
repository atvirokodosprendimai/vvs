package queries

import (
	"context"
	"fmt"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"gorm.io/gorm"
)

type GetInvoiceQuery struct {
	ID string
}

type GetInvoiceHandler struct {
	db *gormsqlite.DB
}

func NewGetInvoiceHandler(db *gormsqlite.DB) *GetInvoiceHandler {
	return &GetInvoiceHandler{db: db}
}

func (h *GetInvoiceHandler) Handle(ctx context.Context, id string) (*InvoiceReadModel, error) {
	var row invoiceRow
	err := h.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Preload("LineItems").Where("id = ?", id).First(&row).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invoice not found: %s", id)
		}
		return nil, fmt.Errorf("get invoice: %w", err)
	}
	rm := row.toReadModel()
	return &rm, nil
}
