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
	ID               string     `gorm:"primaryKey" json:"id"`
	InvoiceNumber    string     `json:"invoice_number"`
	CustomerID       string     `json:"customer_id"`
	CustomerName     string     `json:"customer_name"`
	SubtotalAmount   int64      `json:"subtotal_amount"`
	SubtotalCurrency string     `json:"subtotal_currency"`
	TaxRate          int        `json:"tax_rate"`
	TaxAmount        int64      `json:"tax_amount"`
	TaxCurrency      string     `json:"tax_currency"`
	TotalAmount      int64      `json:"total_amount"`
	TotalCurrency    string     `json:"total_currency"`
	Status           string     `json:"status"`
	IssueDate        time.Time  `json:"issue_date"`
	DueDate          time.Time  `json:"due_date"`
	PaidDate         *time.Time `json:"paid_date"`
	RecurringID      *string    `json:"recurring_id"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
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
