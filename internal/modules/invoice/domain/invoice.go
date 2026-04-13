package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/shared/domain"
)

var (
	ErrInvoiceNumberRequired = errors.New("invoice number is required")
	ErrCustomerIDRequired    = errors.New("customer ID is required")
	ErrCustomerNameRequired  = errors.New("customer name is required")
	ErrInvalidDueDate        = errors.New("due date must be after issue date")
	ErrInvoiceNotDraft       = errors.New("invoice is not in draft status")
	ErrCannotMarkPaid        = errors.New("invoice must be finalized or sent to mark as paid")
	ErrCannotVoid            = errors.New("invoice can only be voided from draft or finalized status")
	ErrLineNotFound          = errors.New("invoice line not found")
	ErrInvoiceNotFound       = errors.New("invoice not found")
	ErrInvalidQuantity       = errors.New("quantity must be greater than zero")
)

type InvoiceStatus string

const (
	StatusDraft     InvoiceStatus = "draft"
	StatusFinalized InvoiceStatus = "finalized"
	StatusSent      InvoiceStatus = "sent"
	StatusPaid      InvoiceStatus = "paid"
	StatusOverdue   InvoiceStatus = "overdue"
	StatusVoid      InvoiceStatus = "void"
)

type InvoiceLine struct {
	ID          string
	ProductID   string
	ProductName string
	Description string
	Quantity    int
	UnitPrice   domain.Money
	Total       domain.Money
}

type Invoice struct {
	ID            string
	InvoiceNumber string
	CustomerID    string
	CustomerName  string
	Lines         []InvoiceLine
	Subtotal      domain.Money
	TaxRate       int
	TaxAmount     domain.Money
	Total         domain.Money
	Status        InvoiceStatus
	IssueDate     time.Time
	DueDate       time.Time
	PaidDate      *time.Time
	RecurringID   *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NewInvoice(number, customerID, customerName string, issueDate, dueDate time.Time) (*Invoice, error) {
	number = strings.TrimSpace(number)
	if number == "" {
		return nil, ErrInvoiceNumberRequired
	}

	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return nil, ErrCustomerIDRequired
	}

	customerName = strings.TrimSpace(customerName)
	if customerName == "" {
		return nil, ErrCustomerNameRequired
	}

	if dueDate.Before(issueDate) {
		return nil, ErrInvalidDueDate
	}

	now := time.Now().UTC()
	return &Invoice{
		ID:            uuid.Must(uuid.NewV7()).String(),
		InvoiceNumber: number,
		CustomerID:    customerID,
		CustomerName:  customerName,
		Lines:         []InvoiceLine{},
		Subtotal:      domain.EUR(0),
		TaxRate:       21,
		TaxAmount:     domain.EUR(0),
		Total:         domain.EUR(0),
		Status:        StatusDraft,
		IssueDate:     issueDate,
		DueDate:       dueDate,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (inv *Invoice) AddLine(productID, productName, description string, quantity int, unitPrice domain.Money) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	line := InvoiceLine{
		ID:          uuid.Must(uuid.NewV7()).String(),
		ProductID:   strings.TrimSpace(productID),
		ProductName: strings.TrimSpace(productName),
		Description: strings.TrimSpace(description),
		Quantity:    quantity,
		UnitPrice:   unitPrice,
		Total:       unitPrice.Multiply(int64(quantity)),
	}

	inv.Lines = append(inv.Lines, line)
	inv.Recalculate()
	return nil
}

func (inv *Invoice) RemoveLine(lineID string) error {
	idx := -1
	for i, l := range inv.Lines {
		if l.ID == lineID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ErrLineNotFound
	}

	inv.Lines = append(inv.Lines[:idx], inv.Lines[idx+1:]...)
	inv.Recalculate()
	return nil
}

func (inv *Invoice) Recalculate() {
	subtotal := domain.EUR(0)
	for _, line := range inv.Lines {
		subtotal, _ = subtotal.Add(line.Total)
	}
	inv.Subtotal = subtotal
	inv.TaxAmount = domain.EUR(subtotal.Amount * int64(inv.TaxRate) / 100)
	total, _ := subtotal.Add(inv.TaxAmount)
	inv.Total = total
	inv.UpdatedAt = time.Now().UTC()
}

func (inv *Invoice) Finalize() error {
	if inv.Status != StatusDraft {
		return ErrInvoiceNotDraft
	}
	inv.Status = StatusFinalized
	inv.UpdatedAt = time.Now().UTC()
	return nil
}

func (inv *Invoice) MarkPaid(paidDate time.Time) error {
	if inv.Status != StatusFinalized && inv.Status != StatusSent {
		return ErrCannotMarkPaid
	}
	inv.Status = StatusPaid
	inv.PaidDate = &paidDate
	inv.UpdatedAt = time.Now().UTC()
	return nil
}

func (inv *Invoice) Void() error {
	if inv.Status != StatusDraft && inv.Status != StatusFinalized {
		return ErrCannotVoid
	}
	inv.Status = StatusVoid
	inv.UpdatedAt = time.Now().UTC()
	return nil
}
