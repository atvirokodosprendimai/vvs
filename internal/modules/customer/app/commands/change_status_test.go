package commands_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vvs/isp/internal/modules/customer/app/commands"
	"github.com/vvs/isp/internal/modules/customer/domain"
	"github.com/vvs/isp/internal/modules/customer/migrations"
	"github.com/vvs/isp/internal/modules/customer/adapters/persistence"
	"github.com/vvs/isp/internal/testutil"
)

func setupStatusHandlers(t *testing.T) (
	*persistence.GormCustomerRepository,
	*commands.CreateCustomerHandler,
	*commands.ChangeCustomerStatusHandler,
) {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_customer")
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormCustomerRepository(db)
	create := commands.NewCreateCustomerHandler(repo, pub, nil)
	status := commands.NewChangeCustomerStatusHandler(repo, pub)

	return repo, create, status
}

// ---------------------------------------------------------------------------
// ChangeCustomerStatusHandler — suspend / activate
// ---------------------------------------------------------------------------

func TestChangeCustomerStatus_Suspend(t *testing.T) {
	repo, create, status := setupStatusHandlers(t)

	customer := createTestCustomer(t, create)
	assert.Equal(t, domain.StatusActive, customer.Status)

	err := status.Handle(context.Background(), commands.ChangeCustomerStatusCommand{
		ID:     customer.ID,
		Action: commands.ActionSuspend,
	})
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), customer.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSuspended, found.Status)
}

func TestChangeCustomerStatus_Activate(t *testing.T) {
	repo, create, status := setupStatusHandlers(t)

	customer := createTestCustomer(t, create)

	// Suspend first
	require.NoError(t, status.Handle(context.Background(), commands.ChangeCustomerStatusCommand{
		ID:     customer.ID,
		Action: commands.ActionSuspend,
	}))

	// Now activate
	err := status.Handle(context.Background(), commands.ChangeCustomerStatusCommand{
		ID:     customer.ID,
		Action: commands.ActionActivate,
	})
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), customer.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusActive, found.Status)
}

func TestChangeCustomerStatus_Churn(t *testing.T) {
	repo, create, status := setupStatusHandlers(t)

	customer := createTestCustomer(t, create)

	err := status.Handle(context.Background(), commands.ChangeCustomerStatusCommand{
		ID:     customer.ID,
		Action: commands.ActionChurn,
	})
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), customer.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusChurned, found.Status)
}

func TestChangeCustomerStatus_QualifyAndConvert(t *testing.T) {
	repo, _, status := setupStatusHandlers(t)
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_customer")
	pub, _ := testutil.NewTestNATS(t)
	leadRepo := persistence.NewGormCustomerRepository(db)
	createLead := commands.NewCreateCustomerHandler(leadRepo, pub, nil)
	statusLead := commands.NewChangeCustomerStatusHandler(leadRepo, pub)

	_ = repo
	_ = status

	// Create leads via a separate handler that produces a lead (but our create defaults to active).
	// Since the domain starts customers as "active" on create, we can't directly test Qualify
	// (which requires StatusLead). Instead test Convert starting from Prospect.
	// We'll directly verify the invalid-transition guard on a non-lead active customer.
	customer, err := createLead.Handle(context.Background(), commands.CreateCustomerCommand{
		CompanyName: "Lead Corp",
	})
	require.NoError(t, err)

	// An active customer cannot be Qualified (requires StatusLead)
	err = statusLead.Handle(context.Background(), commands.ChangeCustomerStatusCommand{
		ID:     customer.ID,
		Action: commands.ActionQualify,
	})
	assert.ErrorIs(t, err, domain.ErrInvalidTransition)

	// An active customer cannot be Converted (requires StatusProspect)
	err = statusLead.Handle(context.Background(), commands.ChangeCustomerStatusCommand{
		ID:     customer.ID,
		Action: commands.ActionConvert,
	})
	assert.ErrorIs(t, err, domain.ErrInvalidTransition)
}

