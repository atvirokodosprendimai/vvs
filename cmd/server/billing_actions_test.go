package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vvs/isp/internal/testutil"

	customermigrations "github.com/vvs/isp/internal/modules/customer/migrations"
	invoicemigrations "github.com/vvs/isp/internal/modules/invoice/migrations"
	servicemigrations "github.com/vvs/isp/internal/modules/service/migrations"

	customerpersistence "github.com/vvs/isp/internal/modules/customer/adapters/persistence"
	customercommands "github.com/vvs/isp/internal/modules/customer/app/commands"
	invoicepersistence "github.com/vvs/isp/internal/modules/invoice/adapters/persistence"
	servicepersistence "github.com/vvs/isp/internal/modules/service/adapters/persistence"
	servicecommands "github.com/vvs/isp/internal/modules/service/app/commands"
)

func setupBillingTest(t *testing.T) (
	*customerpersistence.GormCustomerRepository,
	*customercommands.CreateCustomerHandler,
	*servicepersistence.GormServiceRepository,
	*servicecommands.AssignServiceHandler,
	*invoicepersistence.InvoiceRepository,
) {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, customermigrations.FS, "goose_customer")
	testutil.RunMigrations(t, db, servicemigrations.FS, "goose_service")
	testutil.RunMigrations(t, db, invoicemigrations.FS, "goose_invoice")

	pub, _ := testutil.NewTestNATS(t)

	customerRepo := customerpersistence.NewGormCustomerRepository(db)
	createCustomer := customercommands.NewCreateCustomerHandler(customerRepo, pub, nil)

	serviceRepo := servicepersistence.NewGormServiceRepository(db)
	assignService := servicecommands.NewAssignServiceHandler(serviceRepo, pub)

	invoiceRepo := invoicepersistence.NewInvoiceRepository(db)

	// Wire billing action against the same DB.
	RegisterBillingActions(db)

	return customerRepo, createCustomer, serviceRepo, assignService, invoiceRepo
}

// TestBillingAction_HappyPath verifies the full billing pipeline:
// assign service → backdate NextBillingDate → run action → invoice created + NextBillingDate advanced.
func TestBillingAction_HappyPath(t *testing.T) {
	ctx := context.Background()
	_, createCustomer, serviceRepo, assignService, invoiceRepo := setupBillingTest(t)

	// Create customer.
	cust, err := createCustomer.Handle(ctx, customercommands.CreateCustomerCommand{
		CompanyName: "Test ISP Customer",
	})
	require.NoError(t, err)

	// Assign monthly service.
	svc, err := assignService.Handle(ctx, servicecommands.AssignServiceCommand{
		CustomerID:   cust.ID,
		ProductID:    "prod-1",
		ProductName:  "Fiber 100",
		PriceAmount:  2999,
		Currency:     "EUR",
		StartDate:    time.Now().UTC().AddDate(0, -2, 0),
		BillingCycle: "monthly",
	})
	require.NoError(t, err)

	// Backdate NextBillingDate to yesterday so it is due now.
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	svc.NextBillingDate = &yesterday
	require.NoError(t, serviceRepo.Save(ctx, svc))

	// Run billing action.
	err = actions["generate-due-invoices"](ctx)
	require.NoError(t, err)

	// Assert: invoice created for the customer.
	invoices, err := invoiceRepo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, invoices, 1, "expected exactly 1 invoice")
	assert.Equal(t, cust.ID, invoices[0].CustomerID)

	// Assert: NextBillingDate advanced by 1 month from yesterday.
	updated, err := serviceRepo.FindByID(ctx, svc.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.NextBillingDate)
	expected := yesterday.AddDate(0, 1, 0)
	assert.WithinDuration(t, expected, *updated.NextBillingDate, time.Second)
}

// TestBillingAction_NoDueServices verifies no invoice is created when nothing is due.
func TestBillingAction_NoDueServices(t *testing.T) {
	ctx := context.Background()
	_, createCustomer, _, assignService, invoiceRepo := setupBillingTest(t)

	cust, err := createCustomer.Handle(ctx, customercommands.CreateCustomerCommand{
		CompanyName: "Future Customer",
	})
	require.NoError(t, err)

	// Service with NextBillingDate next month — not yet due.
	_, err = assignService.Handle(ctx, servicecommands.AssignServiceCommand{
		CustomerID:   cust.ID,
		ProductID:    "prod-1",
		ProductName:  "Fiber 100",
		PriceAmount:  2999,
		Currency:     "EUR",
		StartDate:    time.Now().UTC(),
		BillingCycle: "monthly",
	})
	require.NoError(t, err)

	err = actions["generate-due-invoices"](ctx)
	require.NoError(t, err)

	invoices, err := invoiceRepo.ListAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, invoices, "expected no invoices when nothing is due")
}

// TestBillingAction_CustomerNotFound_SkipsGracefully verifies a service with
// a nonexistent customer_id is skipped without error.
func TestBillingAction_CustomerNotFound_SkipsGracefully(t *testing.T) {
	ctx := context.Background()
	_, _, serviceRepo, assignService, invoiceRepo := setupBillingTest(t)

	past := time.Now().UTC().AddDate(0, 0, -1)

	// Assign directly to a nonexistent customer — service repo doesn't FK-check customer_id.
	svc, err := assignService.Handle(ctx, servicecommands.AssignServiceCommand{
		CustomerID:   "nonexistent-customer-id",
		ProductID:    "prod-1",
		ProductName:  "Orphan Service",
		PriceAmount:  1000,
		Currency:     "EUR",
		StartDate:    time.Now().UTC().AddDate(0, -2, 0),
		BillingCycle: "monthly",
	})
	require.NoError(t, err)

	// Backdate so it's due.
	svc.NextBillingDate = &past
	require.NoError(t, serviceRepo.Save(ctx, svc))

	// Must not return error — just log and skip the orphan customer.
	err = actions["generate-due-invoices"](ctx)
	require.NoError(t, err)

	// No invoice created for nonexistent customer.
	invoices, err := invoiceRepo.ListAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, invoices)
}
