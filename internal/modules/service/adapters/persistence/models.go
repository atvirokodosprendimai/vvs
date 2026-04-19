package persistence

import (
	"time"

	"github.com/vvs/isp/internal/modules/service/domain"
)

// ServiceModel is the GORM model mapping to the customer_services table.
type ServiceModel struct {
	ID              string     `gorm:"primaryKey;type:text"`
	CustomerID      string     `gorm:"type:text;not null"`
	ProductID       string     `gorm:"type:text;not null"`
	ProductName     string     `gorm:"type:text;not null"`
	PriceAmount     int64      `gorm:"not null"`
	Currency        string     `gorm:"type:text;not null;default:'EUR'"`
	StartDate       time.Time  `gorm:"not null"`
	Status          string     `gorm:"type:text;not null;default:'active'"`
	BillingCycle    string     `gorm:"type:text;not null;default:'monthly'"`
	NextBillingDate *time.Time `gorm:"type:datetime"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (ServiceModel) TableName() string { return "customer_services" }

func toModel(s *domain.Service) ServiceModel {
	return ServiceModel{
		ID:              s.ID,
		CustomerID:      s.CustomerID,
		ProductID:       s.ProductID,
		ProductName:     s.ProductName,
		PriceAmount:     s.PriceAmount,
		Currency:        s.Currency,
		StartDate:       s.StartDate,
		Status:          s.Status,
		BillingCycle:    s.BillingCycle,
		NextBillingDate: s.NextBillingDate,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}
}

func (m *ServiceModel) toDomain() *domain.Service {
	return &domain.Service{
		ID:              m.ID,
		CustomerID:      m.CustomerID,
		ProductID:       m.ProductID,
		ProductName:     m.ProductName,
		PriceAmount:     m.PriceAmount,
		Currency:        m.Currency,
		StartDate:       m.StartDate,
		Status:          m.Status,
		BillingCycle:    m.BillingCycle,
		NextBillingDate: m.NextBillingDate,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}
