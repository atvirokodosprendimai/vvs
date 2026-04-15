package persistence

import (
	"context"

	"github.com/vvs/isp/internal/infrastructure/database"
	"github.com/vvs/isp/internal/modules/network/domain"
	"gorm.io/gorm"
)

type GormRouterRepository struct {
	writer *database.WriteSerializer
	reader *gorm.DB
}

func NewGormRouterRepository(writer *database.WriteSerializer, reader *gorm.DB) *GormRouterRepository {
	return &GormRouterRepository{writer: writer, reader: reader}
}

func (r *GormRouterRepository) Save(ctx context.Context, router *domain.Router) error {
	model := toModel(router)
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		return tx.Save(model).Error
	})
}

func (r *GormRouterRepository) FindByID(_ context.Context, id string) (*domain.Router, error) {
	var model RouterModel
	if err := r.reader.Where("id = ?", id).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrRouterNotFound
		}
		return nil, err
	}
	return toDomain(&model), nil
}

func (r *GormRouterRepository) FindAll(_ context.Context) ([]*domain.Router, error) {
	var models []RouterModel
	if err := r.reader.Order("name ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	routers := make([]*domain.Router, len(models))
	for i, m := range models {
		routers[i] = toDomain(&m)
	}
	return routers, nil
}

func (r *GormRouterRepository) Delete(ctx context.Context, id string) error {
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		return tx.Delete(&RouterModel{}, "id = ?", id).Error
	})
}
