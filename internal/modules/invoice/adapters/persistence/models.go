package persistence

import (
	"time"

	"github.com/vvs/isp/internal/modules/invoice/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
)

type InvoiceModel struct {
	ID               string `gorm:"primaryKey;type:text"`
	InvoiceNumber    string `gorm:"uniqueIndex;type:text;not null"`
	CustomerID       string `gorm:"type:text;not null"`
	CustomerName     string `gorm:"type:text;not null"`
	SubtotalAmount   int64  `gorm:"not null;default:0"`
	SubtotalCurrency string `gorm:"type:text;not null;default:'EUR'"`
	TaxRate          int    `gorm:"not null;default:21"`
	TaxAmount        int64  `gorm:"not null;default:0"`
	TaxCurrency      string `gorm:"type:text;not null;default:'EUR'"`
	TotalAmount      int64  `gorm:"not null;default:0"`
	TotalCurrency    string `gorm:"type:text;not null;default:'EUR'"`
	Status           string `gorm:"type:text;not null;default:'draft'"`
	IssueDate        time.Time
	DueDate          time.Time
	PaidDate         *time.Time
	RecurringID      *string   `gorm:"type:text"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (InvoiceModel) TableName() string { return "invoices" }

type InvoiceLineModel struct {
	ID                string `gorm:"primaryKey;type:text"`
	InvoiceID         string `gorm:"type:text;not null"`
	ProductID         string `gorm:"type:text;default:''"`
	ProductName       string `gorm:"type:text;default:''"`
	Description       string `gorm:"type:text;default:''"`
	Quantity          int    `gorm:"not null;default:1"`
	UnitPriceAmount   int64  `gorm:"not null;default:0"`
	UnitPriceCurrency string `gorm:"type:text;not null;default:'EUR'"`
	TotalAmount       int64  `gorm:"not null;default:0"`
	TotalCurrency     string `gorm:"type:text;not null;default:'EUR'"`
	SortOrder         int    `gorm:"not null;default:0"`
}

func (InvoiceLineModel) TableName() string { return "invoice_lines" }

func toModel(inv *domain.Invoice) (*InvoiceModel, []InvoiceLineModel) {
	model := &InvoiceModel{
		ID:               inv.ID,
		InvoiceNumber:    inv.InvoiceNumber,
		CustomerID:       inv.CustomerID,
		CustomerName:     inv.CustomerName,
		SubtotalAmount:   inv.Subtotal.Amount,
		SubtotalCurrency: inv.Subtotal.Currency,
		TaxRate:          inv.TaxRate,
		TaxAmount:        inv.TaxAmount.Amount,
		TaxCurrency:      inv.TaxAmount.Currency,
		TotalAmount:      inv.Total.Amount,
		TotalCurrency:    inv.Total.Currency,
		Status:           string(inv.Status),
		IssueDate:        inv.IssueDate,
		DueDate:          inv.DueDate,
		PaidDate:         inv.PaidDate,
		RecurringID:      inv.RecurringID,
		CreatedAt:        inv.CreatedAt,
		UpdatedAt:        inv.UpdatedAt,
	}

	lines := make([]InvoiceLineModel, len(inv.Lines))
	for i, l := range inv.Lines {
		lines[i] = InvoiceLineModel{
			ID:                l.ID,
			InvoiceID:         inv.ID,
			ProductID:         l.ProductID,
			ProductName:       l.ProductName,
			Description:       l.Description,
			Quantity:          l.Quantity,
			UnitPriceAmount:   l.UnitPrice.Amount,
			UnitPriceCurrency: l.UnitPrice.Currency,
			TotalAmount:       l.Total.Amount,
			TotalCurrency:     l.Total.Currency,
			SortOrder:         i,
		}
	}

	return model, lines
}

func toDomain(m *InvoiceModel, lineModels []InvoiceLineModel) *domain.Invoice {
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
		ID:            m.ID,
		InvoiceNumber: m.InvoiceNumber,
		CustomerID:    m.CustomerID,
		CustomerName:  m.CustomerName,
		Lines:         lines,
		Subtotal:      shareddomain.NewMoney(m.SubtotalAmount, m.SubtotalCurrency),
		TaxRate:       m.TaxRate,
		TaxAmount:     shareddomain.NewMoney(m.TaxAmount, m.TaxCurrency),
		Total:         shareddomain.NewMoney(m.TotalAmount, m.TotalCurrency),
		Status:        domain.InvoiceStatus(m.Status),
		IssueDate:     m.IssueDate,
		DueDate:       m.DueDate,
		PaidDate:      m.PaidDate,
		RecurringID:   m.RecurringID,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}
