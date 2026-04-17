package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/modules/invoice/domain"
	"gorm.io/gorm"
)

// ── GORM models (unexported, internal to persistence) ───────────────

type invoiceModel struct {
	ID           string     `gorm:"primaryKey;type:text"`
	CustomerID   string     `gorm:"column:customer_id;type:text;not null"`
	CustomerName string     `gorm:"column:customer_name;type:text;not null;default:''"`
	Code         string     `gorm:"uniqueIndex;type:text;not null"`
	Status       string     `gorm:"type:text;not null;default:'draft'"`
	IssueDate    time.Time  `gorm:"column:issue_date"`
	DueDate      time.Time  `gorm:"column:due_date"`
	TotalAmount  int64      `gorm:"column:total_amount;not null;default:0"`
	Currency     string     `gorm:"type:text;not null;default:'EUR'"`
	Notes        string     `gorm:"type:text;not null;default:''"`
	PaidAt       *time.Time `gorm:"column:paid_at"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at"`

	LineItems []lineItemModel `gorm:"foreignKey:InvoiceID;references:ID"`
}

func (invoiceModel) TableName() string { return "invoices" }

type lineItemModel struct {
	ID          string `gorm:"primaryKey;type:text"`
	InvoiceID   string `gorm:"column:invoice_id;type:text;not null"`
	ProductID   string `gorm:"column:product_id;type:text;not null;default:''"`
	ProductName string `gorm:"column:product_name;type:text;not null;default:''"`
	Description string `gorm:"type:text;not null;default:''"`
	Quantity    int    `gorm:"not null;default:1"`
	UnitPrice   int64  `gorm:"column:unit_price;not null;default:0"`
	TotalPrice  int64  `gorm:"column:total_price;not null;default:0"`
}

func (lineItemModel) TableName() string { return "invoice_line_items" }

// ── Repository ──────────────────────────────────────────────────────

// InvoiceRepository implements domain.InvoiceRepository using GORM + SQLite.
type InvoiceRepository struct {
	db *gormsqlite.DB
}

func NewInvoiceRepository(db *gormsqlite.DB) *InvoiceRepository {
	return &InvoiceRepository{db: db}
}

func (r *InvoiceRepository) Save(ctx context.Context, inv *domain.Invoice) error {
	model := toModel(inv)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		// Delete old line items, then upsert the invoice with new line items.
		if err := tx.Where("invoice_id = ?", model.ID).Delete(&lineItemModel{}).Error; err != nil {
			return fmt.Errorf("delete old line items: %w", err)
		}
		// Omit LineItems so GORM doesn't try to upsert associations itself.
		if err := tx.Omit("LineItems").Save(&model).Error; err != nil {
			return fmt.Errorf("save invoice: %w", err)
		}
		for i := range model.LineItems {
			model.LineItems[i].InvoiceID = model.ID
			if err := tx.Create(&model.LineItems[i]).Error; err != nil {
				return fmt.Errorf("create line item: %w", err)
			}
		}
		return nil
	})
}

func (r *InvoiceRepository) FindByID(ctx context.Context, id string) (*domain.Invoice, error) {
	var model invoiceModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Preload("LineItems").Where("id = ?", id).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrInvoiceNotFound
		}
		return nil, err
	}
	return toDomain(&model), nil
}

func (r *InvoiceRepository) ListByCustomer(ctx context.Context, customerID string) ([]*domain.Invoice, error) {
	var models []invoiceModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Preload("LineItems").
			Where("customer_id = ?", customerID).
			Order("issue_date DESC").
			Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Invoice, len(models))
	for i := range models {
		result[i] = toDomain(&models[i])
	}
	return result, nil
}

func (r *InvoiceRepository) ListAll(ctx context.Context) ([]*domain.Invoice, error) {
	var models []invoiceModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Preload("LineItems").
			Order("created_at DESC").
			Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Invoice, len(models))
	for i := range models {
		result[i] = toDomain(&models[i])
	}
	return result, nil
}

func (r *InvoiceRepository) NextCode(ctx context.Context) (string, error) {
	var code string
	err := r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		// Upsert: insert if missing, then increment and return.
		if err := tx.Exec(
			"INSERT INTO invoice_code_sequences (prefix, last_number) VALUES ('INV', 0) ON CONFLICT(prefix) DO NOTHING",
		).Error; err != nil {
			return fmt.Errorf("ensure sequence row: %w", err)
		}
		var seq struct {
			Prefix     string
			LastNumber int
		}
		if err := tx.Raw(
			"UPDATE invoice_code_sequences SET last_number = last_number + 1 WHERE prefix = 'INV' RETURNING prefix, last_number",
		).Scan(&seq).Error; err != nil {
			return fmt.Errorf("next code: %w", err)
		}
		code = fmt.Sprintf("%s-%05d", seq.Prefix, seq.LastNumber)
		return nil
	})
	return code, err
}

// ── Conversion helpers ──────────────────────────────────────────────

func toModel(inv *domain.Invoice) *invoiceModel {
	items := make([]lineItemModel, len(inv.LineItems))
	for i, li := range inv.LineItems {
		items[i] = lineItemModel{
			ID:          li.ID,
			InvoiceID:   inv.ID,
			ProductID:   li.ProductID,
			ProductName: li.ProductName,
			Description: li.Description,
			Quantity:    li.Quantity,
			UnitPrice:   li.UnitPrice,
			TotalPrice:  li.TotalPrice,
		}
	}
	return &invoiceModel{
		ID:           inv.ID,
		CustomerID:   inv.CustomerID,
		CustomerName: inv.CustomerName,
		Code:         inv.Code,
		Status:       string(inv.Status),
		IssueDate:    inv.IssueDate,
		DueDate:      inv.DueDate,
		TotalAmount:  inv.TotalAmount,
		Currency:     inv.Currency,
		Notes:        inv.Notes,
		PaidAt:       inv.PaidAt,
		CreatedAt:    inv.CreatedAt,
		UpdatedAt:    inv.UpdatedAt,
		LineItems:    items,
	}
}

func toDomain(m *invoiceModel) *domain.Invoice {
	items := make([]domain.LineItem, len(m.LineItems))
	for i, li := range m.LineItems {
		items[i] = domain.LineItem{
			ID:          li.ID,
			ProductID:   li.ProductID,
			ProductName: li.ProductName,
			Description: li.Description,
			Quantity:    li.Quantity,
			UnitPrice:   li.UnitPrice,
			TotalPrice:  li.TotalPrice,
		}
	}
	return &domain.Invoice{
		ID:           m.ID,
		CustomerID:   m.CustomerID,
		CustomerName: m.CustomerName,
		Code:         m.Code,
		Status:       domain.InvoiceStatus(m.Status),
		IssueDate:    m.IssueDate,
		DueDate:      m.DueDate,
		LineItems:    items,
		TotalAmount:  m.TotalAmount,
		Currency:     m.Currency,
		Notes:        m.Notes,
		PaidAt:       m.PaidAt,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}
