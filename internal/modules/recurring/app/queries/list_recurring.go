package queries

import (
	"context"
	"time"

	"github.com/vvs/isp/internal/modules/recurring/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
	"gorm.io/gorm"
)

type ListRecurringQuery struct {
	Search     string
	Status     string
	CustomerID string
	Page       int
	PageSize   int
}

type ListRecurringResult struct {
	Invoices   []*domain.RecurringInvoice
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

type ListRecurringHandler struct {
	db *gorm.DB
}

func NewListRecurringHandler(db *gorm.DB) *ListRecurringHandler {
	return &ListRecurringHandler{db: db}
}

func (h *ListRecurringHandler) Handle(_ context.Context, q ListRecurringQuery) (ListRecurringResult, error) {
	page := shareddomain.NewPagination(q.Page, q.PageSize)

	query := h.db.Table("recurring_invoices")

	if q.Search != "" {
		search := "%" + q.Search + "%"
		query = query.Where("customer_name LIKE ?", search)
	}

	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	}

	if q.CustomerID != "" {
		query = query.Where("customer_id = ?", q.CustomerID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ListRecurringResult{}, err
	}

	var models []RecurringReadModel
	if err := query.Order("created_at DESC").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&models).Error; err != nil {
		return ListRecurringResult{}, err
	}

	invoices := make([]*domain.RecurringInvoice, len(models))
	for i, m := range models {
		inv := m.ToDomain()
		// Load lines for each invoice
		var lineModels []RecurringLineReadModel
		h.db.Table("recurring_lines").Where("recurring_id = ?", m.ID).Order("sort_order ASC").Find(&lineModels)
		for _, lm := range lineModels {
			inv.Lines = append(inv.Lines, lm.ToDomain())
		}
		invoices[i] = inv
	}

	return ListRecurringResult{
		Invoices:   invoices,
		Total:      total,
		Page:       page.Page,
		PageSize:   page.PageSize,
		TotalPages: page.TotalPages(total),
	}, nil
}

type RecurringReadModel struct {
	ID           string `gorm:"primaryKey"`
	CustomerID   string
	CustomerName string
	Frequency    string
	DayOfMonth   int
	NextRunDate  time.Time
	LastRunDate  *time.Time
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (RecurringReadModel) TableName() string { return "recurring_invoices" }

func (m *RecurringReadModel) ToDomain() *domain.RecurringInvoice {
	return &domain.RecurringInvoice{
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
}

type RecurringLineReadModel struct {
	ID              string `gorm:"primaryKey"`
	RecurringID     string
	ProductID       string
	ProductName     string
	Description     string
	Quantity        int
	UnitPriceAmount int64
	UnitPriceCurrency string
	SortOrder       int
}

func (RecurringLineReadModel) TableName() string { return "recurring_lines" }

func (m *RecurringLineReadModel) ToDomain() domain.RecurringLine {
	return domain.RecurringLine{
		ID:          m.ID,
		ProductID:   m.ProductID,
		ProductName: m.ProductName,
		Description: m.Description,
		Quantity:    m.Quantity,
		UnitPrice:   shareddomain.NewMoney(m.UnitPriceAmount, m.UnitPriceCurrency),
		SortOrder:   m.SortOrder,
	}
}
