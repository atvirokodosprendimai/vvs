package domain

import (
	"errors"
	"math"
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
	ID             string
	ProductID      string
	ProductName    string
	Description    string
	Quantity       int
	UnitPriceGross int64 // cents, price entered by user (includes VAT)
	UnitPrice      int64 // cents, net price (calculated: gross * 100 / (100 + VATRate))
	VATRate        int   // percentage: 0, 5, 9, 21
	TotalPrice     int64 // net total = qty * UnitPrice
	TotalVAT       int64 // VAT amount for line
	TotalGross     int64 // gross total = qty * UnitPriceGross
}

// Invoice is the aggregate root for the invoicing bounded context.
type Invoice struct {
	ID           string
	CustomerID   string
	CustomerName string
	CustomerCode string        // cloned at creation, immutable snapshot
	Code         string        // auto-generated: INV-001, INV-002...
	Status       InvoiceStatus // draft, finalized, paid, void
	IssueDate    time.Time
	DueDate      time.Time
	LineItems    []LineItem
	SubTotal     int64  // cents, sum of net line totals
	VATTotal     int64  // cents, sum of VAT amounts
	TotalAmount  int64  // cents, grand total (SubTotal + VATTotal)
	Currency     string // default "EUR"
	Notes        string
	PaidAt          *time.Time
	ReminderSentAt  *time.Time // set when dunning reminder was last sent
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewInvoice creates a new invoice in draft status.
func NewInvoice(id, customerID, customerName, customerCode, code string) *Invoice {
	now := time.Now().UTC()
	return &Invoice{
		ID:           id,
		CustomerID:   customerID,
		CustomerName: customerName,
		CustomerCode: customerCode,
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

// UpdateLineItem updates a line item's fields by ID. Only allowed in draft status.
func (inv *Invoice) UpdateLineItem(itemID, productName, description string, quantity int, unitPriceGross int64, vatRate int) error {
	if inv.Status != StatusDraft {
		return ErrInvoiceNotDraft
	}
	for i, li := range inv.LineItems {
		if li.ID == itemID {
			inv.LineItems[i].ProductName = productName
			inv.LineItems[i].Description = description
			inv.LineItems[i].Quantity = quantity
			inv.LineItems[i].UnitPriceGross = unitPriceGross
			inv.LineItems[i].VATRate = vatRate
			inv.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return ErrLineItemNotFound
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

// Recalculate recomputes each line item's prices (net from gross + VAT) and invoice totals.
func (inv *Invoice) Recalculate() {
	var subTotal, vatTotal, grandTotal int64
	for i := range inv.LineItems {
		li := &inv.LineItems[i]
		li.TotalGross = int64(li.Quantity) * li.UnitPriceGross
		li.UnitPrice = netFromGross(li.UnitPriceGross, li.VATRate)
		li.TotalPrice = int64(li.Quantity) * li.UnitPrice
		li.TotalVAT = li.TotalGross - li.TotalPrice
		subTotal += li.TotalPrice
		vatTotal += li.TotalVAT
		grandTotal += li.TotalGross
	}
	inv.SubTotal = subTotal
	inv.VATTotal = vatTotal
	inv.TotalAmount = grandTotal
	inv.UpdatedAt = time.Now().UTC()
}

// netFromGross calculates the net price from a gross price and VAT percentage.
// Example: gross=1210, vatRate=21 → net=1000
func netFromGross(grossCents int64, vatRate int) int64 {
	if vatRate <= 0 {
		return grossCents
	}
	return int64(math.Round(float64(grossCents) * 100 / float64(100+vatRate)))
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

// ReminderSentAt is set when a dunning reminder email is sent for this invoice.
// Added to Invoice struct — see field definition in struct body.

// MarkReminderSent records the current time as the last reminder sent.
// Returns ErrInvalidTransition if the invoice is not finalized and overdue.
func (inv *Invoice) MarkReminderSent() error {
	if !inv.IsOverdue() {
		return ErrInvalidTransition
	}
	now := time.Now().UTC()
	inv.ReminderSentAt = &now
	inv.UpdatedAt = now
	return nil
}

// NeedsReminder returns true if the invoice is overdue and either no reminder
// has been sent, or the last reminder was sent more than minInterval ago.
func (inv *Invoice) NeedsReminder(minInterval time.Duration) bool {
	if !inv.IsOverdue() {
		return false
	}
	if inv.ReminderSentAt == nil {
		return true
	}
	return time.Since(*inv.ReminderSentAt) >= minInterval
}
