package persistence

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
	"gorm.io/gorm"
)

// GormVMPlanRepository persists VM plan records.
type GormVMPlanRepository struct {
	db *gormsqlite.DB
}

func NewGormVMPlanRepository(db *gormsqlite.DB) *GormVMPlanRepository {
	return &GormVMPlanRepository{db: db}
}

func (r *GormVMPlanRepository) Save(ctx context.Context, plan *domain.VMPlan) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(toVMPlanModel(plan)).Error
	})
}

func (r *GormVMPlanRepository) FindByID(ctx context.Context, id string) (*domain.VMPlan, error) {
	var model VMPlanModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrVMPlanNotFound
		}
		return nil, err
	}
	return toVMPlanDomain(&model), nil
}

func (r *GormVMPlanRepository) FindAll(ctx context.Context) ([]*domain.VMPlan, error) {
	return r.findWhere(ctx, "")
}

func (r *GormVMPlanRepository) FindEnabled(ctx context.Context) ([]*domain.VMPlan, error) {
	return r.findWhere(ctx, "enabled = ?", true)
}

func (r *GormVMPlanRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&VMPlanModel{}).Error
	})
}

func (r *GormVMPlanRepository) findWhere(ctx context.Context, condition string, args ...any) ([]*domain.VMPlan, error) {
	var plans []*domain.VMPlan
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []VMPlanModel
		q := tx.Order("name ASC")
		if condition != "" {
			q = q.Where(condition, args...)
		}
		if err := q.Find(&models).Error; err != nil {
			return err
		}
		plans = make([]*domain.VMPlan, len(models))
		for i := range models {
			plans[i] = toVMPlanDomain(&models[i])
		}
		return nil
	})
	return plans, err
}
