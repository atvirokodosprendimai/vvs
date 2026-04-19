package persistence

import (
	"context"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/service/domain"
	"gorm.io/gorm"
)

// GormServiceRepository implements domain.ServiceRepository using GORM + SQLite.
type GormServiceRepository struct {
	db *gormsqlite.DB
}

func NewGormServiceRepository(db *gormsqlite.DB) *GormServiceRepository {
	return &GormServiceRepository{db: db}
}

func (r *GormServiceRepository) Save(ctx context.Context, s *domain.Service) error {
	model := toModel(s)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(&model).Error
	})
}

func (r *GormServiceRepository) FindByID(ctx context.Context, id string) (*domain.Service, error) {
	var model ServiceModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return model.toDomain(), nil
}

func (r *GormServiceRepository) ListForCustomer(ctx context.Context, customerID string) ([]*domain.Service, error) {
	var models []ServiceModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("customer_id = ?", customerID).Order("created_at DESC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	return modelsToServices(models), nil
}

func (r *GormServiceRepository) ListForProduct(ctx context.Context, productID string) ([]*domain.Service, error) {
	var models []ServiceModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("product_id = ?", productID).Order("created_at DESC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	return modelsToServices(models), nil
}

func (r *GormServiceRepository) ListDueForBilling(ctx context.Context, asOf time.Time) ([]*domain.Service, error) {
	var models []ServiceModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("status = ? AND next_billing_date IS NOT NULL AND next_billing_date <= ?", domain.StatusActive, asOf).
			Order("next_billing_date ASC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	return modelsToServices(models), nil
}

func modelsToServices(models []ServiceModel) []*domain.Service {
	result := make([]*domain.Service, len(models))
	for i := range models {
		result[i] = models[i].toDomain()
	}
	return result
}
