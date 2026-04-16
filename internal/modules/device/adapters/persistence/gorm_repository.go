package persistence

import (
	"context"

	"gorm.io/gorm"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/modules/device/domain"
)

// GormDeviceRepository implements domain.DeviceRepository using GORM + SQLite.
type GormDeviceRepository struct {
	db *gormsqlite.DB
}

func NewGormDeviceRepository(db *gormsqlite.DB) *GormDeviceRepository {
	return &GormDeviceRepository{db: db}
}

func (r *GormDeviceRepository) Save(ctx context.Context, d *domain.Device) error {
	model := toModel(d)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(&model).Error
	})
}

func (r *GormDeviceRepository) FindByID(ctx context.Context, id string) (*domain.Device, error) {
	var model DeviceModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return model.toDomain(), nil
}

func (r *GormDeviceRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		result := tx.Where("id = ?", id).Delete(&DeviceModel{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}
