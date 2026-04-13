package queries

import (
	"context"

	"github.com/vvs/isp/internal/modules/recurring/domain"
	"gorm.io/gorm"
)

type GetRecurringQuery struct {
	ID string
}

type GetRecurringHandler struct {
	db *gorm.DB
}

func NewGetRecurringHandler(db *gorm.DB) *GetRecurringHandler {
	return &GetRecurringHandler{db: db}
}

func (h *GetRecurringHandler) Handle(_ context.Context, q GetRecurringQuery) (*domain.RecurringInvoice, error) {
	var model RecurringReadModel
	if err := h.db.Table("recurring_invoices").Where("id = ?", q.ID).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrRecurringNotFound
		}
		return nil, err
	}

	inv := model.ToDomain()

	var lineModels []RecurringLineReadModel
	h.db.Table("recurring_lines").Where("recurring_id = ?", q.ID).Order("sort_order ASC").Find(&lineModels)
	for _, lm := range lineModels {
		inv.Lines = append(inv.Lines, lm.ToDomain())
	}

	return inv, nil
}
