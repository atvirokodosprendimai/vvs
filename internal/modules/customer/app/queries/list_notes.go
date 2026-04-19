package queries

import (
	"context"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
)

type NoteReadModel struct {
	ID         string    `gorm:"column:id"`
	CustomerID string    `gorm:"column:customer_id"`
	Body       string    `gorm:"column:body"`
	AuthorID   string    `gorm:"column:author_id"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

type ListNotesHandler struct {
	db *gormsqlite.DB
}

func NewListNotesHandler(db *gormsqlite.DB) *ListNotesHandler {
	return &ListNotesHandler{db: db}
}

func (h *ListNotesHandler) Handle(ctx context.Context, customerID string) ([]NoteReadModel, error) {
	var notes []NoteReadModel
	err := h.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Raw(
			"SELECT id, customer_id, body, author_id, created_at FROM customer_notes WHERE customer_id = ? ORDER BY created_at DESC",
			customerID,
		).Scan(&notes).Error
	})
	return notes, err
}
