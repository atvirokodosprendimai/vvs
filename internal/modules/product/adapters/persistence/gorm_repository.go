package persistence

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/product/domain"
	shareddomain "github.com/atvirokodosprendimai/vvs/internal/shared/domain"
	"gorm.io/gorm"
)

type GormProductRepository struct {
	db *gormsqlite.DB
}

func NewGormProductRepository(db *gormsqlite.DB) *GormProductRepository {
	return &GormProductRepository{db: db}
}

func (r *GormProductRepository) Save(ctx context.Context, product *domain.Product) error {
	model := toModel(product)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(model).Error
	})
}

func (r *GormProductRepository) FindByID(ctx context.Context, id string) (*domain.Product, error) {
	var model ProductModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrProductNotFound
		}
		return nil, err
	}
	return toDomain(&model), nil
}

func (r *GormProductRepository) FindAll(ctx context.Context, filter domain.ProductFilter, page shareddomain.Pagination) ([]*domain.Product, int64, error) {
	var products []*domain.Product
	var total int64

	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		query := tx.Model(&ProductModel{})

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

		if err := query.Count(&total).Error; err != nil {
			return err
		}

		var models []ProductModel
		if err := query.Order("created_at DESC").
			Offset(page.Offset()).Limit(page.PageSize).
			Find(&models).Error; err != nil {
			return err
		}

		products = make([]*domain.Product, len(models))
		for i, m := range models {
			products[i] = toDomain(&m)
		}
		return nil
	})

	return products, total, err
}

func (r *GormProductRepository) FindActive(ctx context.Context, page shareddomain.Pagination) ([]*domain.Product, int64, error) {
	var products []*domain.Product
	var total int64

	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		query := tx.Model(&ProductModel{}).Where("is_active = ?", true)

		if err := query.Count(&total).Error; err != nil {
			return err
		}

		var models []ProductModel
		if err := query.Order("created_at DESC").
			Offset(page.Offset()).Limit(page.PageSize).
			Find(&models).Error; err != nil {
			return err
		}

		products = make([]*domain.Product, len(models))
		for i, m := range models {
			products[i] = toDomain(&m)
		}
		return nil
	})

	return products, total, err
}

func (r *GormProductRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Delete(&ProductModel{}, "id = ?", id).Error
	})
}
