package queries

import (
	"context"

	"gorm.io/gorm"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/device/domain"
)

type GetDeviceQuery struct {
	ID string
}

type GetDeviceHandler struct {
	db *gormsqlite.DB
}

func NewGetDeviceHandler(db *gormsqlite.DB) *GetDeviceHandler {
	return &GetDeviceHandler{db: db}
}

func (h *GetDeviceHandler) Handle(ctx context.Context, q GetDeviceQuery) (*DeviceReadModel, error) {
	var model DeviceReadModel
	err := h.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", q.ID).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &model, nil
}
