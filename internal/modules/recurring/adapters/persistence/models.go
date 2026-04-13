package persistence

import (
	"time"

	"github.com/vvs/isp/internal/modules/recurring/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
)

type RecurringModel struct {
	ID           string     `gorm:"primaryKey;type:text"`
	CustomerID   string     `gorm:"type:text;not null"`
	CustomerName string     `gorm:"type:text;not null"`
	Frequency    string     `gorm:"type:text;not null;default:'monthly'"`
	DayOfMonth   int        `gorm:"not null;default:1"`
	NextRunDate  time.Time  `gorm:"not null"`
	LastRunDate  *time.Time
	Status       string     `gorm:"type:text;not null;default:'active'"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (RecurringModel) TableName() string { return "recurring_invoices" }

type RecurringLineModel struct {
	ID                string `gorm:"primaryKey;type:text"`
	RecurringID       string `gorm:"type:text;not null"`
	ProductID         string `gorm:"type:text"`
	ProductName       string `gorm:"type:text"`
	Description       string `gorm:"type:text"`
	Quantity          int    `gorm:"not null;default:1"`
	UnitPriceAmount   int64  `gorm:"not null;default:0"`
	UnitPriceCurrency string `gorm:"type:text;not null;default:'EUR'"`
	SortOrder         int    `gorm:"not null;default:0"`
}

func (RecurringLineModel) TableName() string { return "recurring_lines" }

func toModel(r *domain.RecurringInvoice) (*RecurringModel, []RecurringLineModel) {
	model := &RecurringModel{
		ID:           r.ID,
		CustomerID:   r.CustomerID,
		CustomerName: r.CustomerName,
		Frequency:    string(r.Schedule.Frequency),
		DayOfMonth:   r.Schedule.DayOfMonth,
		NextRunDate:  r.NextRunDate,
		LastRunDate:  r.LastRunDate,
		Status:       string(r.Status),
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}

	lines := make([]RecurringLineModel, len(r.Lines))
	for i, l := range r.Lines {
		lines[i] = RecurringLineModel{
			ID:                l.ID,
			RecurringID:       r.ID,
			ProductID:         l.ProductID,
			ProductName:       l.ProductName,
			Description:       l.Description,
			Quantity:          l.Quantity,
			UnitPriceAmount:   l.UnitPrice.Amount,
			UnitPriceCurrency: l.UnitPrice.Currency,
			SortOrder:         l.SortOrder,
		}
	}

	return model, lines
}

func toDomain(m *RecurringModel, lines []RecurringLineModel) *domain.RecurringInvoice {
	inv := &domain.RecurringInvoice{
		ID:           m.ID,
		CustomerID:   m.CustomerID,
		CustomerName: m.CustomerName,
		Schedule: domain.Schedule{
			Frequency:  domain.Frequency(m.Frequency),
			DayOfMonth: m.DayOfMonth,
		},
		NextRunDate: m.NextRunDate,
		LastRunDate: m.LastRunDate,
		Status:      domain.RecurringStatus(m.Status),
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}

	inv.Lines = make([]domain.RecurringLine, len(lines))
	for i, l := range lines {
		inv.Lines[i] = domain.RecurringLine{
			ID:          l.ID,
			ProductID:   l.ProductID,
			ProductName: l.ProductName,
			Description: l.Description,
			Quantity:    l.Quantity,
			UnitPrice:   shareddomain.NewMoney(l.UnitPriceAmount, l.UnitPriceCurrency),
			SortOrder:   l.SortOrder,
		}
	}

	return inv
}
