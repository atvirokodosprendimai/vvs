package domain

import "time"

// PaymentEntry is a single row parsed from the bank CSV export.
type PaymentEntry struct {
	Date        time.Time
	PayerName   string
	PayerIBAN   string
	Amount      int64  // cents, always positive (credits only)
	Currency    string
	Reference   string // raw reference field from bank
	Description string // raw description/details field
}

// MatchConfidence indicates how well a PaymentEntry matched an invoice.
type MatchConfidence string

const (
	ConfidenceExact          MatchConfidence = "exact"
	ConfidenceAmountMismatch MatchConfidence = "amount_mismatch"
	ConfidenceUnmatched      MatchConfidence = "unmatched"
)

// InvoiceRef is the minimal invoice data needed for matching (no import cycle).
type InvoiceRef struct {
	ID           string
	Code         string // INV-001
	CustomerCode string
	Amount       int64  // cents
	Status       string // "finalized", "paid", etc.
}

// MatchResult pairs a PaymentEntry with its best matching invoice (or nil).
type MatchResult struct {
	Entry      PaymentEntry
	Invoice    *InvoiceRef
	Confidence MatchConfidence
}
