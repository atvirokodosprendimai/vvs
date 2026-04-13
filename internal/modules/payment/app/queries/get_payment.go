package queries

import (
	"context"

	"github.com/vvs/isp/internal/modules/payment/domain"
	"gorm.io/gorm"
)

type GetPaymentQuery struct {
	ID string
}

type GetPaymentHandler struct {
	db *gorm.DB
}

func NewGetPaymentHandler(db *gorm.DB) *GetPaymentHandler {
	return &GetPaymentHandler{db: db}
}

func (h *GetPaymentHandler) Handle(_ context.Context, q GetPaymentQuery) (*domain.Payment, error) {
	var model PaymentReadModel
	if err := h.db.Table("payments").Where("id = ?", q.ID).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, err
	}
	return model.ToDomain(), nil
}
