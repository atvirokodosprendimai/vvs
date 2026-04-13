package persistence

import (
	"context"
	"time"

	"github.com/vvs/isp/internal/infrastructure/database"
	"github.com/vvs/isp/internal/modules/recurring/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
	"gorm.io/gorm"
)

type GormRecurringRepository struct {
	writer *database.WriteSerializer
	reader *gorm.DB
}

func NewGormRecurringRepository(writer *database.WriteSerializer, reader *gorm.DB) *GormRecurringRepository {
	return &GormRecurringRepository{writer: writer, reader: reader}
}

func (r *GormRecurringRepository) Save(ctx context.Context, invoice *domain.RecurringInvoice) error {
	model, lines := toModel(invoice)
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		if err := tx.Save(model).Error; err != nil {
			return err
		}
		// Delete existing lines and re-insert
		if err := tx.Where("recurring_id = ?", invoice.ID).Delete(&RecurringLineModel{}).Error; err != nil {
			return err
		}
		for _, line := range lines {
			if err := tx.Create(&line).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *GormRecurringRepository) FindByID(_ context.Context, id string) (*domain.RecurringInvoice, error) {
	var model RecurringModel
	if err := r.reader.Where("id = ?", id).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrRecurringNotFound
		}
		return nil, err
	}

	var lines []RecurringLineModel
	r.reader.Where("recurring_id = ?", id).Order("sort_order ASC").Find(&lines)

	return toDomain(&model, lines), nil
}

func (r *GormRecurringRepository) FindByCustomer(_ context.Context, customerID string) ([]*domain.RecurringInvoice, error) {
	var models []RecurringModel
	if err := r.reader.Where("customer_id = ?", customerID).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}

	invoices := make([]*domain.RecurringInvoice, len(models))
	for i, m := range models {
		var lines []RecurringLineModel
		r.reader.Where("recurring_id = ?", m.ID).Order("sort_order ASC").Find(&lines)
		invoices[i] = toDomain(&m, lines)
	}

	return invoices, nil
}

func (r *GormRecurringRepository) FindDueForGeneration(_ context.Context, asOf time.Time) ([]*domain.RecurringInvoice, error) {
	var models []RecurringModel
	if err := r.reader.Where("status = ? AND next_run_date <= ?", string(domain.StatusActive), asOf).
		Order("next_run_date ASC").Find(&models).Error; err != nil {
		return nil, err
	}

	invoices := make([]*domain.RecurringInvoice, len(models))
	for i, m := range models {
		var lines []RecurringLineModel
		r.reader.Where("recurring_id = ?", m.ID).Order("sort_order ASC").Find(&lines)
		invoices[i] = toDomain(&m, lines)
	}

	return invoices, nil
}

func (r *GormRecurringRepository) FindAll(_ context.Context, filter domain.RecurringFilter, page shareddomain.Pagination) ([]*domain.RecurringInvoice, int64, error) {
	query := r.reader.Model(&RecurringModel{})

	if filter.Search != "" {
		search := "%" + filter.Search + "%"
		query = query.Where("customer_name LIKE ?", search)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.CustomerID != "" {
		query = query.Where("customer_id = ?", filter.CustomerID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var models []RecurringModel
	if err := query.Order("created_at DESC").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&models).Error; err != nil {
		return nil, 0, err
	}

	invoices := make([]*domain.RecurringInvoice, len(models))
	for i, m := range models {
		var lines []RecurringLineModel
		r.reader.Where("recurring_id = ?", m.ID).Order("sort_order ASC").Find(&lines)
		invoices[i] = toDomain(&m, lines)
	}

	return invoices, total, nil
}

func (r *GormRecurringRepository) Delete(ctx context.Context, id string) error {
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		if err := tx.Where("recurring_id = ?", id).Delete(&RecurringLineModel{}).Error; err != nil {
			return err
		}
		return tx.Delete(&RecurringModel{}, "id = ?", id).Error
	})
}
