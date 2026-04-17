package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vvs/isp/internal/modules/invoice/adapters/persistence"
	"github.com/vvs/isp/internal/modules/invoice/app/commands"
	"github.com/vvs/isp/internal/modules/invoice/domain"
	"github.com/vvs/isp/internal/modules/invoice/migrations"
	"github.com/vvs/isp/internal/testutil"
)

func setupTest(t *testing.T) (*persistence.InvoiceRepository, *commands.CreateInvoiceHandler) {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_invoice")
	pub, _ := testutil.NewTestNATS(t)
	repo := persistence.NewInvoiceRepository(db)
	handler := commands.NewCreateInvoiceHandler(repo, pub)
	return repo, handler
}

func createDraftInvoice(t *testing.T, repo *persistence.InvoiceRepository, handler *commands.CreateInvoiceHandler) *domain.Invoice {
	t.Helper()
	now := time.Now().UTC()
	cmd := commands.CreateInvoiceCommand{
		CustomerID:   "cust-1",
		CustomerName: "Acme Corp",
		IssueDate:    now,
		DueDate:      now.AddDate(0, 0, 30),
		Notes:        "Test invoice",
		LineItems: []commands.LineItemInput{
			{
				ProductID:   "prod-1",
				ProductName: "Internet 100Mbps",
				Description: "Monthly subscription",
				Quantity:    1,
				UnitPriceGross:   2999,
			},
		},
	}
	inv, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	return inv
}

func TestCreateInvoiceHandler(t *testing.T) {
	repo, handler := setupTest(t)

	now := time.Now().UTC()
	cmd := commands.CreateInvoiceCommand{
		CustomerID:   "cust-1",
		CustomerName: "Acme Corp",
		IssueDate:    now,
		DueDate:      now.AddDate(0, 0, 30),
		Notes:        "First invoice",
		LineItems: []commands.LineItemInput{
			{
				ProductID:   "prod-1",
				ProductName: "Internet 100Mbps",
				Description: "Monthly",
				Quantity:    1,
				UnitPriceGross:   2999,
			},
			{
				ProductID:   "prod-2",
				ProductName: "Static IP",
				Description: "Extra IP address",
				Quantity:    2,
				UnitPriceGross:   500,
			},
		},
	}

	inv, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	require.NotNil(t, inv)

	assert.Equal(t, "cust-1", inv.CustomerID)
	assert.Equal(t, "Acme Corp", inv.CustomerName)
	assert.Equal(t, "INV-00001", inv.Code)
	assert.Equal(t, domain.StatusDraft, inv.Status)
	assert.Equal(t, "First invoice", inv.Notes)
	assert.Len(t, inv.LineItems, 2)
	assert.Equal(t, int64(2999), inv.LineItems[0].TotalPrice)
	assert.Equal(t, int64(1000), inv.LineItems[1].TotalPrice)
	assert.Equal(t, int64(3999), inv.TotalAmount)

	// Verify persisted
	found, err := repo.FindByID(context.Background(), inv.ID)
	require.NoError(t, err)
	assert.Equal(t, inv.Code, found.Code)
	assert.Equal(t, inv.TotalAmount, found.TotalAmount)
	assert.Len(t, found.LineItems, 2)
}

func TestCreateInvoiceHandler_SequentialCodes(t *testing.T) {
	_, handler := setupTest(t)

	now := time.Now().UTC()
	cmd := commands.CreateInvoiceCommand{
		CustomerID:   "cust-1",
		CustomerName: "Test",
		IssueDate:    now,
		DueDate:      now.AddDate(0, 0, 30),
	}

	inv1, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	assert.Equal(t, "INV-00001", inv1.Code)

	inv2, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	assert.Equal(t, "INV-00002", inv2.Code)
}

