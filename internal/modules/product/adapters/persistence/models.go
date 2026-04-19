package persistence

import (
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/product/domain"
	shareddomain "github.com/atvirokodosprendimai/vvs/internal/shared/domain"
)

type ProductModel struct {
	ID            string `gorm:"primaryKey;type:text"`
	Name          string `gorm:"type:text;not null"`
	Description   string `gorm:"type:text"`
	Type          string `gorm:"type:text;not null;default:'internet'"`
	PriceAmount   int64  `gorm:"not null;default:0"`
	PriceCurrency string `gorm:"type:text;not null;default:'EUR'"`
	BillingPeriod string `gorm:"type:text;not null;default:'monthly'"`
	IsActive      bool   `gorm:"not null;default:true"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (ProductModel) TableName() string { return "products" }

func toModel(p *domain.Product) *ProductModel {
	return &ProductModel{
		ID:            p.ID,
		Name:          p.Name,
		Description:   p.Description,
		Type:          string(p.Type),
		PriceAmount:   p.Price.Amount,
		PriceCurrency: p.Price.Currency,
		BillingPeriod: string(p.BillingPeriod),
		IsActive:      p.IsActive,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}
}

func toDomain(m *ProductModel) *domain.Product {
	return &domain.Product{
		ID:            m.ID,
		Name:          m.Name,
		Description:   m.Description,
		Type:          domain.ProductType(m.Type),
		Price:         shareddomain.NewMoney(m.PriceAmount, m.PriceCurrency),
		BillingPeriod: domain.BillingPeriod(m.BillingPeriod),
		IsActive:      m.IsActive,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}
