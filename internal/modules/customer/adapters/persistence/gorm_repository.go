package persistence

import (
	"context"
	"fmt"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/customer/domain"
	shareddomain "github.com/atvirokodosprendimai/vvs/internal/shared/domain"
	"gorm.io/gorm"
)

type GormCustomerRepository struct {
	db *gormsqlite.DB
}

func NewGormCustomerRepository(db *gormsqlite.DB) *GormCustomerRepository {
	return &GormCustomerRepository{db: db}
}

func (r *GormCustomerRepository) NextCode(ctx context.Context) (shareddomain.CompanyCode, error) {
	var code shareddomain.CompanyCode
	err := r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
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
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(model).Error
	})
}

func (r *GormCustomerRepository) FindByID(ctx context.Context, id string) (*domain.Customer, error) {
	var model CustomerModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrCustomerNotFound
		}
		return nil, err
	}
	return toDomain(&model), nil
}

func (r *GormCustomerRepository) FindByCode(ctx context.Context, code string) (*domain.Customer, error) {
	var model CustomerModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("code = ?", code).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrCustomerNotFound
		}
		return nil, err
	}
	return toDomain(&model), nil
}

func (r *GormCustomerRepository) FindAll(ctx context.Context, filter domain.CustomerFilter, page shareddomain.Pagination) ([]*domain.Customer, int64, error) {
	var customers []*domain.Customer
	var total int64

	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		query := tx.Model(&CustomerModel{})

		if filter.Search != "" {
			search := "%" + filter.Search + "%"
			query = query.Where("company_name LIKE ? OR code LIKE ? OR email LIKE ? OR contact_name LIKE ?",
				search, search, search, search)
		}
		if filter.Status != "" {
			query = query.Where("status = ?", filter.Status)
		}

		if err := query.Count(&total).Error; err != nil {
			return err
		}

		var models []CustomerModel
		if err := query.Order("created_at DESC").
			Offset(page.Offset()).Limit(page.PageSize).
			Find(&models).Error; err != nil {
			return err
		}

		customers = make([]*domain.Customer, len(models))
		for i, m := range models {
			customers[i] = toDomain(&m)
		}
		return nil
	})

	return customers, total, err
}

func (r *GormCustomerRepository) FindByEmail(ctx context.Context, email string) (*domain.Customer, error) {
	var model CustomerModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("LOWER(email) = LOWER(?)", email).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrCustomerNotFound
		}
		return nil, err
	}
	return toDomain(&model), nil
}

func (r *GormCustomerRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Delete(&CustomerModel{}, "id = ?", id).Error
	})
}
