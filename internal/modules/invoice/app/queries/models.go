package queries

import "time"

// invoiceRow is the GORM model for the invoices table (query-side only).
type invoiceRow struct {
	ID           string          `gorm:"primaryKey;column:id"`
	CustomerID   string          `gorm:"column:customer_id"`
	CustomerName string          `gorm:"column:customer_name"`
	CustomerCode string          `gorm:"column:customer_code"`
	Code         string          `gorm:"column:code"`
	Status       string          `gorm:"column:status"`
	IssueDate    time.Time       `gorm:"column:issue_date"`
	DueDate      time.Time       `gorm:"column:due_date"`
	SubTotal     int64           `gorm:"column:sub_total"`
	VATTotal     int64           `gorm:"column:vat_total"`
	TotalAmount  int64           `gorm:"column:total_amount"`
	Currency     string          `gorm:"column:currency"`
	Notes        string          `gorm:"column:notes"`
	PaidAt       *time.Time      `gorm:"column:paid_at"`
	LineItems    []lineItemRow   `gorm:"foreignKey:InvoiceID"`
	CreatedAt    time.Time       `gorm:"column:created_at"`
	UpdatedAt    time.Time       `gorm:"column:updated_at"`
}

func (invoiceRow) TableName() string { return "invoices" }

// lineItemRow is the GORM model for the invoice_line_items table (query-side only).
type lineItemRow struct {
	ID             string `gorm:"primaryKey;column:id"`
	InvoiceID      string `gorm:"column:invoice_id"`
	ProductID      string `gorm:"column:product_id"`
	ProductName    string `gorm:"column:product_name"`
	Description    string `gorm:"column:description"`
	Quantity       int    `gorm:"column:quantity"`
	UnitPriceGross int64  `gorm:"column:unit_price_gross"`
	UnitPrice      int64  `gorm:"column:unit_price"`
	VATRate        int    `gorm:"column:vat_rate"`
	TotalPrice     int64  `gorm:"column:total_price"`
	TotalVAT       int64  `gorm:"column:total_vat"`
	TotalGross     int64  `gorm:"column:total_gross"`
}

func (lineItemRow) TableName() string { return "invoice_line_items" }

// toReadModel maps a GORM invoice row to the read model DTO.
func (r *invoiceRow) toReadModel() InvoiceReadModel {
	items := make([]LineItemReadModel, len(r.LineItems))
	for i, li := range r.LineItems {
		items[i] = LineItemReadModel{
			ID:             li.ID,
			ProductID:      li.ProductID,
			ProductName:    li.ProductName,
			Description:    li.Description,
			Quantity:       li.Quantity,
			UnitPriceGross: li.UnitPriceGross,
			UnitPrice:      li.UnitPrice,
			VATRate:        li.VATRate,
			TotalPrice:     li.TotalPrice,
			TotalVAT:       li.TotalVAT,
			TotalGross:     li.TotalGross,
		}
	}
	return InvoiceReadModel{
		ID:           r.ID,
		CustomerID:   r.CustomerID,
		CustomerName: r.CustomerName,
		CustomerCode: r.CustomerCode,
		Code:         r.Code,
		Status:       r.Status,
		IssueDate:    r.IssueDate,
		DueDate:      r.DueDate,
		SubTotal:     r.SubTotal,
		VATTotal:     r.VATTotal,
		TotalAmount:  r.TotalAmount,
		Currency:     r.Currency,
		Notes:        r.Notes,
		PaidAt:       r.PaidAt,
		LineItems:    items,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}
