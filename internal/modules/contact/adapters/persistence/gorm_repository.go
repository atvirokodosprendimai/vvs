package persistence

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/contact/domain"
)

type contactModel struct {
	ID         string    `gorm:"primaryKey;type:text"`
	CustomerID string    `gorm:"type:text;not null;index"`
	FirstName  string    `gorm:"type:text;not null"`
	LastName   string    `gorm:"type:text;not null;default:''"`
	Email      string    `gorm:"type:text;not null;default:''"`
	Phone      string    `gorm:"type:text;not null;default:''"`
	Role       string    `gorm:"type:text;not null;default:''"`
	Notes      string    `gorm:"type:text;not null;default:''"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (contactModel) TableName() string { return "contacts" }

func (m *contactModel) toDomain() *domain.Contact {
	return &domain.Contact{
		ID:         m.ID,
		CustomerID: m.CustomerID,
		FirstName:  m.FirstName,
		LastName:   m.LastName,
		Email:      m.Email,
		Phone:      m.Phone,
		Role:       m.Role,
		Notes:      m.Notes,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}

func toModel(c *domain.Contact) contactModel {
	return contactModel{
		ID:         c.ID,
		CustomerID: c.CustomerID,
		FirstName:  c.FirstName,
		LastName:   c.LastName,
		Email:      c.Email,
		Phone:      c.Phone,
		Role:       c.Role,
		Notes:      c.Notes,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
}

type GormContactRepository struct {
	db *gormsqlite.DB
}

func NewGormContactRepository(db *gormsqlite.DB) *GormContactRepository {
	return &GormContactRepository{db: db}
}

func (r *GormContactRepository) Save(ctx context.Context, c *domain.Contact) error {
	m := toModel(c)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(&m).Error
	})
}

func (r *GormContactRepository) FindByID(ctx context.Context, id string) (*domain.Contact, error) {
	var m contactModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return m.toDomain(), nil
}

func (r *GormContactRepository) ListForCustomer(ctx context.Context, customerID string) ([]*domain.Contact, error) {
	var models []contactModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("customer_id = ?", customerID).Order("created_at ASC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Contact, len(models))
	for i := range models {
		result[i] = models[i].toDomain()
	}
	return result, nil
}

func (r *GormContactRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&contactModel{}).Error
	})
}
