package persistence

import (
	"context"

	"github.com/vvs/isp/internal/infrastructure/database"
	"github.com/vvs/isp/internal/modules/payment/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
	"gorm.io/gorm"
)

type GormPaymentRepository struct {
	writer *database.WriteSerializer
	reader *gorm.DB
}

func NewGormPaymentRepository(writer *database.WriteSerializer, reader *gorm.DB) *GormPaymentRepository {
	return &GormPaymentRepository{writer: writer, reader: reader}
}

func (r *GormPaymentRepository) Save(ctx context.Context, payment *domain.Payment) error {
	model := toModel(payment)
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		return tx.Save(model).Error
	})
}

func (r *GormPaymentRepository) SaveBatch(ctx context.Context, payments []*domain.Payment) error {
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		for _, p := range payments {
			model := toModel(p)
			if err := tx.Create(model).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *GormPaymentRepository) FindByID(_ context.Context, id string) (*domain.Payment, error) {
	var model PaymentModel
	if err := r.reader.Where("id = ?", id).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, err
	}
	return toDomain(&model), nil
}

func (r *GormPaymentRepository) FindUnmatched(_ context.Context) ([]*domain.Payment, error) {
	var models []PaymentModel
	if err := r.reader.Where("status IN (?, ?)", string(domain.StatusImported), string(domain.StatusUnmatched)).
		Order("booking_date DESC").Find(&models).Error; err != nil {
		return nil, err
	}

	payments := make([]*domain.Payment, len(models))
	for i, m := range models {
		payments[i] = toDomain(&m)
	}
	return payments, nil
}

func (r *GormPaymentRepository) FindByInvoice(_ context.Context, invoiceID string) ([]*domain.Payment, error) {
	var models []PaymentModel
	if err := r.reader.Where("invoice_id = ?", invoiceID).
		Order("booking_date DESC").Find(&models).Error; err != nil {
		return nil, err
	}

	payments := make([]*domain.Payment, len(models))
	for i, m := range models {
		payments[i] = toDomain(&m)
	}
	return payments, nil
}

func (r *GormPaymentRepository) FindAll(_ context.Context, filter domain.PaymentFilter, page shareddomain.Pagination) ([]*domain.Payment, int64, error) {
	query := r.reader.Model(&PaymentModel{})

	if filter.Search != "" {
		search := "%" + filter.Search + "%"
		query = query.Where("reference LIKE ? OR payer_name LIKE ? OR payer_iban LIKE ?",
			search, search, search)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.CustomerID != "" {
		query = query.Where("customer_id = ?", filter.CustomerID)
	}
	if filter.InvoiceID != "" {
		query = query.Where("invoice_id = ?", filter.InvoiceID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var models []PaymentModel
	if err := query.Order("booking_date DESC, created_at DESC").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&models).Error; err != nil {
		return nil, 0, err
	}

	payments := make([]*domain.Payment, len(models))
	for i, m := range models {
		payments[i] = toDomain(&m)
	}

	return payments, total, nil
}
