package commands_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	dealmigrations "github.com/atvirokodosprendimai/vvs/internal/modules/deal/migrations"
	customermigrations "github.com/atvirokodosprendimai/vvs/internal/modules/customer/migrations"
	"github.com/atvirokodosprendimai/vvs/internal/modules/deal/adapters/persistence"
	"github.com/atvirokodosprendimai/vvs/internal/modules/deal/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/deal/domain"
	"github.com/atvirokodosprendimai/vvs/internal/testutil"
)

// insertDealCustomer inserts a bare-minimum customer row to satisfy the FK constraint.
func insertDealCustomer(t *testing.T, db *gormsqlite.DB, id string) {
	t.Helper()
	err := db.W.Exec(
		"INSERT INTO customers (id, code, company_name, status) VALUES (?, ?, ?, ?)",
		id, "CLI-00001", "Test Corp", "active",
	).Error
	require.NoError(t, err)
}

func setupDealDB(t *testing.T) (*gormsqlite.DB, string) {
	t.Helper()
	db := testutil.NewTestDB(t)
	// Customer table must exist because deals.customer_id FK references it.
	testutil.RunMigrations(t, db, customermigrations.FS, "goose_customer")
	testutil.RunMigrations(t, db, dealmigrations.FS, "goose_deal")
	customerID := "cust-deal-test-001"
	insertDealCustomer(t, db, customerID)
	return db, customerID
}

func TestAddDealHandler_HappyPath(t *testing.T) {
	db, customerID := setupDealDB(t)
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormDealRepository(db)
	handler := commands.NewAddDealHandler(repo, pub)

	cmd := commands.AddDealCommand{
		CustomerID: customerID,
		Title:      "Fiber 1Gbps Upgrade",
		Value:      50000,
		Currency:   "EUR",
		Notes:      "Potential upsell",
	}

	deal, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	require.NotNil(t, deal)

	assert.Equal(t, customerID, deal.CustomerID)
	assert.Equal(t, "Fiber 1Gbps Upgrade", deal.Title)
	assert.Equal(t, int64(50000), deal.Value)
	assert.Equal(t, "EUR", deal.Currency)
	assert.Equal(t, "Potential upsell", deal.Notes)
	assert.Equal(t, domain.StageNew, deal.Stage)
	assert.NotEmpty(t, deal.ID)

	// Verify persisted — read back from DB.
	found, err := repo.FindByID(context.Background(), deal.ID)
	require.NoError(t, err)
	assert.Equal(t, deal.Title, found.Title)
	assert.Equal(t, deal.Value, found.Value)
	assert.Equal(t, domain.StageNew, found.Stage)
}

func TestAddDealHandler_EmptyTitle(t *testing.T) {
	db, customerID := setupDealDB(t)
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormDealRepository(db)
	handler := commands.NewAddDealHandler(repo, pub)

	deal, err := handler.Handle(context.Background(), commands.AddDealCommand{
		CustomerID: customerID,
		Title:      "",
		Value:      10000,
		Currency:   "EUR",
	})
	assert.ErrorIs(t, err, domain.ErrTitleRequired)
	assert.Nil(t, deal)
}

func TestUpdateDealHandler_HappyPath(t *testing.T) {
	db, customerID := setupDealDB(t)
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormDealRepository(db)
	addHandler := commands.NewAddDealHandler(repo, pub)
	updateHandler := commands.NewUpdateDealHandler(repo, pub)

	// Create first.
	deal, err := addHandler.Handle(context.Background(), commands.AddDealCommand{
		CustomerID: customerID,
		Title:      "Initial Deal",
		Value:      20000,
		Currency:   "EUR",
	})
	require.NoError(t, err)

	// Update it.
	err = updateHandler.Handle(context.Background(), commands.UpdateDealCommand{
		ID:       deal.ID,
		Title:    "Revised Deal",
		Value:    35000,
		Currency: "EUR",
		Notes:    "Updated after meeting",
	})
	require.NoError(t, err)

	// Verify new values persisted.
	found, err := repo.FindByID(context.Background(), deal.ID)
	require.NoError(t, err)
	assert.Equal(t, "Revised Deal", found.Title)
	assert.Equal(t, int64(35000), found.Value)
	assert.Equal(t, "Updated after meeting", found.Notes)
}