func TestFinalizeInvoiceHandler(t *testing.T) {
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_invoice")
	pub, _ := testutil.NewTestNATS(t)
	repo := persistence.NewInvoiceRepository(db)

	createHandler := commands.NewCreateInvoiceHandler(repo, pub)
	inv := createDraftInvoice(t, repo, createHandler)

	finalizeHandler := commands.NewFinalizeInvoiceHandler(repo, pub)
	finalized, err := finalizeHandler.Handle(context.Background(), commands.FinalizeInvoiceCommand{
		InvoiceID: inv.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusFinalized, finalized.Status)

	// Verify persisted
	found, err := repo.FindByID(context.Background(), inv.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusFinalized, found.Status)
}

func TestMarkPaidHandler(t *testing.T) {
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_invoice")
	pub, _ := testutil.NewTestNATS(t)
	repo := persistence.NewInvoiceRepository(db)

	createHandler := commands.NewCreateInvoiceHandler(repo, pub)
	inv := createDraftInvoice(t, repo, createHandler)

	// Finalize first
	finalizeHandler := commands.NewFinalizeInvoiceHandler(repo, pub)
	_, err := finalizeHandler.Handle(context.Background(), commands.FinalizeInvoiceCommand{
		InvoiceID: inv.ID,
	})
	require.NoError(t, err)

	// Mark paid
	paidHandler := commands.NewMarkPaidHandler(repo, pub)
	paid, err := paidHandler.Handle(context.Background(), commands.MarkPaidCommand{
		InvoiceID: inv.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusPaid, paid.Status)
	assert.NotNil(t, paid.PaidAt)

	// Verify persisted
	found, err := repo.FindByID(context.Background(), inv.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusPaid, found.Status)
	assert.NotNil(t, found.PaidAt)
}

func TestVoidInvoiceHandler(t *testing.T) {
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_invoice")
	pub, _ := testutil.NewTestNATS(t)
	repo := persistence.NewInvoiceRepository(db)

	createHandler := commands.NewCreateInvoiceHandler(repo, pub)
	inv := createDraftInvoice(t, repo, createHandler)

	voidHandler := commands.NewVoidInvoiceHandler(repo, pub)
	voided, err := voidHandler.Handle(context.Background(), commands.VoidInvoiceCommand{
		InvoiceID: inv.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusVoid, voided.Status)

	// Verify persisted
	found, err := repo.FindByID(context.Background(), inv.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusVoid, found.Status)
}

func TestAddLineItemHandler(t *testing.T) {
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_invoice")
	pub, _ := testutil.NewTestNATS(t)
	repo := persistence.NewInvoiceRepository(db)

	createHandler := commands.NewCreateInvoiceHandler(repo, pub)
	inv := createDraftInvoice(t, repo, createHandler)
	originalTotal := inv.TotalAmount

	addHandler := commands.NewAddLineItemHandler(repo, pub)
	updated, err := addHandler.Handle(context.Background(), commands.AddLineItemCommand{
		InvoiceID:   inv.ID,
		ProductID:   "prod-2",
		ProductName: "Static IP",
		Description: "Extra IP address",
		Quantity:    3,
		UnitPriceGross:   500,
	})
	require.NoError(t, err)
	assert.Len(t, updated.LineItems, 2)
	assert.Equal(t, originalTotal+1500, updated.TotalAmount)

	// Verify persisted
	found, err := repo.FindByID(context.Background(), inv.ID)
	require.NoError(t, err)
	assert.Len(t, found.LineItems, 2)
	assert.Equal(t, updated.TotalAmount, found.TotalAmount)
}

func TestRemoveLineItemHandler(t *testing.T) {
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_invoice")
	pub, _ := testutil.NewTestNATS(t)
	repo := persistence.NewInvoiceRepository(db)

	// Create invoice with 2 line items
	createHandler := commands.NewCreateInvoiceHandler(repo, pub)
	now := time.Now().UTC()
	inv, err := createHandler.Handle(context.Background(), commands.CreateInvoiceCommand{
		CustomerID:   "cust-1",
		CustomerName: "Acme Corp",
		IssueDate:    now,
		DueDate:      now.AddDate(0, 0, 30),
		LineItems: []commands.LineItemInput{
			{ProductID: "prod-1", ProductName: "Internet", Quantity: 1, UnitPriceGross: 2999},
			{ProductID: "prod-2", ProductName: "Static IP", Quantity: 1, UnitPriceGross: 500},
		},
	})
	require.NoError(t, err)
	require.Len(t, inv.LineItems, 2)

	removeHandler := commands.NewRemoveLineItemHandler(repo, pub)
	updated, err := removeHandler.Handle(context.Background(), commands.RemoveLineItemCommand{
		InvoiceID:  inv.ID,
		LineItemID: inv.LineItems[0].ID,
	})
	require.NoError(t, err)
	assert.Len(t, updated.LineItems, 1)
	assert.Equal(t, int64(500), updated.TotalAmount)
}

func TestFinalizeInvoiceHandler_EmptyInvoice(t *testing.T) {
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_invoice")
	pub, _ := testutil.NewTestNATS(t)
	repo := persistence.NewInvoiceRepository(db)

	// Create invoice with no line items
	createHandler := commands.NewCreateInvoiceHandler(repo, pub)
	now := time.Now().UTC()
	inv, err := createHandler.Handle(context.Background(), commands.CreateInvoiceCommand{
		CustomerID:   "cust-1",
		CustomerName: "Acme Corp",
		IssueDate:    now,
		DueDate:      now.AddDate(0, 0, 30),
	})
	require.NoError(t, err)

	// Try to finalize — should fail with ErrNoLineItems
	finalizeHandler := commands.NewFinalizeInvoiceHandler(repo, pub)
	_, err = finalizeHandler.Handle(context.Background(), commands.FinalizeInvoiceCommand{
		InvoiceID: inv.ID,
	})
	assert.ErrorIs(t, err, domain.ErrNoLineItems)
}

func TestMarkPaidHandler_FromDraft(t *testing.T) {
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_invoice")
	pub, _ := testutil.NewTestNATS(t)
	repo := persistence.NewInvoiceRepository(db)

	createHandler := commands.NewCreateInvoiceHandler(repo, pub)
	inv := createDraftInvoice(t, repo, createHandler)

	// Try to mark paid from draft — should fail
	paidHandler := commands.NewMarkPaidHandler(repo, pub)
	_, err := paidHandler.Handle(context.Background(), commands.MarkPaidCommand{
		InvoiceID: inv.ID,
	})
	assert.ErrorIs(t, err, domain.ErrInvalidTransition)
}

func TestGenerateFromSubscriptionsHandler(t *testing.T) {
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_invoice")
	pub, _ := testutil.NewTestNATS(t)
	repo := persistence.NewInvoiceRepository(db)

	lister := &stubServiceLister{
		services: []commands.ServiceInfo{
			{ID: "svc-1", ProductID: "prod-1", ProductName: "Internet 100Mbps", PriceAmount: 2999},
			{ID: "svc-2", ProductID: "prod-2", ProductName: "Static IP", PriceAmount: 500},
		},
	}

	handler := commands.NewGenerateFromSubscriptionsHandler(repo, pub, lister)
	inv, err := handler.Handle(context.Background(), commands.GenerateFromSubscriptionsCommand{
		CustomerID:   "cust-1",
		CustomerName: "Acme Corp",
	})
	require.NoError(t, err)
	require.NotNil(t, inv)

	assert.Equal(t, "cust-1", inv.CustomerID)
	assert.Equal(t, "Acme Corp", inv.CustomerName)
	assert.Equal(t, domain.StatusDraft, inv.Status)
	assert.Len(t, inv.LineItems, 2)
	assert.Equal(t, int64(3499), inv.TotalAmount)
	assert.NotEmpty(t, inv.Code)

	// DueDate should be ~30 days from now
	assert.WithinDuration(t, time.Now().UTC().AddDate(0, 0, 30), inv.DueDate, 5*time.Second)
}

// stubServiceLister is a test double for ActiveServiceLister.
type stubServiceLister struct {
	services []commands.ServiceInfo
}

func (s *stubServiceLister) ListActiveForCustomer(_ context.Context, _ string) ([]commands.ServiceInfo, error) {
	return s.services, nil
}