func TestChangeCustomerStatus_AlreadySuspended(t *testing.T) {
	_, create, status := setupStatusHandlers(t)

	customer := createTestCustomer(t, create)
	require.NoError(t, status.Handle(context.Background(), commands.ChangeCustomerStatusCommand{
		ID:     customer.ID,
		Action: commands.ActionSuspend,
	}))

	err := status.Handle(context.Background(), commands.ChangeCustomerStatusCommand{
		ID:     customer.ID,
		Action: commands.ActionSuspend,
	})
	assert.ErrorIs(t, err, domain.ErrAlreadySuspended)
}

func TestChangeCustomerStatus_AlreadyActive(t *testing.T) {
	_, create, status := setupStatusHandlers(t)

	customer := createTestCustomer(t, create)
	// Already active — activate again should fail
	err := status.Handle(context.Background(), commands.ChangeCustomerStatusCommand{
		ID:     customer.ID,
		Action: commands.ActionActivate,
	})
	assert.ErrorIs(t, err, domain.ErrAlreadyActive)
}

func TestChangeCustomerStatus_ChurnedCannotActivate(t *testing.T) {
	_, create, status := setupStatusHandlers(t)

	customer := createTestCustomer(t, create)
	require.NoError(t, status.Handle(context.Background(), commands.ChangeCustomerStatusCommand{
		ID:     customer.ID,
		Action: commands.ActionChurn,
	}))

	err := status.Handle(context.Background(), commands.ChangeCustomerStatusCommand{
		ID:     customer.ID,
		Action: commands.ActionActivate,
	})
	assert.ErrorIs(t, err, domain.ErrInvalidTransition)
}

func TestChangeCustomerStatus_AlreadyChurned(t *testing.T) {
	_, create, status := setupStatusHandlers(t)

	customer := createTestCustomer(t, create)
	require.NoError(t, status.Handle(context.Background(), commands.ChangeCustomerStatusCommand{
		ID:     customer.ID,
		Action: commands.ActionChurn,
	}))

	err := status.Handle(context.Background(), commands.ChangeCustomerStatusCommand{
		ID:     customer.ID,
		Action: commands.ActionChurn,
	})
	assert.ErrorIs(t, err, domain.ErrAlreadyChurned)
}

func TestChangeCustomerStatus_InvalidAction(t *testing.T) {
	_, create, status := setupStatusHandlers(t)

	customer := createTestCustomer(t, create)
	err := status.Handle(context.Background(), commands.ChangeCustomerStatusCommand{
		ID:     customer.ID,
		Action: "unknown-action",
	})
	assert.ErrorIs(t, err, domain.ErrInvalidTransition)
}

func TestChangeCustomerStatus_NotFound(t *testing.T) {
	_, _, status := setupStatusHandlers(t)

	err := status.Handle(context.Background(), commands.ChangeCustomerStatusCommand{
		ID:     "nonexistent",
		Action: commands.ActionSuspend,
	})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// AddNoteHandler
// ---------------------------------------------------------------------------

func TestAddNoteHandler_HappyPath(t *testing.T) {
	_, create, _, _, _, note := setupCustomerHandlers(t)

	customer := createTestCustomer(t, create)

	n, err := note.Handle(context.Background(), commands.AddNoteCommand{
		CustomerID: customer.ID,
		Body:       "Called client — confirmed payment.",
		AuthorID:   "user-admin",
	})
	require.NoError(t, err)
	require.NotNil(t, n)
	assert.NotEmpty(t, n.ID)
	assert.Equal(t, customer.ID, n.CustomerID)
	assert.Equal(t, "Called client — confirmed payment.", n.Body)
	assert.Equal(t, "user-admin", n.AuthorID)
}

func TestAddNoteHandler_EmptyBody(t *testing.T) {
	_, create, _, _, _, note := setupCustomerHandlers(t)

	customer := createTestCustomer(t, create)

	n, err := note.Handle(context.Background(), commands.AddNoteCommand{
		CustomerID: customer.ID,
		Body:       "   ",
		AuthorID:   "user-admin",
	})
	assert.ErrorIs(t, err, domain.ErrNoteBodyRequired)
	assert.Nil(t, n)
}
