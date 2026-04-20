package persistence_test

import (
	"context"
	"testing"

	"github.com/atvirokodosprendimai/vvs/internal/modules/billing/adapters/persistence"
	"github.com/atvirokodosprendimai/vvs/internal/modules/billing/domain"
	billingmigrations "github.com/atvirokodosprendimai/vvs/internal/modules/billing/migrations"
	"github.com/atvirokodosprendimai/vvs/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRepo(t *testing.T) domain.BalanceRepository {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, billingmigrations.FS, "goose_billing")
	return persistence.NewGormBalanceRepository(db)
}

func TestBalance_ZeroForNewCustomer(t *testing.T) {
	repo := setupRepo(t)
	bal, err := repo.GetBalance(context.Background(), "cust-1")
	require.NoError(t, err)
	assert.Equal(t, int64(0), bal)
}

func TestBalance_Credit(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Credit(ctx, "cust-1", 1000, domain.EntryTypeTopUp, "test credit", ""))
	bal, err := repo.GetBalance(ctx, "cust-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1000), bal)
}

func TestBalance_MultipleCredits(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Credit(ctx, "cust-1", 500, domain.EntryTypeTopUp, "first", ""))
	require.NoError(t, repo.Credit(ctx, "cust-1", 300, domain.EntryTypeTopUp, "second", ""))
	bal, err := repo.GetBalance(ctx, "cust-1")
	require.NoError(t, err)
	assert.Equal(t, int64(800), bal)
}

func TestBalance_DeductSuccess(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Credit(ctx, "cust-1", 1000, domain.EntryTypeTopUp, "top up", ""))
	require.NoError(t, repo.Deduct(ctx, "cust-1", 400, domain.EntryTypeVMPurchase, "buy vm"))
	bal, err := repo.GetBalance(ctx, "cust-1")
	require.NoError(t, err)
	assert.Equal(t, int64(600), bal)
}

func TestBalance_DeductInsufficientBalance(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Credit(ctx, "cust-1", 100, domain.EntryTypeTopUp, "top up", ""))
	err := repo.Deduct(ctx, "cust-1", 500, domain.EntryTypeVMPurchase, "too much")
	assert.ErrorIs(t, err, domain.ErrInsufficientBalance)

	// Balance unchanged
	bal, err := repo.GetBalance(ctx, "cust-1")
	require.NoError(t, err)
	assert.Equal(t, int64(100), bal)
}

func TestBalance_StripeIdempotency(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Credit(ctx, "cust-1", 500, domain.EntryTypeTopUp, "stripe", "sess_abc"))
	// Second call with same stripe session ID must fail (unique index)
	err := repo.Credit(ctx, "cust-1", 500, domain.EntryTypeTopUp, "stripe dup", "sess_abc")
	assert.Error(t, err) // unique constraint violation

	// Balance = 500 (not 1000)
	bal, err := repo.GetBalance(ctx, "cust-1")
	require.NoError(t, err)
	assert.Equal(t, int64(500), bal)
}

func TestBalance_GetLedger(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Credit(ctx, "cust-1", 1000, domain.EntryTypeTopUp, "top up", ""))
	require.NoError(t, repo.Deduct(ctx, "cust-1", 200, domain.EntryTypeVMPurchase, "vm purchase"))

	entries, err := repo.GetLedger(ctx, "cust-1")
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	// newest first
	assert.Equal(t, int64(-200), entries[0].AmountCents)
	assert.Equal(t, int64(1000), entries[1].AmountCents)
}
