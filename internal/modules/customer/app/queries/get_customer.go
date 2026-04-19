package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/customer/domain"
	"gorm.io/gorm"
)

type GetCustomerQuery struct {
	ID string
}

type GetCustomerHandler struct {
	db *gormsqlite.DB
}

func NewGetCustomerHandler(db *gormsqlite.DB) *GetCustomerHandler {
	return &GetCustomerHandler{db: db}
}

func (h *GetCustomerHandler) Handle(ctx context.Context, q GetCustomerQuery) (*domain.Customer, error) {
	var model CustomerReadModel
	err := h.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Table("customers").Where("id = ?", q.ID).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrCustomerNotFound
		}
		return nil, err
	}
	return model.ToDomain(), nil
}
