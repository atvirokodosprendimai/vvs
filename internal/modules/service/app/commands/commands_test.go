package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/atvirokodosprendimai/vvs/internal/modules/service/adapters/persistence"
	"github.com/atvirokodosprendimai/vvs/internal/modules/service/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/service/domain"
	"github.com/atvirokodosprendimai/vvs/internal/modules/service/migrations"
	"github.com/atvirokodosprendimai/vvs/internal/testutil"
)

func setup(t *testing.T) (*persistence.GormServiceRepository, *commands.AssignServiceHandler, *commands.SuspendServiceHandler, *commands.ReactivateServiceHandler, *commands.CancelServiceHandler) {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_service")
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormServiceRepository(db)
	assign := commands.NewAssignServiceHandler(repo, pub)
	suspend := commands.NewSuspendServiceHandler(repo, pub)
	reactivate := commands.NewReactivateServiceHandler(repo, pub)
	cancel := commands.NewCancelServiceHandler(repo, pub)

	return repo, assign, suspend, reactivate, cancel
}

func assignedService(t *testing.T, handler *commands.AssignServiceHandler) *domain.Service {
	t.Helper()
	svc, err := handler.Handle(context.Background(), commands.AssignServiceCommand{
		CustomerID:  "cust-001",
		ProductID:   "prod-001",
		ProductName: "Fibre 100",
		PriceAmount: 2500,
		Currency:    "EUR",
		StartDate:   time.Now().UTC(),
	})
	require.NoError(t, err)
	require.NotNil(t, svc)
	return svc
}

// ---------------------------------------------------------------------------
// AssignServiceHandler
// ---------------------------------------------------------------------------

func TestAssignServiceHandler_HappyPath(t *testing.T) {
	repo, assign, _, _, _ := setup(t)

	svc, err := assign.Handle(context.Background(), commands.AssignServiceCommand{
		CustomerID:  "cust-abc",
		ProductID:   "prod-xyz",
		ProductName: "Fibre 500",
		PriceAmount: 4999,
		Currency:    "EUR",
		StartDate:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})

	require.NoError(t, err)
	require.NotNil(t, svc)
	assert.NotEmpty(t, svc.ID)
	assert.Equal(t, "cust-abc", svc.CustomerID)
	assert.Equal(t, "prod-xyz", svc.ProductID)
	assert.Equal(t, "Fibre 500", svc.ProductName)
	assert.Equal(t, int64(4999), svc.PriceAmount)
	assert.Equal(t, "EUR", svc.Currency)
	assert.Equal(t, domain.StatusActive, svc.Status)

	// Verify persisted
	found, err := repo.FindByID(context.Background(), svc.ID)
	require.NoError(t, err)
	assert.Equal(t, svc.ID, found.ID)
	assert.Equal(t, domain.StatusActive, found.Status)
}

func TestAssignServiceHandler_DefaultsCurrencyToEUR(t *testing.T) {
	_, assign, _, _, _ := setup(t)

	svc, err := assign.Handle(context.Background(), commands.AssignServiceCommand{
		CustomerID:  "cust-001",
		ProductID:   "prod-001",
		ProductName: "Basic Plan",
		PriceAmount: 1000,
		Currency:    "", // intentionally empty
		StartDate:   time.Now().UTC(),
	})

	require.NoError(t, err)
	assert.Equal(t, "EUR", svc.Currency)
}

func TestAssignServiceHandler_MissingCustomerID(t *testing.T) {
	_, assign, _, _, _ := setup(t)

	svc, err := assign.Handle(context.Background(), commands.AssignServiceCommand{
		CustomerID:  "",
		ProductID:   "prod-001",
		ProductName: "Plan",
		StartDate:   time.Now().UTC(),
	})

	assert.Error(t, err)
	assert.Nil(t, svc)
}

func TestAssignServiceHandler_MissingProductID(t *testing.T) {
	_, assign, _, _, _ := setup(t)

	svc, err := assign.Handle(context.Background(), commands.AssignServiceCommand{
		CustomerID:  "cust-001",
		ProductID:   "",
		ProductName: "Plan",
		StartDate:   time.Now().UTC(),
	})

	assert.Error(t, err)
	assert.Nil(t, svc)
}

func TestAssignServiceHandler_MissingProductName(t *testing.T) {
	_, assign, _, _, _ := setup(t)

	svc, err := assign.Handle(context.Background(), commands.AssignServiceCommand{
		CustomerID:  "cust-001",
		ProductID:   "prod-001",
		ProductName: "",
		StartDate:   time.Now().UTC(),
	})

	assert.Error(t, err)
	assert.Nil(t, svc)
}

