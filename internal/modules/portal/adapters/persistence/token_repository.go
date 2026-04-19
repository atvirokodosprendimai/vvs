package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/modules/portal/domain"
	"gorm.io/gorm"
)

type portalTokenModel struct {
	ID         string     `gorm:"primaryKey;type:text"`
	CustomerID string     `gorm:"column:customer_id;type:text;not null;index"`
	TokenHash  string     `gorm:"column:token_hash;type:text;uniqueIndex;not null"`
	ExpiresAt  time.Time  `gorm:"column:expires_at"`
	CreatedAt  time.Time  `gorm:"column:created_at"`
	UsedAt     *time.Time `gorm:"column:used_at"`
}

func (portalTokenModel) TableName() string { return "portal_tokens" }

// GormPortalTokenRepository persists PortalToken records.
type GormPortalTokenRepository struct {
	db *gormsqlite.DB
}

func NewGormPortalTokenRepository(db *gormsqlite.DB) *GormPortalTokenRepository {
	return &GormPortalTokenRepository{db: db}
}

func (r *GormPortalTokenRepository) Save(ctx context.Context, t *domain.PortalToken) error {
	m := portalTokenModel{
		ID:         t.ID,
		CustomerID: t.CustomerID,
		TokenHash:  t.TokenHash,
		ExpiresAt:  t.ExpiresAt,
		CreatedAt:  t.CreatedAt,
	}
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(&m).Error
	})
}

func (r *GormPortalTokenRepository) FindByHash(ctx context.Context, hash string) (*domain.PortalToken, error) {
	var m portalTokenModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("token_hash = ?", hash).First(&m).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &domain.PortalToken{
		ID:         m.ID,
		CustomerID: m.CustomerID,
		TokenHash:  m.TokenHash,
		ExpiresAt:  m.ExpiresAt,
		CreatedAt:  m.CreatedAt,
		UsedAt:     m.UsedAt,
	}, nil
}

func (r *GormPortalTokenRepository) MarkUsed(ctx context.Context, tokenHash string) error {
	now := time.Now().UTC()
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Model(&portalTokenModel{}).
			Where("token_hash = ? AND used_at IS NULL", tokenHash).
			Update("used_at", now).Error
	})
}

func (r *GormPortalTokenRepository) DeleteByCustomerID(ctx context.Context, customerID string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("customer_id = ?", customerID).Delete(&portalTokenModel{}).Error
	})
}

func (r *GormPortalTokenRepository) PruneExpired(ctx context.Context) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("expires_at < datetime('now')").Delete(&portalTokenModel{}).Error
	})
}
