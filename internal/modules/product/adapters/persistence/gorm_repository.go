package persistence

import (
	"context"

	"github.com/vvs/isp/internal/infrastructure/database"
	"github.com/vvs/isp/internal/modules/product/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
	"gorm.io/gorm"
)

type GormProductRepository struct {
	writer *database.WriteSerializer
	reader *gorm.DB
}

func NewGormProductRepository(writer *database.WriteSerializer, reader *gorm.DB) *GormProductRepository {
	return &GormProductRepository{writer: writer, reader: reader}
}

func (r *GormProductRepository) Save(ctx context.Context, product *domain.Product) error {
	model := toModel(product)
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		return tx.Save(model).Error
	})
}

func (r *GormProductRepository) FindByID(_ context.Context, id string) (*domain.Product, error) {
	var model ProductModel
	if err := r.reader.Where("id = ?", id).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrProductNotFound
		}
		return nil, err
	}
	return toDomain(&model), nil
}

func (r *GormProductRepository) FindAll(_ context.Context, filter domain.ProductFilter, page shareddomain.Pagination) ([]*domain.Product, int64, error) {
	query := r.reader.Model(&ProductModel{})

	if filter.Search != "" {
		search := "%" + filter.Search + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", search, search)
	}
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.Active != nil {
		query = query.Where("is_active = ?", *filter.Active)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var models []ProductModel
	if err := query.Order("created_at DESC").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&models).Error; err != nil {
		return nil, 0, err
	}

	products := make([]*domain.Product, len(models))
	for i, m := range models {
		products[i] = toDomain(&m)
	}

	return products, total, nil
}

func (r *GormProductRepository) FindActive(_ context.Context, page shareddomain.Pagination) ([]*domain.Product, int64, error) {
	query := r.reader.Model(&ProductModel{}).Where("is_active = ?", true)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var models []ProductModel
	if err := query.Order("created_at DESC").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&models).Error; err != nil {
		return nil, 0, err
	}

	products := make([]*domain.Product, len(models))
	for i, m := range models {
		products[i] = toDomain(&m)
	}

	return products, total, nil
}

func (r *GormProductRepository) Delete(ctx context.Context, id string) error {
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		return tx.Delete(&ProductModel{}, "id = ?", id).Error
	})
}