func TestDeleteDealHandler_HappyPath(t *testing.T) {
	db, customerID := setupDealDB(t)
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormDealRepository(db)
	addHandler := commands.NewAddDealHandler(repo, pub)
	deleteHandler := commands.NewDeleteDealHandler(repo, pub)

	// Create first.
	deal, err := addHandler.Handle(context.Background(), commands.AddDealCommand{
		CustomerID: customerID,
		Title:      "Deal to Delete",
		Value:      5000,
		Currency:   "EUR",
	})
	require.NoError(t, err)

	// Delete it.
	err = deleteHandler.Handle(context.Background(), commands.DeleteDealCommand{ID: deal.ID})
	require.NoError(t, err)

	// Verify gone — FindByID must return ErrNotFound.
	_, err = repo.FindByID(context.Background(), deal.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAdvanceDealHandler_FullPipeline(t *testing.T) {
	db, customerID := setupDealDB(t)
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormDealRepository(db)
	addHandler := commands.NewAddDealHandler(repo, pub)
	advanceHandler := commands.NewAdvanceDealHandler(repo, pub)

	// Create a new deal (stage = "new").
	deal, err := addHandler.Handle(context.Background(), commands.AddDealCommand{
		CustomerID: customerID,
		Title:      "Pipeline Deal",
		Value:      100000,
		Currency:   "EUR",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StageNew, deal.Stage)

	// qualify → StageQualified
	err = advanceHandler.Handle(context.Background(), commands.AdvanceDealCommand{ID: deal.ID, Action: "qualify"})
	require.NoError(t, err)
	found, err := repo.FindByID(context.Background(), deal.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StageQualified, found.Stage)

	// propose → StageProposal
	err = advanceHandler.Handle(context.Background(), commands.AdvanceDealCommand{ID: deal.ID, Action: "propose"})
	require.NoError(t, err)
	found, err = repo.FindByID(context.Background(), deal.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StageProposal, found.Stage)

	// negotiate → StageNegotiation
	err = advanceHandler.Handle(context.Background(), commands.AdvanceDealCommand{ID: deal.ID, Action: "negotiate"})
	require.NoError(t, err)
	found, err = repo.FindByID(context.Background(), deal.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StageNegotiation, found.Stage)

	// win → StageWon
	err = advanceHandler.Handle(context.Background(), commands.AdvanceDealCommand{ID: deal.ID, Action: "win"})
	require.NoError(t, err)
	found, err = repo.FindByID(context.Background(), deal.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StageWon, found.Stage)
}

func TestUpdateDealHandler_NotFound(t *testing.T) {
	db, _ := setupDealDB(t)
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormDealRepository(db)
	handler := commands.NewUpdateDealHandler(repo, pub)

	err := handler.Handle(context.Background(), commands.UpdateDealCommand{
		ID:       "nonexistent-id",
		Title:    "X",
		Value:    1,
		Currency: "EUR",
	})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestDeleteDealHandler_NotFound(t *testing.T) {
	db, _ := setupDealDB(t)
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormDealRepository(db)
	handler := commands.NewDeleteDealHandler(repo, pub)

	err := handler.Handle(context.Background(), commands.DeleteDealCommand{ID: "nonexistent-id"})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAdvanceDealHandler_NotFound(t *testing.T) {
	db, _ := setupDealDB(t)
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormDealRepository(db)
	handler := commands.NewAdvanceDealHandler(repo, pub)

	err := handler.Handle(context.Background(), commands.AdvanceDealCommand{ID: "nonexistent-id", Action: "qualify"})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAdvanceDealHandler_UnknownAction(t *testing.T) {
	db, customerID := setupDealDB(t)
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormDealRepository(db)
	addHandler := commands.NewAddDealHandler(repo, pub)
	advanceHandler := commands.NewAdvanceDealHandler(repo, pub)

	deal, err := addHandler.Handle(context.Background(), commands.AddDealCommand{
		CustomerID: customerID,
		Title:      "Test Deal",
		Value:      1000,
		Currency:   "EUR",
	})
	require.NoError(t, err)

	err = advanceHandler.Handle(context.Background(), commands.AdvanceDealCommand{
		ID:     deal.ID,
		Action: "bogus",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown action")
}