// ---------------------------------------------------------------------------
// SuspendServiceHandler
// ---------------------------------------------------------------------------

func TestSuspendServiceHandler_HappyPath(t *testing.T) {
	repo, assign, suspend, _, _ := setup(t)

	svc := assignedService(t, assign)

	err := suspend.Handle(context.Background(), commands.SuspendServiceCommand{ID: svc.ID})
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), svc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSuspended, found.Status)
}

func TestSuspendServiceHandler_AlreadySuspended(t *testing.T) {
	_, assign, suspend, _, _ := setup(t)

	svc := assignedService(t, assign)

	require.NoError(t, suspend.Handle(context.Background(), commands.SuspendServiceCommand{ID: svc.ID}))

	// Suspending again must fail
	err := suspend.Handle(context.Background(), commands.SuspendServiceCommand{ID: svc.ID})
	assert.ErrorIs(t, err, domain.ErrInvalidTransition)
}

func TestSuspendServiceHandler_NotFound(t *testing.T) {
	_, _, suspend, _, _ := setup(t)

	err := suspend.Handle(context.Background(), commands.SuspendServiceCommand{ID: "nonexistent"})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ---------------------------------------------------------------------------
// ReactivateServiceHandler
// ---------------------------------------------------------------------------

func TestReactivateServiceHandler_HappyPath(t *testing.T) {
	repo, assign, suspend, reactivate, _ := setup(t)

	svc := assignedService(t, assign)
	require.NoError(t, suspend.Handle(context.Background(), commands.SuspendServiceCommand{ID: svc.ID}))

	err := reactivate.Handle(context.Background(), commands.ReactivateServiceCommand{ID: svc.ID})
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), svc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusActive, found.Status)
}

func TestReactivateServiceHandler_RequiresSuspended(t *testing.T) {
	_, assign, _, reactivate, _ := setup(t)

	// Active service cannot be reactivated directly
	svc := assignedService(t, assign)

	err := reactivate.Handle(context.Background(), commands.ReactivateServiceCommand{ID: svc.ID})
	assert.ErrorIs(t, err, domain.ErrInvalidTransition)
}

func TestReactivateServiceHandler_NotFound(t *testing.T) {
	_, _, _, reactivate, _ := setup(t)

	err := reactivate.Handle(context.Background(), commands.ReactivateServiceCommand{ID: "nonexistent"})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ---------------------------------------------------------------------------
// CancelServiceHandler
// ---------------------------------------------------------------------------

func TestCancelServiceHandler_HappyPath(t *testing.T) {
	repo, assign, _, _, cancel := setup(t)

	svc := assignedService(t, assign)

	err := cancel.Handle(context.Background(), commands.CancelServiceCommand{ID: svc.ID})
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), svc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCancelled, found.Status)
}

func TestCancelServiceHandler_CanCancelSuspended(t *testing.T) {
	repo, assign, suspend, _, cancel := setup(t)

	svc := assignedService(t, assign)
	require.NoError(t, suspend.Handle(context.Background(), commands.SuspendServiceCommand{ID: svc.ID}))

	err := cancel.Handle(context.Background(), commands.CancelServiceCommand{ID: svc.ID})
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), svc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCancelled, found.Status)
}

func TestCancelServiceHandler_CannotReactivateAfterCancel(t *testing.T) {
	_, assign, _, reactivate, cancel := setup(t)

	svc := assignedService(t, assign)
	require.NoError(t, cancel.Handle(context.Background(), commands.CancelServiceCommand{ID: svc.ID}))

	// Reactivate requires suspended status, not cancelled
	err := reactivate.Handle(context.Background(), commands.ReactivateServiceCommand{ID: svc.ID})
	assert.ErrorIs(t, err, domain.ErrInvalidTransition)
}

func TestCancelServiceHandler_AlreadyCancelled(t *testing.T) {
	_, assign, _, _, cancel := setup(t)

	svc := assignedService(t, assign)
	require.NoError(t, cancel.Handle(context.Background(), commands.CancelServiceCommand{ID: svc.ID}))

	err := cancel.Handle(context.Background(), commands.CancelServiceCommand{ID: svc.ID})
	assert.ErrorIs(t, err, domain.ErrInvalidTransition)
}

func TestCancelServiceHandler_NotFound(t *testing.T) {
	_, _, _, _, cancel := setup(t)

	err := cancel.Handle(context.Background(), commands.CancelServiceCommand{ID: "nonexistent"})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
