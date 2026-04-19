package persistence

import (
	"context"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/invoice/domain"
	"gorm.io/gorm"
)

type invoiceTokenModel struct {
	ID        string    `gorm:"primaryKey;type:text"`
	InvoiceID string    `gorm:"column:invoice_id;type:text;not null"`
	TokenHash string    `gorm:"column:token_hash;type:text;uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (invoiceTokenModel) TableName() string { return "invoice_tokens" }

// InvoiceTokenRepository persists InvoiceToken records.
type InvoiceTokenRepository struct {
	db *gormsqlite.DB
}

// NewInvoiceTokenRepository constructs the repo.
func NewInvoiceTokenRepository(db *gormsqlite.DB) *InvoiceTokenRepository {
	return &InvoiceTokenRepository{db: db}
}

func (r *InvoiceTokenRepository) Save(ctx context.Context, t *domain.InvoiceToken) error {
	m := invoiceTokenModel{
		ID:        t.ID,
		InvoiceID: t.InvoiceID,
		TokenHash: t.TokenHash,
		ExpiresAt: t.ExpiresAt,
		CreatedAt: t.CreatedAt,
	}
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(&m).Error
	})
}

func (r *InvoiceTokenRepository) FindByHash(ctx context.Context, hash string) (*domain.InvoiceToken, error) {
	var m invoiceTokenModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("token_hash = ?", hash).First(&m).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrInvoiceNotFound
		}
		return nil, err
	}
	return &domain.InvoiceToken{
		ID:        m.ID,
		InvoiceID: m.InvoiceID,
		TokenHash: m.TokenHash,
		ExpiresAt: m.ExpiresAt,
		CreatedAt: m.CreatedAt,
	}, nil
}
