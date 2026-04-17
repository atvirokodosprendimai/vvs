package queries

import "time"

// InvoiceReadModel is the flattened read model for the invoice list/detail view.
type InvoiceReadModel struct {
	ID           string
	CustomerID   string
	CustomerName string
	Code         string
	Status       string
	IssueDate    time.Time
	DueDate      time.Time
	TotalAmount  int64
	Currency     string
	Notes        string
	PaidAt       *time.Time
	LineItems    []LineItemReadModel
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// LineItemReadModel is the flattened read model for an invoice line item.
type LineItemReadModel struct {
	ID          string
	ProductID   string
	ProductName string
	Description string
	Quantity    int
	UnitPrice   int64
	TotalPrice  int64
}
