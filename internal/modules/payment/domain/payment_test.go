package domain

import (
	"testing"
	"time"
)

// ── ParseCSV ────────────────────────────────────────────────────────

func TestParseCSV_SemicolonFormat(t *testing.T) {
	csv := []byte(`Date;Beneficiary;IBAN;Amount;Currency;Reference;Description
2026-04-01;UAB Klientas;LT123456789;100.00;EUR;INV-001;Invoice payment
2026-04-02;UAB Kitas;LT987654321;50.50;EUR;INV-002;Payment for services
`)
	entries, err := ParseCSV(csv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	if entries[0].Amount != 10000 {
		t.Errorf("want 10000 cents, got %d", entries[0].Amount)
	}
	if entries[0].Reference != "INV-001" {
		t.Errorf("want INV-001, got %s", entries[0].Reference)
	}
	if entries[1].Amount != 5050 {
		t.Errorf("want 5050 cents, got %d", entries[1].Amount)
	}
}

func TestParseCSV_CommaFormat(t *testing.T) {
	csv := []byte(`Date,Beneficiary,IBAN,Amount,Currency,Reference,Description
2026-04-01,UAB Klientas,LT123456789,200.00,EUR,INV-003,Payment
`)
	entries, err := ParseCSV(csv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Amount != 20000 {
		t.Errorf("want 20000, got %d", entries[0].Amount)
	}
}

func TestParseCSV_SkipsDebits(t *testing.T) {
	csv := []byte(`Date;Beneficiary;IBAN;Amount;Currency;Reference;Description
2026-04-01;Bank Fee;;-5.00;EUR;;Monthly fee
2026-04-02;UAB Klientas;LT123;100.00;EUR;INV-005;Payment
`)
	entries, err := ParseCSV(csv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry (debit skipped), got %d", len(entries))
	}
}

func TestParseCSV_CommaDecimalAmount(t *testing.T) {
	csv := []byte(`Date;Beneficiary;IBAN;Amount;Currency;Reference;Description
2026-04-01;UAB Klientas;LT123;99,99;EUR;INV-010;Payment
`)
	entries, err := ParseCSV(csv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Amount != 9999 {
		t.Errorf("want 9999, got %d", entries[0].Amount)
	}
}

func TestParseCSV_EUThousandSeparator(t *testing.T) {
	// "1.234,56" — dot = thousand separator, comma = decimal (European format)
	csv := []byte(`Date;Beneficiary;IBAN;Amount;Currency;Reference;Description
2026-04-01;UAB Klientas;LT123;1.234,56;EUR;INV-011;Payment
`)
	entries, err := ParseCSV(csv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Amount != 123456 {
		t.Errorf("want 123456 cents (1234.56 EUR), got %d", entries[0].Amount)
	}
}

func TestParseCSV_DateFormats(t *testing.T) {
	tests := []struct {
		date string
		want time.Time
	}{
		{"2026-04-01", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)},
		{"01.04.2026", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)},
		{"2026/04/01", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		csv := []byte("Date;Beneficiary;IBAN;Amount;Currency;Reference;Description\n" +
			tt.date + ";UAB;LT123;10.00;EUR;INV-020;pay\n")
		entries, err := ParseCSV(csv)
		if err != nil {
			t.Fatalf("date %s: unexpected error: %v", tt.date, err)
		}
		if len(entries) != 1 {
			t.Fatalf("date %s: want 1 entry, got %d", tt.date, len(entries))
		}
		if !entries[0].Date.Equal(tt.want) {
			t.Errorf("date %s: want %v, got %v", tt.date, tt.want, entries[0].Date)
		}
	}
}

func TestParseCSV_Empty(t *testing.T) {
	entries, err := ParseCSV([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("want 0 entries, got %d", len(entries))
	}
}

// ── ExtractInvoiceCode ───────────────────────────────────────────────

func TestExtractInvoiceCode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"INV-001 payment", "INV-001"},
		{"Payment for inv-023", "INV-023"},
		{"ref: INV-999 services", "INV-999"},
		{"no invoice here", ""},
		{"", ""},
		{"INV-001 and INV-002", "INV-001"}, // first match
	}
	for _, tt := range tests {
		got := ExtractInvoiceCode(tt.input)
		if got != tt.want {
			t.Errorf("ExtractInvoiceCode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ── MatchPayments ────────────────────────────────────────────────────

func TestMatchPayments_ExactMatch(t *testing.T) {
	entries := []PaymentEntry{
		{Amount: 10000, Reference: "INV-001"},
	}
	invoices := []InvoiceRef{
		{ID: "id1", Code: "INV-001", Amount: 10000, Status: "finalized"},
	}
	results := MatchPayments(entries, invoices)
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if results[0].Confidence != ConfidenceExact {
		t.Errorf("want exact, got %s", results[0].Confidence)
	}
	if results[0].Invoice == nil || results[0].Invoice.ID != "id1" {
		t.Errorf("wrong invoice matched")
	}
}

func TestMatchPayments_AmountMismatch(t *testing.T) {
	entries := []PaymentEntry{
		{Amount: 9000, Reference: "INV-002"},
	}
	invoices := []InvoiceRef{
		{ID: "id2", Code: "INV-002", Amount: 10000, Status: "finalized"},
	}
	results := MatchPayments(entries, invoices)
	if results[0].Confidence != ConfidenceAmountMismatch {
		t.Errorf("want amount_mismatch, got %s", results[0].Confidence)
	}
	if results[0].Invoice == nil {
		t.Error("invoice should still be set on mismatch")
	}
}

func TestMatchPayments_Unmatched(t *testing.T) {
	entries := []PaymentEntry{
		{Amount: 10000, Reference: "no code here"},
	}
	invoices := []InvoiceRef{
		{ID: "id3", Code: "INV-003", Amount: 10000, Status: "finalized"},
	}
	results := MatchPayments(entries, invoices)
	if results[0].Confidence != ConfidenceUnmatched {
		t.Errorf("want unmatched, got %s", results[0].Confidence)
	}
	if results[0].Invoice != nil {
		t.Error("invoice should be nil on unmatched")
	}
}

func TestMatchPayments_CodeInDescription(t *testing.T) {
	entries := []PaymentEntry{
		{Amount: 5000, Reference: "bank transfer", Description: "INV-004 services"},
	}
	invoices := []InvoiceRef{
		{ID: "id4", Code: "INV-004", Amount: 5000, Status: "finalized"},
	}
	results := MatchPayments(entries, invoices)
	if results[0].Confidence != ConfidenceExact {
		t.Errorf("want exact via description, got %s", results[0].Confidence)
	}
}

func TestMatchPayments_AmountTolerance(t *testing.T) {
	// ±1 cent should still be exact
	entries := []PaymentEntry{
		{Amount: 9999, Reference: "INV-005"},
	}
	invoices := []InvoiceRef{
		{ID: "id5", Code: "INV-005", Amount: 10000, Status: "finalized"},
	}
	results := MatchPayments(entries, invoices)
	if results[0].Confidence != ConfidenceExact {
		t.Errorf("want exact within 1 cent tolerance, got %s", results[0].Confidence)
	}
}

func TestMatchPayments_MultipleEntries(t *testing.T) {
	entries := []PaymentEntry{
		{Amount: 10000, Reference: "INV-001"},
		{Amount: 20000, Reference: "INV-002"},
		{Amount: 30000, Reference: "no match"},
	}
	invoices := []InvoiceRef{
		{ID: "id1", Code: "INV-001", Amount: 10000, Status: "finalized"},
		{ID: "id2", Code: "INV-002", Amount: 25000, Status: "finalized"},
	}
	results := MatchPayments(entries, invoices)
	if len(results) != 3 {
		t.Fatalf("want 3 results, got %d", len(results))
	}
	if results[0].Confidence != ConfidenceExact {
		t.Errorf("[0] want exact, got %s", results[0].Confidence)
	}
	if results[1].Confidence != ConfidenceAmountMismatch {
		t.Errorf("[1] want amount_mismatch, got %s", results[1].Confidence)
	}
	if results[2].Confidence != ConfidenceUnmatched {
		t.Errorf("[2] want unmatched, got %s", results[2].Confidence)
	}
}
