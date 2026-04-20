package persistence

import (
	"context"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
	"gorm.io/gorm"
)

// GormVMRepository persists virtual machine records.
type GormVMRepository struct {
	db *gormsqlite.DB
}

func NewGormVMRepository(db *gormsqlite.DB) *GormVMRepository {
	return &GormVMRepository{db: db}
}

func (r *GormVMRepository) Save(ctx context.Context, vm *domain.VirtualMachine) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(toVMModel(vm)).Error
	})
}

func (r *GormVMRepository) FindByID(ctx context.Context, id string) (*domain.VirtualMachine, error) {
	var model VMModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrVMNotFound
		}
		return nil, err
	}
	return toVMDomain(&model), nil
}

func (r *GormVMRepository) FindByCustomerID(ctx context.Context, customerID string) ([]*domain.VirtualMachine, error) {
	return r.findWhere(ctx, "customer_id = ?", customerID)
}

func (r *GormVMRepository) FindByNodeID(ctx context.Context, nodeID string) ([]*domain.VirtualMachine, error) {
	return r.findWhere(ctx, "node_id = ?", nodeID)
}

func (r *GormVMRepository) FindAll(ctx context.Context) ([]*domain.VirtualMachine, error) {
	return r.findWhere(ctx, "")
}

func (r *GormVMRepository) UpdateStatus(ctx context.Context, id string, status domain.VMStatus) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Model(&VMModel{}).
			Where("id = ?", id).
			Updates(map[string]any{
				"status":     string(status),
				"updated_at": time.Now().UTC(),
			}).Error
	})
}

func (r *GormVMRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&VMModel{}).Error
	})
}

func (r *GormVMRepository) findWhere(ctx context.Context, condition string, args ...any) ([]*domain.VirtualMachine, error) {
	var vms []*domain.VirtualMachine
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []VMModel
		q := tx.Order("created_at DESC")
		if condition != "" {
			q = q.Where(condition, args...)
		}
		if err := q.Find(&models).Error; err != nil {
			return err
		}
		vms = make([]*domain.VirtualMachine, len(models))
		for i := range models {
			vms[i] = toVMDomain(&models[i])
		}
		return nil
	})
	return vms, err
}
