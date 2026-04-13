package importers

import (
	"context"
	"strings"
	"testing"
)

func TestSepaCSVImporter_Parse(t *testing.T) {
	csvData := `Date;Amount;Reference;PayerName;PayerIBAN
2025-06-15;150.00;INV-001;Acme Corp;DE89370400440532013000
2025-06-16;75.50;INV-002;Beta LLC;DE89370400440532013001`

	importer := NewSepaCSVImporter()
	payments, err := importer.Parse(context.Background(), strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(payments) != 2 {
		t.Fatalf("expected 2 payments, got %d", len(payments))
	}

	// First payment
	if payments[0].Amount.Amount != 15000 {
		t.Errorf("expected amount 15000, got %d", payments[0].Amount.Amount)
	}
	if payments[0].Reference != "INV-001" {
		t.Errorf("expected reference INV-001, got %s", payments[0].Reference)
	}
	if payments[0].PayerName != "Acme Corp" {
		t.Errorf("expected payer Acme Corp, got %s", payments[0].PayerName)
	}
	if payments[0].PayerIBAN != "DE89370400440532013000" {
		t.Errorf("expected IBAN DE89370400440532013000, got %s", payments[0].PayerIBAN)
	}

	// Second payment
	if payments[1].Amount.Amount != 7550 {
		t.Errorf("expected amount 7550, got %d", payments[1].Amount.Amount)
	}
	if payments[1].Reference != "INV-002" {
		t.Errorf("expected reference INV-002, got %s", payments[1].Reference)
	}

	// All should share the same batch ID
	if payments[0].ImportBatchID == "" {
		t.Error("expected non-empty batch ID")
	}
	if payments[0].ImportBatchID != payments[1].ImportBatchID {
		t.Error("expected same batch ID for all payments in import")
	}
}

func TestSepaCSVImporter_EmptyFile(t *testing.T) {
	csvData := `Date;Amount;Reference;PayerName;PayerIBAN`

	importer := NewSepaCSVImporter()
	payments, err := importer.Parse(context.Background(), strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(payments) != 0 {
		t.Errorf("expected 0 payments for empty file, got %d", len(payments))
	}
}

func TestSepaCSVImporter_MalformedAmount(t *testing.T) {
	csvData := `Date;Amount;Reference;PayerName;PayerIBAN
2025-06-15;abc;INV-001;Acme Corp;DE89370400440532013000`

	importer := NewSepaCSVImporter()
	_, err := importer.Parse(context.Background(), strings.NewReader(csvData))
	if err == nil {
		t.Fatal("expected error for malformed amount")
	}
}

func TestSepaCSVImporter_MalformedDate(t *testing.T) {
	csvData := `Date;Amount;Reference;PayerName;PayerIBAN
15/06/2025;150.00;INV-001;Acme Corp;DE89370400440532013000`

	importer := NewSepaCSVImporter()
	_, err := importer.Parse(context.Background(), strings.NewReader(csvData))
	if err == nil {
		t.Fatal("expected error for malformed date")
	}
}

func TestSepaCSVImporter_Format(t *testing.T) {
	importer := NewSepaCSVImporter()
	if importer.Format() != "sepa_csv" {
		t.Errorf("expected format sepa_csv, got %s", importer.Format())
	}
}
