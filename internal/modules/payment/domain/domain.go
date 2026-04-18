package domain

import "time"

// Confidence represents how well a CSV row matched an invoice.
type Confidence string

const (
	ConfidenceExact          Confidence = "exact"
	ConfidenceAmountMismatch Confidence = "amount_mismatch"
	ConfidenceUnmatched      Confidence = "unmatched"
)

// CSVRow holds the raw fields parsed from a SEPA CSV line.
type CSVRow struct {
	Date      string
	Amount    string // e.g. "150.00"
	Reference string
	Payer     string
	IBAN      string
}

// MatchResult is the result of trying to match one CSV row to an invoice.
type MatchResult struct {
	Row            CSVRow
	InvoiceID      string
	InvoiceNumber  string
	InvoiceStatus  string // "draft", "finalized", "paid", "void"
	Confidence     Confidence
	BookingDate    time.Time
}
