package queries

import (
	"context"

	"github.com/vvs/isp/internal/modules/product/domain"
	"gorm.io/gorm"
)

type GetProductQuery struct {
	ID string
}

type GetProductHandler struct {
	db *gorm.DB
}

func NewGetProductHandler(db *gorm.DB) *GetProductHandler {
	return &GetProductHandler{db: db}
}

func (h *GetProductHandler) Handle(_ context.Context, q GetProductQuery) (*domain.Product, error) {
	var model ProductReadModel
	if err := h.db.Table("products").Where("id = ?", q.ID).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrProductNotFound
		}
		return nil, err
	}
	return model.ToDomain(), nil
}
