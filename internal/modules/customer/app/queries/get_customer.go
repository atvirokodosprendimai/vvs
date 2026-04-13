package queries

import (
	"context"

	"github.com/vvs/isp/internal/modules/customer/domain"
	"gorm.io/gorm"
)

type GetCustomerQuery struct {
	ID string
}

type GetCustomerHandler struct {
	db *gorm.DB
}

func NewGetCustomerHandler(db *gorm.DB) *GetCustomerHandler {
	return &GetCustomerHandler{db: db}
}

func (h *GetCustomerHandler) Handle(_ context.Context, q GetCustomerQuery) (*domain.Customer, error) {
	var model CustomerReadModel
	if err := h.db.Table("customers").Where("id = ?", q.ID).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrCustomerNotFound
		}
		return nil, err
	}
	return model.ToDomain(), nil
}
