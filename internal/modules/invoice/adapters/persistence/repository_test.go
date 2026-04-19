package persistence_test

import (
	"context"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/invoice/adapters/persistence"
	"github.com/atvirokodosprendimai/vvs/internal/modules/invoice/domain"
	"github.com/atvirokodosprendimai/vvs/internal/modules/invoice/migrations"
	"github.com/atvirokodosprendimai/vvs/internal/testutil"
)

func setupRepo(t *testing.T) *persistence.InvoiceRepository {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_invoice")
	return persistence.NewInvoiceRepository(db)
}

func makeInvoice(id, customerID, customerName, code string) *domain.Invoice {
	now := time.Now().UTC().Truncate(time.Second)
	return &domain.Invoice{
		ID:           id,
		CustomerID:   customerID,
		CustomerName: customerName,
		CustomerCode: "TST-00001",
		Code:         code,
		Status:       domain.StatusDraft,
		IssueDate:    now,
		DueDate:      now.Add(30 * 24 * time.Hour),
		LineItems:    []domain.LineItem{},
		TotalAmount:  0,
		Currency:     "EUR",
		Notes:        "test invoice",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func TestSaveAndFindByID(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	inv := makeInvoice("inv-1", "cust-1", "Acme Corp", "INV-00001")
	inv.LineItems = []domain.LineItem{
		{
			ID:             "li-1",
			ProductID:      "prod-1",
			ProductName:    "Internet 100Mbps",
			Description:    "Monthly subscription",
			Quantity:       1,
			UnitPriceGross: 3629,
			UnitPrice:      2999,
			VATRate:        21,
			TotalPrice:     2999,
			TotalVAT:       630,
			TotalGross:     3629,
		},
		{
			ID:             "li-2",
			ProductID:      "prod-2",
			ProductName:    "Router rental",
			Description:    "Mikrotik hAP",
			Quantity:       1,
			UnitPriceGross: 500,
			UnitPrice:      500,
			VATRate:        0,
			TotalPrice:     500,
			TotalVAT:       0,
			TotalGross:     500,
		},
	}
	inv.SubTotal = 3499
	inv.VATTotal = 630
	inv.TotalAmount = 4129

	if err := repo.Save(ctx, inv); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByID(ctx, "inv-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	// Assert top-level fields
	if got.ID != inv.ID {
		t.Errorf("ID = %q, want %q", got.ID, inv.ID)
	}
	if got.CustomerID != inv.CustomerID {
		t.Errorf("CustomerID = %q, want %q", got.CustomerID, inv.CustomerID)
	}
	if got.CustomerName != inv.CustomerName {
		t.Errorf("CustomerName = %q, want %q", got.CustomerName, inv.CustomerName)
	}
	if got.Code != inv.Code {
		t.Errorf("Code = %q, want %q", got.Code, inv.Code)
	}
	if got.Status != inv.Status {
		t.Errorf("Status = %q, want %q", got.Status, inv.Status)
	}
	if got.CustomerCode != inv.CustomerCode {
		t.Errorf("CustomerCode = %q, want %q", got.CustomerCode, inv.CustomerCode)
	}
	if got.TotalAmount != inv.TotalAmount {
		t.Errorf("TotalAmount = %d, want %d", got.TotalAmount, inv.TotalAmount)
	}
	if got.SubTotal != inv.SubTotal {
		t.Errorf("SubTotal = %d, want %d", got.SubTotal, inv.SubTotal)
	}
	if got.VATTotal != inv.VATTotal {
		t.Errorf("VATTotal = %d, want %d", got.VATTotal, inv.VATTotal)
	}
	if got.Currency != inv.Currency {
		t.Errorf("Currency = %q, want %q", got.Currency, inv.Currency)
	}
	if got.Notes != inv.Notes {
		t.Errorf("Notes = %q, want %q", got.Notes, inv.Notes)
	}

	// Assert line items
	if len(got.LineItems) != 2 {
		t.Fatalf("LineItems count = %d, want 2", len(got.LineItems))
	}

	// Find line items by ID (order may differ)
	liByID := make(map[string]domain.LineItem, len(got.LineItems))
	for _, li := range got.LineItems {
		liByID[li.ID] = li
	}

	li1 := liByID["li-1"]
	if li1.ProductName != "Internet 100Mbps" {
		t.Errorf("li-1 ProductName = %q, want %q", li1.ProductName, "Internet 100Mbps")
	}
	if li1.Quantity != 1 {
		t.Errorf("li-1 Quantity = %d, want 1", li1.Quantity)
	}
	if li1.UnitPriceGross != 3629 {
		t.Errorf("li-1 UnitPriceGross = %d, want 3629", li1.UnitPriceGross)
	}
	if li1.UnitPrice != 2999 {
		t.Errorf("li-1 UnitPrice = %d, want 2999", li1.UnitPrice)
	}
	if li1.VATRate != 21 {
		t.Errorf("li-1 VATRate = %d, want 21", li1.VATRate)
	}
	if li1.TotalPrice != 2999 {
		t.Errorf("li-1 TotalPrice = %d, want 2999", li1.TotalPrice)
	}

	li2 := liByID["li-2"]
	if li2.ProductName != "Router rental" {
		t.Errorf("li-2 ProductName = %q, want %q", li2.ProductName, "Router rental")
	}
}

func TestSaveUpdates(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	inv := makeInvoice("inv-1", "cust-1", "Acme Corp", "INV-00001")
	inv.LineItems = []domain.LineItem{
		{ID: "li-1", ProductName: "Old product", Quantity: 1, UnitPriceGross: 1000, UnitPrice: 1000, VATRate: 0, TotalPrice: 1000, TotalGross: 1000},
	}
	inv.TotalAmount = 1000

	if err := repo.Save(ctx, inv); err != nil {
		t.Fatalf("Save (create): %v", err)
	}

	// Update: change notes, replace line items
	inv.Notes = "updated notes"
	inv.LineItems = []domain.LineItem{
		{ID: "li-3", ProductName: "New product", Quantity: 2, UnitPriceGross: 500, UnitPrice: 500, VATRate: 0, TotalPrice: 1000, TotalGross: 1000},
	}
	inv.TotalAmount = 1000
	inv.UpdatedAt = time.Now().UTC().Truncate(time.Second)

	if err := repo.Save(ctx, inv); err != nil {
		t.Fatalf("Save (update): %v", err)
	}

	got, err := repo.FindByID(ctx, "inv-1")
	if err != nil {
		t.Fatalf("FindByID after update: %v", err)
	}

	if got.Notes != "updated notes" {
		t.Errorf("Notes = %q, want %q", got.Notes, "updated notes")
	}
	if len(got.LineItems) != 1 {
		t.Fatalf("LineItems count = %d, want 1", len(got.LineItems))
	}
	if got.LineItems[0].ID != "li-3" {
		t.Errorf("LineItem ID = %q, want %q", got.LineItems[0].ID, "li-3")
	}
	if got.LineItems[0].ProductName != "New product" {
		t.Errorf("LineItem ProductName = %q, want %q", got.LineItems[0].ProductName, "New product")
	}
	if got.LineItems[0].Quantity != 2 {
		t.Errorf("LineItem Quantity = %d, want 2", got.LineItems[0].Quantity)
	}
}

func TestListByCustomer(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	// Create invoices for two customers
	inv1 := makeInvoice("inv-1", "cust-1", "Acme Corp", "INV-00001")
	inv1.IssueDate = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	inv2 := makeInvoice("inv-2", "cust-1", "Acme Corp", "INV-00002")
	inv2.IssueDate = time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)

	inv3 := makeInvoice("inv-3", "cust-2", "Other Co", "INV-00003")

	for _, inv := range []*domain.Invoice{inv1, inv2, inv3} {
		if err := repo.Save(ctx, inv); err != nil {
			t.Fatalf("Save %s: %v", inv.ID, err)
		}
	}

	// List for cust-1: should return 2 invoices, newest first
	list, err := repo.ListByCustomer(ctx, "cust-1")
	if err != nil {
		t.Fatalf("ListByCustomer: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListByCustomer count = %d, want 2", len(list))
	}
	// Ordered by issue_date DESC: inv-2 (March) before inv-1 (January)
	if list[0].ID != "inv-2" {
		t.Errorf("first invoice ID = %q, want %q", list[0].ID, "inv-2")
	}
	if list[1].ID != "inv-1" {
		t.Errorf("second invoice ID = %q, want %q", list[1].ID, "inv-1")
	}

	// List for cust-2: should return 1 invoice
	list2, err := repo.ListByCustomer(ctx, "cust-2")
	if err != nil {
		t.Fatalf("ListByCustomer cust-2: %v", err)
	}
	if len(list2) != 1 {
		t.Fatalf("ListByCustomer cust-2 count = %d, want 1", len(list2))
	}
	if list2[0].ID != "inv-3" {
		t.Errorf("cust-2 invoice ID = %q, want %q", list2[0].ID, "inv-3")
	}
}

func TestListAll(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)

	inv1 := makeInvoice("inv-1", "cust-1", "Acme Corp", "INV-00001")
	inv1.CreatedAt = now.Add(-2 * time.Hour)

	inv2 := makeInvoice("inv-2", "cust-1", "Acme Corp", "INV-00002")
	inv2.CreatedAt = now.Add(-1 * time.Hour)

	inv3 := makeInvoice("inv-3", "cust-2", "Other Co", "INV-00003")
	inv3.CreatedAt = now

	for _, inv := range []*domain.Invoice{inv1, inv2, inv3} {
		if err := repo.Save(ctx, inv); err != nil {
			t.Fatalf("Save %s: %v", inv.ID, err)
		}
	}

	list, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("ListAll count = %d, want 3", len(list))
	}

	// Ordered by created_at DESC: inv-3, inv-2, inv-1
	if list[0].ID != "inv-3" {
		t.Errorf("first ID = %q, want %q", list[0].ID, "inv-3")
	}
	if list[1].ID != "inv-2" {
		t.Errorf("second ID = %q, want %q", list[1].ID, "inv-2")
	}
	if list[2].ID != "inv-1" {
		t.Errorf("third ID = %q, want %q", list[2].ID, "inv-1")
	}
}

func TestNextCode(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	expected := []string{"INV-00001", "INV-00002", "INV-00003"}
	for i, want := range expected {
		got, err := repo.NextCode(ctx)
		if err != nil {
			t.Fatalf("NextCode call %d: %v", i+1, err)
		}
		if got != want {
			t.Errorf("NextCode call %d = %q, want %q", i+1, got, want)
		}
	}
}

func TestFindByIDNotFound(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "does-not-exist")
	if err == nil {
		t.Fatal("FindByID should return error for non-existent ID")
	}
	if err != domain.ErrInvoiceNotFound {
		t.Errorf("error = %v, want %v", err, domain.ErrInvoiceNotFound)
	}
}
