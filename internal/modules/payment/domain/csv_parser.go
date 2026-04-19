package domain

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"
)


var dateFormats = []string{"2006-01-02", "02.01.2006", "2006/01/02", "01/02/2006"}

// ParseCSV parses a bank CSV export into PaymentEntry slice.
// Supports semicolon and comma delimiters. Skips debits (amount <= 0).
func ParseCSV(data []byte) ([]PaymentEntry, error) {
	// Detect delimiter: count semicolons vs commas in first line
	first := string(bytes.SplitN(data, []byte("\n"), 2)[0])
	delim := ','
	if strings.Count(first, ";") > strings.Count(first, ",") {
		delim = ';'
	}

	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = rune(delim)
	r.LazyQuotes = true
	r.TrimLeadingSpace = true

	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse csv: %w", err)
	}
	if len(records) == 0 {
		return nil, nil
	}

	// Detect header row and column indices
	header := records[0]
	cols := detectColumns(header)

	// If no recognized header, treat first row as data with positional fallback
	startRow := 1
	if !cols.hasAny() {
		cols = positionalColumns(len(header))
		startRow = 0
	}

	var entries []PaymentEntry
	for _, row := range records[startRow:] {
		e, ok := parseRow(row, cols)
		if !ok || e.Amount <= 0 {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

type columnMap struct {
	date, payer, iban, amount, currency, reference, description int
}

func (c columnMap) hasAny() bool {
	return c.date >= 0 || c.amount >= 0
}

func detectColumns(header []string) columnMap {
	m := columnMap{-1, -1, -1, -1, -1, -1, -1}
	for i, h := range header {
		h = strings.ToLower(strings.Trim(h, `" `))
		switch {
		case contains(h, "date", "data", "datum", "data"):
			m.date = i
		case contains(h, "beneficiary", "counterparty", "payer", "mokėtojas", "gavėjas"):
			m.payer = i
		case contains(h, "iban", "account", "sąskaita", "saskaita"):
			m.iban = i
		case contains(h, "credit", "kreditas", "amount", "suma"):
			m.amount = i
		case contains(h, "currency", "valiuta"):
			m.currency = i
		case contains(h, "reference", "payment id", "mokėjimo kodas", "numeris", "ref"):
			m.reference = i
		case contains(h, "description", "details", "paskirtis", "pastabos", "detales"):
			m.description = i
		}
	}
	return m
}

// positionalColumns: Date, Payer, IBAN, Amount, Currency, Reference, Description
func positionalColumns(n int) columnMap {
	get := func(i int) int {
		if i < n {
			return i
		}
		return -1
	}
	return columnMap{get(0), get(1), get(2), get(3), get(4), get(5), get(6)}
}

func parseRow(row []string, cols columnMap) (PaymentEntry, bool) {
	get := func(i int) string {
		if i >= 0 && i < len(row) {
			return strings.Trim(row[i], `" `)
		}
		return ""
	}

	amtStr := get(cols.amount)
	if amtStr == "" {
		return PaymentEntry{}, false
	}
	amount, err := parseAmount(amtStr)
	if err != nil {
		return PaymentEntry{}, false
	}

	var t time.Time
	dateStr := get(cols.date)
	for _, fmt := range dateFormats {
		if parsed, err := time.Parse(fmt, dateStr); err == nil {
			t = parsed
			break
		}
	}

	return PaymentEntry{
		Date:        t,
		PayerName:   get(cols.payer),
		PayerIBAN:   get(cols.iban),
		Amount:      amount,
		Currency:    get(cols.currency),
		Reference:   get(cols.reference),
		Description: get(cols.description),
	}, true
}

// parseAmount parses amount strings into integer cents without float math.
// Handles "100.00", "100,00", "1.234,56" (EU), "1,234.56" (US), "1234" (no decimal).
func parseAmount(s string) (int64, error) {
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return 0, fmt.Errorf("empty amount")
	}

	// Determine decimal separator: whichever of '.' or ',' appears last.
	lastDot := strings.LastIndex(s, ".")
	lastComma := strings.LastIndex(s, ",")

	var intPart, fracPart string
	switch {
	case lastDot > lastComma:
		// dot is decimal: "1,234.56" or "1234.56"
		intPart = strings.ReplaceAll(s[:lastDot], ",", "")
		fracPart = s[lastDot+1:]
	case lastComma > lastDot:
		// comma is decimal: "1.234,56" or "1234,56"
		intPart = strings.ReplaceAll(s[:lastComma], ".", "")
		fracPart = s[lastComma+1:]
	default:
		// no separator — whole euros
		intPart = s
		fracPart = ""
	}

	if intPart == "" {
		intPart = "0"
	}
	euros, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", s, err)
	}

	var cents int64
	switch len(fracPart) {
	case 0:
		cents = 0
	case 1:
		d, err := strconv.ParseInt(fracPart, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid amount fraction %q: %w", s, err)
		}
		cents = d * 10
	default:
		// 2+ decimal places: use first two digits only
		d, err := strconv.ParseInt(fracPart[:2], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid amount fraction %q: %w", s, err)
		}
		cents = d
	}

	return euros*100 + cents, nil
}

func contains(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
