package commands_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vvs/isp/internal/modules/customer/adapters/persistence"
	"github.com/vvs/isp/internal/modules/customer/app/commands"
	"github.com/vvs/isp/internal/modules/customer/domain"
	"github.com/vvs/isp/internal/modules/customer/migrations"
	"github.com/vvs/isp/internal/testutil"
)

func setupCustomerHandlers(t *testing.T) (
	*persistence.GormCustomerRepository,
	*commands.CreateCustomerHandler,
	*commands.UpdateCustomerHandler,
	*commands.DeleteCustomerHandler,
	*commands.ChangeCustomerStatusHandler,
	*commands.AddNoteHandler,
) {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_customer")
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormCustomerRepository(db)
	noteRepo := persistence.NewGormNoteRepository(db)

	create := commands.NewCreateCustomerHandler(repo, pub, nil)
	update := commands.NewUpdateCustomerHandler(repo, pub)
	del := commands.NewDeleteCustomerHandler(repo, pub)
	status := commands.NewChangeCustomerStatusHandler(repo, pub)
	note := commands.NewAddNoteHandler(noteRepo)

	return repo, create, update, del, status, note
}

func createTestCustomer(t *testing.T, create *commands.CreateCustomerHandler) *domain.Customer {
	t.Helper()
	c, err := create.Handle(context.Background(), commands.CreateCustomerCommand{
		CompanyName: "Test Corp",
		ContactName: "Test User",
		Email:       "test@corp.com",
		Phone:       "+37060000001",
		NetworkZone: "Vilnius",
	})
	require.NoError(t, err)
	return c
}

// ---------------------------------------------------------------------------
// UpdateCustomerHandler
// ---------------------------------------------------------------------------

func TestUpdateCustomerHandler_HappyPath(t *testing.T) {
	repo, create, update, _, _, _ := setupCustomerHandlers(t)

	customer := createTestCustomer(t, create)

	err := update.Handle(context.Background(), commands.UpdateCustomerCommand{
		ID:          customer.ID,
		CompanyName: "Updated Corp",
		ContactName: "New Name",
		Email:       "new@corp.com",
		Phone:       "+37060000002",
		Street:      "Gedimino pr. 1",
		City:        "Vilnius",
		PostalCode:  "01103",
		Country:     "LT",
		TaxID:       "LT123456789",
		Notes:       "VIP client",
		NetworkZone: "Kaunas",
	})

	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), customer.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Corp", found.CompanyName)
	assert.Equal(t, "New Name", found.ContactName)
	assert.Equal(t, "new@corp.com", found.Email)
	assert.Equal(t, "Gedimino pr. 1", found.Street)
	assert.Equal(t, "Vilnius", found.City)
	assert.Equal(t, "LT123456789", found.TaxID)
	assert.Equal(t, "VIP client", found.Notes)
	assert.Equal(t, "Kaunas", found.NetworkZone)
}

func TestUpdateCustomerHandler_NetworkInfo(t *testing.T) {
	repo, create, update, _, _, _ := setupCustomerHandlers(t)

	customer := createTestCustomer(t, create)
	routerID := "router-42"

	err := update.Handle(context.Background(), commands.UpdateCustomerCommand{
		ID:          customer.ID,
		CompanyName: customer.CompanyName,
		RouterID:    routerID,
		IPAddress:   "10.0.1.55",
		MACAddress:  "AA:BB:CC:DD:EE:FF",
	})

	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), customer.ID)
	require.NoError(t, err)
	require.NotNil(t, found.RouterID)
	assert.Equal(t, routerID, *found.RouterID)
	assert.Equal(t, "10.0.1.55", found.IPAddress)
	assert.Equal(t, "AA:BB:CC:DD:EE:FF", found.MACAddress)
	assert.True(t, found.HasNetworkProvisioning())
}

func TestUpdateCustomerHandler_ClearNetworkInfo(t *testing.T) {
	repo, create, update, _, _, _ := setupCustomerHandlers(t)

	customer := createTestCustomer(t, create)
	routerID := "router-42"

	// First set network info
	err := update.Handle(context.Background(), commands.UpdateCustomerCommand{
		ID:          customer.ID,
		CompanyName: customer.CompanyName,
		RouterID:    routerID,
		IPAddress:   "10.0.1.55",
		MACAddress:  "AA:BB:CC:DD:EE:FF",
	})
	require.NoError(t, err)

	// Now clear it
	err = update.Handle(context.Background(), commands.UpdateCustomerCommand{
		ID:          customer.ID,
		CompanyName: customer.CompanyName,
		RouterID:    "", // empty = clear
	})
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), customer.ID)
	require.NoError(t, err)
	assert.Nil(t, found.RouterID)
	assert.False(t, found.HasNetworkProvisioning())
}

func TestUpdateCustomerHandler_MissingCompanyName(t *testing.T) {
	_, create, update, _, _, _ := setupCustomerHandlers(t)

	customer := createTestCustomer(t, create)

	err := update.Handle(context.Background(), commands.UpdateCustomerCommand{
		ID:          customer.ID,
		CompanyName: "",
	})

	assert.ErrorIs(t, err, domain.ErrCompanyNameRequired)
}

func TestUpdateCustomerHandler_NotFound(t *testing.T) {
	_, _, update, _, _, _ := setupCustomerHandlers(t)

	err := update.Handle(context.Background(), commands.UpdateCustomerCommand{
		ID:          "nonexistent-id",
		CompanyName: "Corp",
	})

	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// DeleteCustomerHandler
// ---------------------------------------------------------------------------

func TestDeleteCustomerHandler_HappyPath(t *testing.T) {
	repo, create, _, del, _, _ := setupCustomerHandlers(t)

	customer := createTestCustomer(t, create)

	err := del.Handle(context.Background(), commands.DeleteCustomerCommand{ID: customer.ID})
	require.NoError(t, err)

	_, err = repo.FindByID(context.Background(), customer.ID)
	assert.ErrorIs(t, err, domain.ErrCustomerNotFound)
}

func TestDeleteCustomerHandler_NotFound(t *testing.T) {
	_, _, _, del, _, _ := setupCustomerHandlers(t)

	err := del.Handle(context.Background(), commands.DeleteCustomerCommand{ID: "nonexistent-id"})
	assert.Error(t, err)
}
