package persistence

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/cron/domain"
)

// GormJobRepository implements domain.JobRepository using GORM + SQLite.
type GormJobRepository struct {
	db *gormsqlite.DB
}

func NewGormJobRepository(db *gormsqlite.DB) *GormJobRepository {
	return &GormJobRepository{db: db}
}

func (r *GormJobRepository) Save(ctx context.Context, j *domain.Job) error {
	model := toModel(j)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(&model).Error
	})
}

func (r *GormJobRepository) FindByID(ctx context.Context, id string) (*domain.Job, error) {
	var model JobModel
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

func (r *GormJobRepository) FindByName(ctx context.Context, name string) (*domain.Job, error) {
	var model JobModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("name = ?", name).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return model.toDomain(), nil
}

func (r *GormJobRepository) ListDue(ctx context.Context, before time.Time) ([]*domain.Job, error) {
	var models []JobModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("status = ? AND next_run <= ?", domain.StatusActive, before).
			Order("next_run ASC").
			Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	jobs := make([]*domain.Job, len(models))
	for i := range models {
		jobs[i] = models[i].toDomain()
	}
	return jobs, nil
}

func (r *GormJobRepository) ListAll(ctx context.Context) ([]*domain.Job, error) {
	var models []JobModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Order("name ASC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	jobs := make([]*domain.Job, len(models))
	for i := range models {
		jobs[i] = models[i].toDomain()
	}
	return jobs, nil
}
