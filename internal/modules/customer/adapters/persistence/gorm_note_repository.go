package persistence

import (
	"context"
	"time"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/modules/customer/domain"
)

type noteModel struct {
	ID         string    `gorm:"primaryKey;type:text"`
	CustomerID string    `gorm:"type:text;not null;index"`
	Body       string    `gorm:"type:text;not null"`
	AuthorID   string    `gorm:"type:text;not null;default:''"`
	CreatedAt  time.Time
}

func (noteModel) TableName() string { return "customer_notes" }

type GormNoteRepository struct {
	db *gormsqlite.DB
}

func NewGormNoteRepository(db *gormsqlite.DB) *GormNoteRepository {
	return &GormNoteRepository{db: db}
}

func (r *GormNoteRepository) SaveNote(ctx context.Context, note *domain.CustomerNote) error {
	m := noteModel{
		ID:         note.ID,
		CustomerID: note.CustomerID,
		Body:       note.Body,
		AuthorID:   note.AuthorID,
		CreatedAt:  note.CreatedAt,
	}
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Create(&m).Error
	})
}

func (r *GormNoteRepository) ListNotes(ctx context.Context, customerID string) ([]*domain.CustomerNote, error) {
	var models []noteModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("customer_id = ?", customerID).Order("created_at DESC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	notes := make([]*domain.CustomerNote, len(models))
	for i, m := range models {
		notes[i] = &domain.CustomerNote{
			ID:         m.ID,
			CustomerID: m.CustomerID,
			Body:       m.Body,
			AuthorID:   m.AuthorID,
			CreatedAt:  m.CreatedAt,
		}
	}
	return notes, nil
}
