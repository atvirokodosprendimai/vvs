package queries

import (
	"context"
	"time"

	shareddomain "github.com/vvs/isp/internal/shared/domain"
	"gorm.io/gorm"
)

type ListInvoicesQuery struct {
	Search     string
	Status     string
	CustomerID string
	Page       int
	PageSize   int
}

type ListInvoicesResult struct {
	Invoices   []InvoiceReadModel
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

type InvoiceReadModel struct {
	ID             string `gorm:"primaryKey"`
	InvoiceNumber  string
	CustomerID     string
	CustomerName   string
	SubtotalAmount int64
	SubtotalCurrency string
	TaxRate        int
	TaxAmount      int64
	TaxCurrency    string
	TotalAmount    int64
	TotalCurrency  string
	Status         string
	IssueDate      time.Time
	DueDate        time.Time
	PaidDate       *time.Time
	RecurringID    *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (InvoiceReadModel) TableName() string { return "invoices" }

type ListInvoicesHandler struct {
	db *gorm.DB
}

func NewListInvoicesHandler(db *gorm.DB) *ListInvoicesHandler {
	return &ListInvoicesHandler{db: db}
}

func (h *ListInvoicesHandler) Handle(_ context.Context, q ListInvoicesQuery) (ListInvoicesResult, error) {
	page := shareddomain.NewPagination(q.Page, q.PageSize)

	query := h.db.Table("invoices")

	if q.Search != "" {
		search := "%" + q.Search + "%"
		query = query.Where("invoice_number LIKE ? OR customer_name LIKE ?", search, search)
	}

	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	}

	if q.CustomerID != "" {
		query = query.Where("customer_id = ?", q.CustomerID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ListInvoicesResult{}, err
	}

	var models []InvoiceReadModel
	if err := query.Order("created_at DESC").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&models).Error; err != nil {
		return ListInvoicesResult{}, err
	}

	return ListInvoicesResult{
		Invoices:   models,
		Total:      total,
		Page:       page.Page,
		PageSize:   page.PageSize,
		TotalPages: page.TotalPages(total),
	}, nil
}
