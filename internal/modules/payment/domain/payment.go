package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/shared/domain"
)

var (
	ErrPaymentNotFound      = errors.New("payment not found")
	ErrReferenceRequired    = errors.New("reference is required")
	ErrAlreadyMatched       = errors.New("payment is already matched")
	ErrNotMatched           = errors.New("payment is not matched")
	ErrInvoiceIDRequired    = errors.New("invoice ID is required for matching")
	ErrCustomerIDRequired   = errors.New("customer ID is required for matching")
)

type PaymentStatus string

const (
	StatusImported        PaymentStatus = "imported"
	StatusMatched         PaymentStatus = "matched"
	StatusUnmatched       PaymentStatus = "unmatched"
	StatusManuallyMatched PaymentStatus = "manually_matched"
)

type Payment struct {
	ID            string
	Amount        domain.Money
	Reference     string
	PayerName     string
	PayerIBAN     string
	BookingDate   time.Time
	InvoiceID     *string
	CustomerID    *string
	Status        PaymentStatus
	ImportBatchID string
	CreatedAt     time.Time
}

func NewPayment(amount domain.Money, reference, payerName, payerIBAN string, bookingDate time.Time, batchID string) *Payment {
	now := time.Now().UTC()
	status := StatusImported
	if strings.TrimSpace(reference) == "" {
		status = StatusUnmatched
	}

	return &Payment{
		ID:            uuid.Must(uuid.NewV7()).String(),
		Amount:        amount,
		Reference:     strings.TrimSpace(reference),
		PayerName:     strings.TrimSpace(payerName),
		PayerIBAN:     strings.TrimSpace(payerIBAN),
		BookingDate:   bookingDate,
		Status:        status,
		ImportBatchID: batchID,
		CreatedAt:     now,
	}
}

func (p *Payment) Match(invoiceID, customerID string) error {
	invoiceID = strings.TrimSpace(invoiceID)
	customerID = strings.TrimSpace(customerID)

	if invoiceID == "" {
		return ErrInvoiceIDRequired
	}
	if customerID == "" {
		return ErrCustomerIDRequired
	}
	if p.Status == StatusMatched || p.Status == StatusManuallyMatched {
		return ErrAlreadyMatched
	}

	p.InvoiceID = &invoiceID
	p.CustomerID = &customerID
	p.Status = StatusManuallyMatched
	return nil
}

func (p *Payment) Unmatch() error {
	if p.Status != StatusMatched && p.Status != StatusManuallyMatched {
		return ErrNotMatched
	}

	p.InvoiceID = nil
	p.CustomerID = nil
	p.Status = StatusUnmatched
	return nil
}
