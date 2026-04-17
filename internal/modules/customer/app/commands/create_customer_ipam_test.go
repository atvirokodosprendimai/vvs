package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vvs/isp/internal/modules/customer/adapters/persistence"
	"github.com/vvs/isp/internal/modules/customer/app/commands"
	"github.com/vvs/isp/internal/modules/customer/migrations"
	"github.com/vvs/isp/internal/testutil"
)

// stubIPAM is a minimal in-test implementation of commands.IPAllocator.
type stubIPAM struct {
	ip  string
	id  int
	err error
}

func (s *stubIPAM) AllocateIP(_ context.Context, _, _ string) (string, int, error) {
	return s.ip, s.id, s.err
}

func setupCreateWithIPAM(t *testing.T, ipam commands.IPAllocator) (*persistence.GormCustomerRepository, *commands.CreateCustomerHandler) {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_customer")
	pub, _ := testutil.NewTestNATS(t)
	repo := persistence.NewGormCustomerRepository(db)
	handler := commands.NewCreateCustomerHandler(repo, pub, ipam)
	return repo, handler
}

// ---------------------------------------------------------------------------
// CreateCustomerHandler — ipam branch
// ---------------------------------------------------------------------------

func TestCreateCustomerHandler_IPAM_AllocatesIP(t *testing.T) {
	ipam := &stubIPAM{ip: "10.0.1.5", id: 42}
	repo, handler := setupCreateWithIPAM(t, ipam)

	customer, err := handler.Handle(context.Background(), commands.CreateCustomerCommand{
		CompanyName: "IPAM Corp",
		NetworkZone: "Kaunas",
	})
	require.NoError(t, err)
	require.NotNil(t, customer)

	// Verify allocated IP is persisted
	found, err := repo.FindByID(context.Background(), customer.ID)
	require.NoError(t, err)
	assert.Equal(t, "10.0.1.5", found.IPAddress)
}

func TestCreateCustomerHandler_IPAM_ErrorIsBestEffort(t *testing.T) {
	// IPAM failure must not block customer creation
	ipam := &stubIPAM{err: errors.New("netbox unavailable")}
	_, handler := setupCreateWithIPAM(t, ipam)

	customer, err := handler.Handle(context.Background(), commands.CreateCustomerCommand{
		CompanyName: "IPAM Fail Corp",
		NetworkZone: "Vilnius",
	})
	require.NoError(t, err)
	require.NotNil(t, customer)
	// IP should be empty since allocation failed
	assert.Empty(t, customer.IPAddress)
}

func TestCreateCustomerHandler_IPAM_EmptyIPSkipsSave(t *testing.T) {
	// IPAM returns empty string — no second save should occur
	ipam := &stubIPAM{ip: "", id: 0}
	repo, handler := setupCreateWithIPAM(t, ipam)

	customer, err := handler.Handle(context.Background(), commands.CreateCustomerCommand{
		CompanyName: "No IP Corp",
		NetworkZone: "Kaunas",
	})
	require.NoError(t, err)
	require.NotNil(t, customer)

	found, err := repo.FindByID(context.Background(), customer.ID)
	require.NoError(t, err)
	assert.Empty(t, found.IPAddress)
}
