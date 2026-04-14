package queries

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type DebtStatusRow struct {
	CustomerID       string
	CustomerCode     string
	CompanyName      string
	TaxID            string
	OverCreditBudget bool
	SyncedAt         time.Time
}

type ListDebtStatusesHandler struct {
	db *gorm.DB
}

func NewListDebtStatusesHandler(db *gorm.DB) *ListDebtStatusesHandler {
	return &ListDebtStatusesHandler{db: db}
}

// Handle returns all customers with a known debt status, debtors first.
func (h *ListDebtStatusesHandler) Handle(_ context.Context) ([]DebtStatusRow, error) {
	var rows []DebtStatusRow
	err := h.db.Table("debt_statuses ds").
		Select("ds.customer_id, c.code as customer_code, c.company_name, ds.tax_id, ds.over_credit_budget, ds.synced_at").
		Joins("JOIN customers c ON c.id = ds.customer_id").
		Order("ds.over_credit_budget DESC, c.company_name ASC").
		Scan(&rows).Error
	return rows, err
}
