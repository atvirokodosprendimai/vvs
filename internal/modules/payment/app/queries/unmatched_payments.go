package queries

import (
	"context"

	"github.com/vvs/isp/internal/modules/payment/domain"
	"gorm.io/gorm"
)

type UnmatchedPaymentsQuery struct{}

type UnmatchedPaymentsHandler struct {
	db *gorm.DB
}

func NewUnmatchedPaymentsHandler(db *gorm.DB) *UnmatchedPaymentsHandler {
	return &UnmatchedPaymentsHandler{db: db}
}

func (h *UnmatchedPaymentsHandler) Handle(_ context.Context, _ UnmatchedPaymentsQuery) ([]*domain.Payment, error) {
	var models []PaymentReadModel
	if err := h.db.Table("payments").
		Where("status IN (?, ?)", string(domain.StatusImported), string(domain.StatusUnmatched)).
		Order("booking_date DESC").
		Find(&models).Error; err != nil {
		return nil, err
	}

	payments := make([]*domain.Payment, len(models))
	for i, m := range models {
		payments[i] = m.ToDomain()
	}

	return payments, nil
}
