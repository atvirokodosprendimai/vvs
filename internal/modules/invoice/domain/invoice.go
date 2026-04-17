package domain

import (
	"errors"
	"time"
)

var (
	ErrInvoiceNotFound   = errors.New("invoice not found")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrInvoiceNotDraft   = errors.New("invoice is not in draft status")
	ErrNoLineItems       = errors.New("invoice has no line items")
	ErrLineItemNotFound  = errors.New("line item not found")
)

type InvoiceStatus string

const (
	StatusDraft     InvoiceStatus = "draft"
	StatusFinalized InvoiceStatus = "finalized"
	StatusPaid      InvoiceStatus = "paid"
	StatusVoid      InvoiceStatus = "void"
)

// LineItem is a value object representing a single line on an invoice.
type LineItem struct {
	ID          string
	ProductID   string
	ProductName string
	Description string
	Quantity    int
	UnitPrice   int64 // cents
	TotalPrice  int64 // quantity * unit_price
}

// Invoice is the aggregate root for the invoicing bounded context.
type Invoice struct {
	ID           string
	CustomerID   string
	CustomerName string
	Code         string        // auto-generated: INV-001, INV-002...
	Status       InvoiceStatus // draft, finalized, paid, void
	IssueDate    time.Time
	DueDate      time.Time
	LineItems    []LineItem
	TotalAmount  int64  // cents
	Currency     string // default "EUR"
	Notes        string
	PaidAt       *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewInvoice creates a new invoice in draft status.
func NewInvoice(id, customerID, customerName, code string) *Invoice {
	now := time.Now().UTC()
	return &Invoice{
		ID:           id,
		CustomerID:   customerID,
		CustomerName: customerName,
		Code:         code,
		Status:       StatusDraft,
		IssueDate:    now,
		DueDate:      now,
		LineItems:    []LineItem{},
		TotalAmount:  0,
		Currency:     "EUR",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// AddLineItem appends a line item to the invoice. Only allowed in draft status.
func (inv *Invoice) AddLineItem(item LineItem) error {
	if inv.Status != StatusDraft {
		return ErrInvoiceNotDraft
	}
	inv.LineItems = append(inv.LineItems, item)
	inv.UpdatedAt = time.Now().UTC()
	return nil
}

// RemoveLineItem removes a line item by ID. Only allowed in draft status.
func (inv *Invoice) RemoveLineItem(itemID string) error {
	if inv.Status != StatusDraft {
		return ErrInvoiceNotDraft
	}
	for i, li := range inv.LineItems {
		if li.ID == itemID {
			inv.LineItems = append(inv.LineItems[:i], inv.LineItems[i+1:]...)
			inv.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return ErrLineItemNotFound
}

// Recalculate recomputes each line item's TotalPrice and the invoice TotalAmount.
func (inv *Invoice) Recalculate() {
	var total int64
	for i := range inv.LineItems {
		inv.LineItems[i].TotalPrice = int64(inv.LineItems[i].Quantity) * inv.LineItems[i].UnitPrice
		total += inv.LineItems[i].TotalPrice
	}
	inv.TotalAmount = total
	inv.UpdatedAt = time.Now().UTC()
}

// Finalize transitions the invoice from draft to finalized.
// Requires at least one line item.
func (inv *Invoice) Finalize() error {
	if inv.Status != StatusDraft {
		return ErrInvalidTransition
	}
	if len(inv.LineItems) == 0 {
		return ErrNoLineItems
	}
	inv.Recalculate()
	inv.Status = StatusFinalized
	inv.UpdatedAt = time.Now().UTC()
	return nil
}

// MarkPaid transitions the invoice from finalized to paid and records the payment time.
func (inv *Invoice) MarkPaid() error {
	if inv.Status != StatusFinalized {
		return ErrInvalidTransition
	}
	now := time.Now().UTC()
	inv.Status = StatusPaid
	inv.PaidAt = &now
	inv.UpdatedAt = now
	return nil
}

// Void transitions the invoice to void. Allowed from draft or finalized, but not from paid.
func (inv *Invoice) Void() error {
	if inv.Status != StatusDraft && inv.Status != StatusFinalized {
		return ErrInvalidTransition
	}
	inv.Status = StatusVoid
	inv.UpdatedAt = time.Now().UTC()
	return nil
}

// IsOverdue returns true if the invoice is finalized (not paid/void) and past its due date.
func (inv *Invoice) IsOverdue() bool {
	return inv.Status == StatusFinalized && time.Now().After(inv.DueDate)
}
