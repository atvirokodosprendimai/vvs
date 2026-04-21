package queries_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/adapters/persistence"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	dockermigrations "github.com/atvirokodosprendimai/vvs/internal/modules/docker/migrations"
	"github.com/atvirokodosprendimai/vvs/internal/testutil"
)

func setupQueryDB(t *testing.T) (
	domain.ContainerRegistryRepository,
	domain.VVSDeploymentRepository,
) {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, dockermigrations.FS, "goose_docker")
	encKey := []byte("test-encryption-key-32-bytes!!!!")
	return persistence.NewGormContainerRegistryRepository(db, encKey),
		persistence.NewGormVVSDeploymentRepository(db)
}

func TestListRegistriesHandler_Empty(t *testing.T) {
	regRepo, _ := setupQueryDB(t)
	h := queries.NewListRegistriesHandler(regRepo)
	result, err := h.Handle(context.Background())
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestListRegistriesHandler_ReturnsAll(t *testing.T) {
	regRepo, _ := setupQueryDB(t)
	ctx := context.Background()

	createCmd := commands.NewCreateRegistryHandler(regRepo)
	for _, name := range []string{"Reg B", "Reg A", "Reg C"} {
		_, err := createCmd.Handle(ctx, commands.CreateRegistryCommand{
			Name: name, URL: "r.io", Username: "u", Password: "p",
		})
		require.NoError(t, err)
	}

	h := queries.NewListRegistriesHandler(regRepo)
	result, err := h.Handle(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 3)
	// sorted A→C
	assert.Equal(t, "Reg A", result[0].Name)
	assert.Equal(t, "Reg C", result[2].Name)
	// password not in read model
}

func TestListRegistriesHandler_NoPassword(t *testing.T) {
	regRepo, _ := setupQueryDB(t)
	ctx := context.Background()

	createCmd := commands.NewCreateRegistryHandler(regRepo)
	_, _ = createCmd.Handle(ctx, commands.CreateRegistryCommand{
		Name: "Reg", URL: "r.io", Username: "user", Password: "secret",
	})

	h := queries.NewListRegistriesHandler(regRepo)
	result, _ := h.Handle(ctx)
	require.Len(t, result, 1)
	// RegistryReadModel has no Password field — just confirm we get ID/Name/URL/Username
	assert.Equal(t, "Reg", result[0].Name)
	assert.Equal(t, "user", result[0].Username)
}

func TestListVVSDeploymentsHandler(t *testing.T) {
	_, depRepo := setupQueryDB(t)
	ctx := context.Background()

	for _, comp := range []domain.VVSComponentType{
		domain.VVSComponentPortal,
		domain.VVSComponentSTB,
	} {
		dep, _ := domain.NewVVSDeployment("c", "n", comp, domain.VVSDeployImage, "nats://x:4222", 0)
		dep.ImageURL = "img:latest"
		require.NoError(t, depRepo.Save(ctx, dep))
	}

	h := queries.NewListVVSDeploymentsHandler(depRepo)
	result, err := h.Handle(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	// most recent first
	assert.NotEmpty(t, result[0].ID)
}

func TestGetVVSDeploymentHandler(t *testing.T) {
	_, depRepo := setupQueryDB(t)
	ctx := context.Background()

	dep, _ := domain.NewVVSDeployment("cluster-1", "node-1", domain.VVSComponentPortal, domain.VVSDeployGit, "nats://x:4222", 9000)
	dep.GitURL = "https://github.com/org/portal.git"
	dep.GitRef = "v2"
	require.NoError(t, depRepo.Save(ctx, dep))

	dep.MarkRunning()
	require.NoError(t, depRepo.Save(ctx, dep))

	h := queries.NewGetVVSDeploymentHandler(depRepo)
	rm, err := h.Handle(ctx, dep.ID)
	require.NoError(t, err)
	assert.Equal(t, dep.ID, rm.ID)
	assert.Equal(t, "portal", rm.Component)
	assert.Equal(t, "git", rm.Source)
	assert.Equal(t, "https://github.com/org/portal.git", rm.GitURL)
	assert.Equal(t, "v2", rm.GitRef)
	assert.Equal(t, 9000, rm.Port)
	assert.Equal(t, "running", rm.Status)
	assert.NotEmpty(t, rm.LastDeployedAt)
}

func TestGetVVSDeploymentHandler_NotFound(t *testing.T) {
	_, depRepo := setupQueryDB(t)
	h := queries.NewGetVVSDeploymentHandler(depRepo)
	_, err := h.Handle(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, domain.ErrDeploymentNotFound)
}
