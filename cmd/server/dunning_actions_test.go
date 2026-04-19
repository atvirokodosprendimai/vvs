package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/testutil"

	customerdomain "github.com/vvs/isp/internal/modules/customer/domain"
	customermigrations "github.com/vvs/isp/internal/modules/customer/migrations"
	customerpersistence "github.com/vvs/isp/internal/modules/customer/adapters/persistence"
	invoicedomain "github.com/vvs/isp/internal/modules/invoice/domain"
	invoicemigrations "github.com/vvs/isp/internal/modules/invoice/migrations"
	invoicepersistence "github.com/vvs/isp/internal/modules/invoice/adapters/persistence"
	invoicecommands "github.com/vvs/isp/internal/modules/invoice/app/commands"
	sharedomain "github.com/vvs/isp/internal/shared/domain"
)

// captureMailer records sent emails without delivering them.
type captureMailer struct {
	sent []string // recipient addresses
}

func (m *captureMailer) SendPlain(_ context.Context, to, _, _ string) error {
	m.sent = append(m.sent, to)
	return nil
}

// Compile-time check: captureMailer satisfies the EmailSender port.
var _ invoicecommands.EmailSender = (*captureMailer)(nil)

// setupDunningTest creates a fresh DB with customer+invoice migrations, wires the
// dunning action with the supplied mailer, and returns repos for fixture setup.
func setupDunningTest(t *testing.T, mailer invoicecommands.EmailSender) (
	*gormsqlite.DB,
	*customerpersistence.GormCustomerRepository,
	*invoicepersistence.InvoiceRepository,
) {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, customermigrations.FS, "goose_customer")
	testutil.RunMigrations(t, db, invoicemigrations.FS, "goose_invoice")

	RegisterDunningActionsWithMailer(db, mailer)

	return db,
		customerpersistence.NewGormCustomerRepository(db),
		invoicepersistence.NewInvoiceRepository(db)
}

func newTestCustomer(t *testing.T, repo *customerpersistence.GormCustomerRepository) *customerdomain.Customer {
	t.Helper()
	c, err := customerdomain.NewCustomer(
		sharedomain.NewCompanyCode("ACM", 1),
		"ACME Corp", "John Doe", "john@acme.lt", "+37060000000",
	)
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), c))
	return c
}

func newOverdueInvoice(id, customerID string) *invoicedomain.Invoice {
	inv := invoicedomain.NewInvoice(id, customerID, "ACME Corp", "ACM-00001", "INV-"+id)
	inv.Status = invoicedomain.StatusFinalized
	inv.DueDate = time.Now().UTC().Add(-48 * time.Hour)
	inv.TotalAmount = 10000
	return inv
}

// TestDunningAction_HappyPath: overdue invoice → reminder sent → ReminderSentAt persisted.
func TestDunningAction_HappyPath(t *testing.T) {
	ctx := context.Background()
	mailer := &captureMailer{}
	_, customerRepo, invoiceRepo := setupDunningTest(t, mailer)

	cust := newTestCustomer(t, customerRepo)
	inv := newOverdueInvoice("001", cust.ID)
	require.NoError(t, invoiceRepo.Save(ctx, inv))

	require.NoError(t, actions["send-dunning-reminders"](ctx))

	assert.Len(t, mailer.sent, 1)
	assert.Equal(t, "john@acme.lt", mailer.sent[0])

	updated, err := invoiceRepo.FindByID(ctx, inv.ID)
	require.NoError(t, err)
	assert.NotNil(t, updated.ReminderSentAt)
}

// TestDunningAction_PaidInvoice_Skipped: paid invoice must never receive a reminder.
func TestDunningAction_PaidInvoice_Skipped(t *testing.T) {
	ctx := context.Background()
	mailer := &captureMailer{}
	_, _, invoiceRepo := setupDunningTest(t, mailer)

	inv := newOverdueInvoice("paid-001", "cust-x")
	inv.Status = invoicedomain.StatusPaid // paid — must not appear in ListOverdue
	require.NoError(t, invoiceRepo.Save(ctx, inv))

	require.NoError(t, actions["send-dunning-reminders"](ctx))

	assert.Empty(t, mailer.sent, "paid invoice must not receive reminder")

	updated, err := invoiceRepo.FindByID(ctx, inv.ID)
	require.NoError(t, err)
	assert.Nil(t, updated.ReminderSentAt)
}

// TestDunningAction_RecentlyReminded_Cooldown: reminded 1 hour ago, 7-day cooldown → skipped.
func TestDunningAction_RecentlyReminded_Cooldown(t *testing.T) {
	ctx := context.Background()
	mailer := &captureMailer{}
	_, _, invoiceRepo := setupDunningTest(t, mailer)

	sentAt := time.Now().UTC().Add(-1 * time.Hour)
	inv := newOverdueInvoice("cooldown-001", "cust-y")
	inv.ReminderSentAt = &sentAt
	require.NoError(t, invoiceRepo.Save(ctx, inv))

	require.NoError(t, actions["send-dunning-reminders"](ctx))

	assert.Empty(t, mailer.sent, "invoice within 7-day cooldown must not receive reminder")

	updated, err := invoiceRepo.FindByID(ctx, inv.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.ReminderSentAt)
	assert.WithinDuration(t, sentAt, *updated.ReminderSentAt, time.Second)
}
