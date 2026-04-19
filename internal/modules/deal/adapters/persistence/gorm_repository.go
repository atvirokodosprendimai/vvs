package persistence

import (
	"context"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/deal/domain"
	"gorm.io/gorm"
)

// DealModel is the GORM model mapping to the deals table.
type DealModel struct {
	ID         string    `gorm:"primaryKey;type:text"`
	CustomerID string    `gorm:"column:customer_id;type:text;not null"`
	Title      string    `gorm:"type:text;not null"`
	Value      int64     `gorm:"not null;default:0"`
	Currency   string    `gorm:"type:text;not null;default:'EUR'"`
	Stage      string    `gorm:"type:text;not null;default:'new'"`
	Notes      string    `gorm:"type:text;not null;default:''"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (DealModel) TableName() string { return "deals" }

// GormDealRepository implements domain.DealRepository using GORM + SQLite.
type GormDealRepository struct {
	db *gormsqlite.DB
}

func NewGormDealRepository(db *gormsqlite.DB) *GormDealRepository {
	return &GormDealRepository{db: db}
}

func (r *GormDealRepository) Save(ctx context.Context, d *domain.Deal) error {
	model := toModel(d)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(&model).Error
	})
}

func (r *GormDealRepository) FindByID(ctx context.Context, id string) (*domain.Deal, error) {
	var model DealModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return toDomain(&model), nil
}

func (r *GormDealRepository) ListForCustomer(ctx context.Context, customerID string) ([]*domain.Deal, error) {
	var models []DealModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("customer_id = ?", customerID).Order("created_at DESC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Deal, len(models))
	for i := range models {
		result[i] = toDomain(&models[i])
	}
	return result, nil
}

func (r *GormDealRepository) ListAll(ctx context.Context) ([]*domain.Deal, error) {
	var models []DealModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Order("created_at DESC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Deal, len(models))
	for i := range models {
		result[i] = toDomain(&models[i])
	}
	return result, nil
}

func (r *GormDealRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&DealModel{}).Error
	})
}

func toModel(d *domain.Deal) DealModel {
	return DealModel{
		ID:         d.ID,
		CustomerID: d.CustomerID,
		Title:      d.Title,
		Value:      d.Value,
		Currency:   d.Currency,
		Stage:      d.Stage,
		Notes:      d.Notes,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}

func toDomain(m *DealModel) *domain.Deal {
	return &domain.Deal{
		ID:         m.ID,
		CustomerID: m.CustomerID,
		Title:      m.Title,
		Value:      m.Value,
		Currency:   m.Currency,
		Stage:      m.Stage,
		Notes:      m.Notes,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}
