package persistence

import (
	"time"

	"github.com/vvs/isp/internal/modules/debt/domain"
)

type DebtStatusModel struct {
	ID               string    `gorm:"primaryKey;type:text"`
	CustomerID       string    `gorm:"uniqueIndex;type:text;not null"`
	TaxID            string    `gorm:"type:text;not null"`
	OverCreditBudget bool      `gorm:"not null;default:false"`
	SyncedAt         time.Time `gorm:"not null"`
}

func (DebtStatusModel) TableName() string { return "debt_statuses" }

func toModel(s *domain.DebtStatus) *DebtStatusModel {
	return &DebtStatusModel{
		ID:               s.ID,
		CustomerID:       s.CustomerID,
		TaxID:            s.TaxID,
		OverCreditBudget: s.OverCreditBudget,
		SyncedAt:         s.SyncedAt,
	}
}

func toDomain(m *DebtStatusModel) *domain.DebtStatus {
	return &domain.DebtStatus{
		ID:               m.ID,
		CustomerID:       m.CustomerID,
		TaxID:            m.TaxID,
		OverCreditBudget: m.OverCreditBudget,
		SyncedAt:         m.SyncedAt,
	}
}
