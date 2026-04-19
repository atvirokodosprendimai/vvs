package commands_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/atvirokodosprendimai/vvs/internal/modules/product/adapters/persistence"
	"github.com/atvirokodosprendimai/vvs/internal/modules/product/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/product/domain"
	"github.com/atvirokodosprendimai/vvs/internal/modules/product/migrations"
	"github.com/atvirokodosprendimai/vvs/internal/testutil"
)

func TestCreateProductHandler_HappyPath(t *testing.T) {
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_product")
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormProductRepository(db)
	handler := commands.NewCreateProductHandler(repo, pub)

	cmd := commands.CreateProductCommand{
		Name:          "Fiber 100",
		Description:   "100 Mbps symmetric fiber",
		Type:          "internet",
		PriceAmount:   2999,
		PriceCurrency: "EUR",
		BillingPeriod: "monthly",
	}

	product, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	require.NotNil(t, product)

	assert.Equal(t, "Fiber 100", product.Name)
	assert.Equal(t, "100 Mbps symmetric fiber", product.Description)
	assert.Equal(t, domain.TypeInternet, product.Type)
	assert.Equal(t, int64(2999), product.Price.Amount)
	assert.Equal(t, domain.BillingMonthly, product.BillingPeriod)
	assert.True(t, product.IsActive)
	assert.NotEmpty(t, product.ID)

	// Verify persisted — read back from DB.
	found, err := repo.FindByID(context.Background(), product.ID)
	require.NoError(t, err)
	assert.Equal(t, product.Name, found.Name)
	assert.Equal(t, product.Description, found.Description)
	assert.Equal(t, product.Price.Amount, found.Price.Amount)
	assert.Equal(t, product.BillingPeriod, found.BillingPeriod)
}

func TestCreateProductHandler_EmptyName(t *testing.T) {
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_product")
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormProductRepository(db)
	handler := commands.NewCreateProductHandler(repo, pub)

	cmd := commands.CreateProductCommand{
		Name:          "",
		Type:          "internet",
		PriceCurrency: "EUR",
		BillingPeriod: "monthly",
	}

	product, err := handler.Handle(context.Background(), cmd)
	assert.ErrorIs(t, err, domain.ErrNameRequired)
	assert.Nil(t, product)
}

func TestUpdateProductHandler_HappyPath(t *testing.T) {
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_product")
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormProductRepository(db)
	createHandler := commands.NewCreateProductHandler(repo, pub)
	updateHandler := commands.NewUpdateProductHandler(repo, pub)

	// Create a product first.
	product, err := createHandler.Handle(context.Background(), commands.CreateProductCommand{
		Name:          "VoIP Basic",
		Type:          "voip",
		PriceAmount:   999,
		PriceCurrency: "EUR",
		BillingPeriod: "monthly",
	})
	require.NoError(t, err)

	// Update it.
	err = updateHandler.Handle(context.Background(), commands.UpdateProductCommand{
		ID:            product.ID,
		Name:          "VoIP Pro",
		Description:   "Advanced VoIP",
		Type:          "voip",
		PriceAmount:   1499,
		PriceCurrency: "EUR",
		BillingPeriod: "yearly",
	})
	require.NoError(t, err)

	// Verify new values persisted.
	found, err := repo.FindByID(context.Background(), product.ID)
	require.NoError(t, err)
	assert.Equal(t, "VoIP Pro", found.Name)
	assert.Equal(t, "Advanced VoIP", found.Description)
	assert.Equal(t, int64(1499), found.Price.Amount)
	assert.Equal(t, domain.BillingYearly, found.BillingPeriod)
}

func TestDeleteProductHandler_HappyPath(t *testing.T) {
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_product")
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormProductRepository(db)
	createHandler := commands.NewCreateProductHandler(repo, pub)
	deleteHandler := commands.NewDeleteProductHandler(repo, pub)

	// Create a product first.
	product, err := createHandler.Handle(context.Background(), commands.CreateProductCommand{
		Name:          "Hosting Basic",
		Type:          "hosting",
		PriceAmount:   499,
		PriceCurrency: "EUR",
		BillingPeriod: "monthly",
	})
	require.NoError(t, err)

	// Delete it.
	err = deleteHandler.Handle(context.Background(), commands.DeleteProductCommand{ID: product.ID})
	require.NoError(t, err)

	// Verify gone — FindByID must return ErrProductNotFound.
	_, err = repo.FindByID(context.Background(), product.ID)
	assert.ErrorIs(t, err, domain.ErrProductNotFound)
}
