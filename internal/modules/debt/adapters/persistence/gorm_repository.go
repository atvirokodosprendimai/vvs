package persistence

import (
	"context"

	"github.com/vvs/isp/internal/infrastructure/database"
	"github.com/vvs/isp/internal/modules/debt/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormDebtRepository struct {
	writer *database.WriteSerializer
	reader *gorm.DB
}

func NewGormDebtRepository(writer *database.WriteSerializer, reader *gorm.DB) *GormDebtRepository {
	return &GormDebtRepository{writer: writer, reader: reader}
}

// Upsert inserts or replaces the debt status for a customer (conflict on customer_id).
func (r *GormDebtRepository) Upsert(ctx context.Context, s *domain.DebtStatus) error {
	model := toModel(s)
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "customer_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"id", "tax_id", "over_credit_budget", "synced_at"}),
		}).Create(model).Error
	})
}

func (r *GormDebtRepository) ListAll(_ context.Context) ([]*domain.DebtStatus, error) {
	var models []DebtStatusModel
	if err := r.reader.Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]*domain.DebtStatus, len(models))
	for i, m := range models {
		result[i] = toDomain(&m)
	}
	return result, nil
}
