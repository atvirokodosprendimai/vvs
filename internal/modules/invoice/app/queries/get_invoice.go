package queries

import (
	"context"
	"time"

	"github.com/vvs/isp/internal/modules/invoice/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
	"gorm.io/gorm"
)

type GetInvoiceQuery struct {
	ID string
}

type InvoiceDetailModel struct {
	ID               string `gorm:"primaryKey"`
	InvoiceNumber    string
	CustomerID       string
	CustomerName     string
	SubtotalAmount   int64
	SubtotalCurrency string
	TaxRate          int
	TaxAmount        int64
	TaxCurrency      string
	TotalAmount      int64
	TotalCurrency    string
	Status           string
	IssueDate        time.Time
	DueDate          time.Time
	PaidDate         *time.Time
	RecurringID      *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (InvoiceDetailModel) TableName() string { return "invoices" }

type InvoiceLineReadModel struct {
	ID                string `gorm:"primaryKey"`
	InvoiceID         string
	ProductID         string
	ProductName       string
	Description       string
	Quantity          int
	UnitPriceAmount   int64
	UnitPriceCurrency string
	TotalAmount       int64
	TotalCurrency     string
	SortOrder         int
}

func (InvoiceLineReadModel) TableName() string { return "invoice_lines" }

type GetInvoiceResult struct {
	Invoice *domain.Invoice
}

type GetInvoiceHandler struct {
	db *gorm.DB
}

func NewGetInvoiceHandler(db *gorm.DB) *GetInvoiceHandler {
	return &GetInvoiceHandler{db: db}
}

func (h *GetInvoiceHandler) Handle(_ context.Context, q GetInvoiceQuery) (*domain.Invoice, error) {
	var model InvoiceDetailModel
	if err := h.db.Table("invoices").Where("id = ?", q.ID).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrInvoiceNotFound
		}
		return nil, err
	}

	var lineModels []InvoiceLineReadModel
	if err := h.db.Table("invoice_lines").Where("invoice_id = ?", q.ID).Order("sort_order ASC").Find(&lineModels).Error; err != nil {
		return nil, err
	}

	lines := make([]domain.InvoiceLine, len(lineModels))
	for i, lm := range lineModels {
		lines[i] = domain.InvoiceLine{
			ID:          lm.ID,
			ProductID:   lm.ProductID,
			ProductName: lm.ProductName,
			Description: lm.Description,
			Quantity:    lm.Quantity,
			UnitPrice:   shareddomain.NewMoney(lm.UnitPriceAmount, lm.UnitPriceCurrency),
			Total:       shareddomain.NewMoney(lm.TotalAmount, lm.TotalCurrency),
		}
	}

	return &domain.Invoice{
		ID:            model.ID,
		InvoiceNumber: model.InvoiceNumber,
		CustomerID:    model.CustomerID,
		CustomerName:  model.CustomerName,
		Lines:         lines,
		Subtotal:      shareddomain.NewMoney(model.SubtotalAmount, model.SubtotalCurrency),
		TaxRate:       model.TaxRate,
		TaxAmount:     shareddomain.NewMoney(model.TaxAmount, model.TaxCurrency),
		Total:         shareddomain.NewMoney(model.TotalAmount, model.TotalCurrency),
		Status:        domain.InvoiceStatus(model.Status),
		IssueDate:     model.IssueDate,
		DueDate:       model.DueDate,
		PaidDate:      model.PaidDate,
		RecurringID:   model.RecurringID,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}, nil
}
