package persistence

import (
	"context"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/modules/network/domain"
	"gorm.io/gorm"
)

type GormRouterRepository struct {
	db *gormsqlite.DB
}

func NewGormRouterRepository(db *gormsqlite.DB) *GormRouterRepository {
	return &GormRouterRepository{db: db}
}

func (r *GormRouterRepository) Save(ctx context.Context, router *domain.Router) error {
	model := toModel(router)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(model).Error
	})
}

func (r *GormRouterRepository) FindByID(ctx context.Context, id string) (*domain.Router, error) {
	var model RouterModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrRouterNotFound
		}
		return nil, err
	}
	return toDomain(&model), nil
}

func (r *GormRouterRepository) FindAll(ctx context.Context) ([]*domain.Router, error) {
	var routers []*domain.Router
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []RouterModel
		if err := tx.Order("name ASC").Find(&models).Error; err != nil {
			return err
		}
		routers = make([]*domain.Router, len(models))
		for i, m := range models {
			routers[i] = toDomain(&m)
		}
		return nil
	})
	return routers, err
}

func (r *GormRouterRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Delete(&RouterModel{}, "id = ?", id).Error
	})
}
