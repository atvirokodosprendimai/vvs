package queries

import (
	"context"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/modules/product/domain"
	"gorm.io/gorm"
)

type GetProductQuery struct {
	ID string
}

type GetProductHandler struct {
	db *gormsqlite.DB
}

func NewGetProductHandler(db *gormsqlite.DB) *GetProductHandler {
	return &GetProductHandler{db: db}
}

func (h *GetProductHandler) Handle(ctx context.Context, q GetProductQuery) (*domain.Product, error) {
	var model ProductReadModel
	err := h.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Table("products").Where("id = ?", q.ID).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrProductNotFound
		}
		return nil, err
	}
	return model.ToDomain(), nil
}
