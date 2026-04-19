package commands_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/atvirokodosprendimai/vvs/internal/modules/customer/adapters/persistence"
	"github.com/atvirokodosprendimai/vvs/internal/modules/customer/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/customer/domain"
	"github.com/atvirokodosprendimai/vvs/internal/modules/customer/migrations"
	"github.com/atvirokodosprendimai/vvs/internal/testutil"
)

func TestCreateCustomerHandler_Integration(t *testing.T) {
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_customer")
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormCustomerRepository(db)
	handler := commands.NewCreateCustomerHandler(repo, pub, nil)

	cmd := commands.CreateCustomerCommand{
		CompanyName: "Acme Corp",
		ContactName: "John Doe",
		Email:       "john@acme.com",
		Phone:       "+37061234567",
		NetworkZone: "Kaunas",
	}

	customer, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	require.NotNil(t, customer)

	assert.Equal(t, "Acme Corp", customer.CompanyName)
	assert.Equal(t, "John Doe", customer.ContactName)
	assert.Equal(t, "john@acme.com", customer.Email)
	assert.Equal(t, "+37061234567", customer.Phone)
	assert.Equal(t, "Kaunas", customer.NetworkZone)
	assert.Equal(t, domain.StatusActive, customer.Status)
	assert.NotEmpty(t, customer.ID)
	assert.Equal(t, "CLI-00001", customer.Code.String())

	// Verify customer persisted — read it back from DB
	found, err := repo.FindByID(context.Background(), customer.ID)
	require.NoError(t, err)
	assert.Equal(t, customer.CompanyName, found.CompanyName)
	assert.Equal(t, customer.Code.String(), found.Code.String())
}

func TestCreateCustomerHandler_MissingCompanyName(t *testing.T) {
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_customer")
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormCustomerRepository(db)
	handler := commands.NewCreateCustomerHandler(repo, pub, nil)

	cmd := commands.CreateCustomerCommand{
		CompanyName: "",
		ContactName: "Jane",
	}

	customer, err := handler.Handle(context.Background(), cmd)
	assert.ErrorIs(t, err, domain.ErrCompanyNameRequired)
	assert.Nil(t, customer)
}

func TestCreateCustomerHandler_SequentialCodes(t *testing.T) {
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_customer")
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormCustomerRepository(db)
	handler := commands.NewCreateCustomerHandler(repo, pub, nil)

	c1, err := handler.Handle(context.Background(), commands.CreateCustomerCommand{CompanyName: "First"})
	require.NoError(t, err)
	assert.Equal(t, "CLI-00001", c1.Code.String())

	c2, err := handler.Handle(context.Background(), commands.CreateCustomerCommand{CompanyName: "Second"})
	require.NoError(t, err)
	assert.Equal(t, "CLI-00002", c2.Code.String())
}
