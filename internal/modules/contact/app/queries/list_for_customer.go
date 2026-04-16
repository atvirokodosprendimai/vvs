package queries

import (
	"context"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
)

type ListContactsForCustomerQuery struct {
	CustomerID string
}

type ListContactsForCustomerHandler struct {
	db *gormsqlite.DB
}

func NewListContactsForCustomerHandler(db *gormsqlite.DB) *ListContactsForCustomerHandler {
	return &ListContactsForCustomerHandler{db: db}
}

func (h *ListContactsForCustomerHandler) Handle(ctx context.Context, q ListContactsForCustomerQuery) ([]ContactReadModel, error) {
	var contacts []ContactReadModel
	err := h.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Raw(
			`SELECT id, customer_id, first_name, last_name, email, phone, role, notes, created_at, updated_at
			 FROM contacts WHERE customer_id = ? ORDER BY created_at ASC`,
			q.CustomerID,
		).Scan(&contacts).Error
	})
	return contacts, err
}
