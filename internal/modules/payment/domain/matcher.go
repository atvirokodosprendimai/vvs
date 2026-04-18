package domain

import (
	"regexp"
	"strings"
)

var invoiceCodeRe = regexp.MustCompile(`(?i)(INV-\d+)`)

// ExtractInvoiceCode extracts the first INV-NNN pattern from s, uppercased.
// Returns empty string if not found.
func ExtractInvoiceCode(s string) string {
	m := invoiceCodeRe.FindString(s)
	if m == "" {
		return ""
	}
	return strings.ToUpper(m)
}

// MatchPayments matches each PaymentEntry against the provided invoices.
// Matching strategy: extract invoice code from Reference, then Description.
// Confidence: exact (code+amount match), amount_mismatch (code match, wrong amount), unmatched.
func MatchPayments(entries []PaymentEntry, invoices []InvoiceRef) []MatchResult {
	// Index invoices by code for O(1) lookup
	byCode := make(map[string]*InvoiceRef, len(invoices))
	for i := range invoices {
		byCode[invoices[i].Code] = &invoices[i]
	}

	results := make([]MatchResult, len(entries))
	for i, e := range entries {
		code := ExtractInvoiceCode(e.Reference)
		if code == "" {
			code = ExtractInvoiceCode(e.Description)
		}

		inv, found := byCode[code]
		if !found || code == "" {
			results[i] = MatchResult{Entry: e, Invoice: nil, Confidence: ConfidenceUnmatched}
			continue
		}

		// Allow ±1 cent tolerance for rounding
		diff := e.Amount - inv.Amount
		if diff < 0 {
			diff = -diff
		}
		if diff <= 1 {
			results[i] = MatchResult{Entry: e, Invoice: inv, Confidence: ConfidenceExact}
		} else {
			results[i] = MatchResult{Entry: e, Invoice: inv, Confidence: ConfidenceAmountMismatch}
		}
	}
	return results
}
