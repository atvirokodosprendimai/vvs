package persistence

import (
	"context"
	"fmt"

	"github.com/vvs/isp/internal/infrastructure/database"
	"github.com/vvs/isp/internal/modules/customer/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
	"gorm.io/gorm"
)

type GormCustomerRepository struct {
	writer *database.WriteSerializer
	reader *gorm.DB
}

func NewGormCustomerRepository(writer *database.WriteSerializer, reader *gorm.DB) *GormCustomerRepository {
	return &GormCustomerRepository{writer: writer, reader: reader}
}

func (r *GormCustomerRepository) NextCode(ctx context.Context) (shareddomain.CompanyCode, error) {
	var code shareddomain.CompanyCode
	err := r.writer.Execute(ctx, func(tx *gorm.DB) error {
		var seq struct {
			Prefix     string
			LastNumber int
		}
		if err := tx.Raw(
			"UPDATE company_code_sequences SET last_number = last_number + 1 WHERE prefix = 'CLI' RETURNING prefix, last_number",
		).Scan(&seq).Error; err != nil {
			return fmt.Errorf("next code: %w", err)
		}
		code = shareddomain.NewCompanyCode(seq.Prefix, seq.LastNumber)
		return nil
	})
	return code, err
}

func (r *GormCustomerRepository) Save(ctx context.Context, customer *domain.Customer) error {
	model := toModel(customer)
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		return tx.Save(model).Error
	})
}

func (r *GormCustomerRepository) FindByID(_ context.Context, id string) (*domain.Customer, error) {
	var model CustomerModel
	if err := r.reader.Where("id = ?", id).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrCustomerNotFound
		}
		return nil, err
	}
	return toDomain(&model), nil
}

func (r *GormCustomerRepository) FindByCode(_ context.Context, code string) (*domain.Customer, error) {
	var model CustomerModel
	if err := r.reader.Where("code = ?", code).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrCustomerNotFound
		}
		return nil, err
	}
	return toDomain(&model), nil
}

func (r *GormCustomerRepository) FindAll(_ context.Context, filter domain.CustomerFilter, page shareddomain.Pagination) ([]*domain.Customer, int64, error) {
	query := r.reader.Model(&CustomerModel{})

	if filter.Search != "" {
		search := "%" + filter.Search + "%"
		query = query.Where("company_name LIKE ? OR code LIKE ? OR email LIKE ? OR contact_name LIKE ?",
			search, search, search, search)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var models []CustomerModel
	if err := query.Order("created_at DESC").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&models).Error; err != nil {
		return nil, 0, err
	}

	customers := make([]*domain.Customer, len(models))
	for i, m := range models {
		customers[i] = toDomain(&m)
	}

	return customers, total, nil
}

func (r *GormCustomerRepository) Delete(ctx context.Context, id string) error {
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		return tx.Delete(&CustomerModel{}, "id = ?", id).Error
	})
}
