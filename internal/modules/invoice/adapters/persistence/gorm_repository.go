package persistence

import (
	"context"
	"fmt"

	"github.com/vvs/isp/internal/infrastructure/database"
	"github.com/vvs/isp/internal/modules/invoice/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
	"gorm.io/gorm"
)

type GormInvoiceRepository struct {
	writer *database.WriteSerializer
	reader *gorm.DB
}

func NewGormInvoiceRepository(writer *database.WriteSerializer, reader *gorm.DB) *GormInvoiceRepository {
	return &GormInvoiceRepository{writer: writer, reader: reader}
}

func (r *GormInvoiceRepository) NextInvoiceNumber(ctx context.Context, year int) (string, error) {
	var number string
	err := r.writer.Execute(ctx, func(tx *gorm.DB) error {
		// Upsert the sequence row for this year
		if err := tx.Exec(
			"INSERT INTO invoice_number_sequences (year, last_number) VALUES (?, 0) ON CONFLICT(year) DO NOTHING", year,
		).Error; err != nil {
			return fmt.Errorf("init sequence: %w", err)
		}

		var seq struct {
			Year       int
			LastNumber int
		}
		if err := tx.Raw(
			"UPDATE invoice_number_sequences SET last_number = last_number + 1 WHERE year = ? RETURNING year, last_number", year,
		).Scan(&seq).Error; err != nil {
			return fmt.Errorf("next invoice number: %w", err)
		}
		number = fmt.Sprintf("INV-%d-%05d", seq.Year, seq.LastNumber)
		return nil
	})
	return number, err
}

func (r *GormInvoiceRepository) Save(ctx context.Context, invoice *domain.Invoice) error {
	model, lines := toModel(invoice)
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		if err := tx.Save(model).Error; err != nil {
			return err
		}
		// Delete existing lines and re-insert
		if err := tx.Where("invoice_id = ?", invoice.ID).Delete(&InvoiceLineModel{}).Error; err != nil {
			return err
		}
		if len(lines) > 0 {
			return tx.Create(&lines).Error
		}
		return nil
	})
}

func (r *GormInvoiceRepository) FindByID(_ context.Context, id string) (*domain.Invoice, error) {
	var model InvoiceModel
	if err := r.reader.Where("id = ?", id).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrInvoiceNotFound
		}
		return nil, err
	}

	var lineModels []InvoiceLineModel
	if err := r.reader.Where("invoice_id = ?", id).Order("sort_order ASC").Find(&lineModels).Error; err != nil {
		return nil, err
	}

	return toDomain(&model, lineModels), nil
}

func (r *GormInvoiceRepository) FindByCustomer(_ context.Context, customerID string) ([]*domain.Invoice, error) {
	var models []InvoiceModel
	if err := r.reader.Where("customer_id = ?", customerID).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}

	invoices := make([]*domain.Invoice, len(models))
	for i, m := range models {
		var lineModels []InvoiceLineModel
		if err := r.reader.Where("invoice_id = ?", m.ID).Order("sort_order ASC").Find(&lineModels).Error; err != nil {
			return nil, err
		}
		invoices[i] = toDomain(&m, lineModels)
	}

	return invoices, nil
}

func (r *GormInvoiceRepository) FindOutstanding(_ context.Context) ([]*domain.Invoice, error) {
	var models []InvoiceModel
	if err := r.reader.Where("status IN ?", []string{string(domain.StatusFinalized), string(domain.StatusSent), string(domain.StatusOverdue)}).
		Order("due_date ASC").Find(&models).Error; err != nil {
		return nil, err
	}

	invoices := make([]*domain.Invoice, len(models))
	for i, m := range models {
		var lineModels []InvoiceLineModel
		if err := r.reader.Where("invoice_id = ?", m.ID).Order("sort_order ASC").Find(&lineModels).Error; err != nil {
			return nil, err
		}
		invoices[i] = toDomain(&m, lineModels)
	}

	return invoices, nil
}

func (r *GormInvoiceRepository) FindAll(_ context.Context, filter domain.InvoiceFilter, page shareddomain.Pagination) ([]*domain.Invoice, int64, error) {
	query := r.reader.Model(&InvoiceModel{})

	if filter.Search != "" {
		search := "%" + filter.Search + "%"
		query = query.Where("invoice_number LIKE ? OR customer_name LIKE ?", search, search)
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

	var models []InvoiceModel
	if err := query.Order("created_at DESC").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&models).Error; err != nil {
		return nil, 0, err
	}

	invoices := make([]*domain.Invoice, len(models))
	for i, m := range models {
		var lineModels []InvoiceLineModel
		if err := r.reader.Where("invoice_id = ?", m.ID).Order("sort_order ASC").Find(&lineModels).Error; err != nil {
			return nil, 0, err
		}
		invoices[i] = toDomain(&m, lineModels)
	}

	return invoices, total, nil
}

func (r *GormInvoiceRepository) Delete(ctx context.Context, id string) error {
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		if err := tx.Where("invoice_id = ?", id).Delete(&InvoiceLineModel{}).Error; err != nil {
			return err
		}
		return tx.Delete(&InvoiceModel{}, "id = ?", id).Error
	})
}
