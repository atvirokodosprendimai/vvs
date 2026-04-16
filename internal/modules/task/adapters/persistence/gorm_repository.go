package persistence

import (
	"context"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/modules/task/domain"
	"gorm.io/gorm"
)

// GormTaskRepository implements domain.TaskRepository using GORM + SQLite.
type GormTaskRepository struct {
	db *gormsqlite.DB
}

func NewGormTaskRepository(db *gormsqlite.DB) *GormTaskRepository {
	return &GormTaskRepository{db: db}
}

func (r *GormTaskRepository) Save(ctx context.Context, task *domain.Task) error {
	model := toModel(task)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(&model).Error
	})
}

func (r *GormTaskRepository) FindByID(ctx context.Context, id string) (*domain.Task, error) {
	var model TaskModel
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

func (r *GormTaskRepository) ListForCustomer(ctx context.Context, customerID string) ([]*domain.Task, error) {
	var models []TaskModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("customer_id = ?", customerID).Order("created_at DESC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	return modelsToDomain(models), nil
}

func (r *GormTaskRepository) ListAll(ctx context.Context) ([]*domain.Task, error) {
	var models []TaskModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Order("created_at DESC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	return modelsToDomain(models), nil
}

func (r *GormTaskRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		result := tx.Where("id = ?", id).Delete(&TaskModel{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func modelsToDomain(models []TaskModel) []*domain.Task {
	result := make([]*domain.Task, len(models))
	for i := range models {
		result[i] = models[i].toDomain()
	}
	return result
}
