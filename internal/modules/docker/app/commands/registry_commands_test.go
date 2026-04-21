package commands_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/adapters/persistence"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	dockermigrations "github.com/atvirokodosprendimai/vvs/internal/modules/docker/migrations"
	"github.com/atvirokodosprendimai/vvs/internal/testutil"
)

func setupRegistryCmd(t *testing.T) (
	*commands.CreateRegistryHandler,
	*commands.UpdateRegistryHandler,
	*commands.DeleteRegistryHandler,
	domain.ContainerRegistryRepository,
) {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, dockermigrations.FS, "goose_docker")
	encKey := []byte("test-encryption-key-32-bytes!!!!")
	repo := persistence.NewGormContainerRegistryRepository(db, encKey)
	return commands.NewCreateRegistryHandler(repo),
		commands.NewUpdateRegistryHandler(repo),
		commands.NewDeleteRegistryHandler(repo),
		repo
}

func TestCreateRegistryHandler_Valid(t *testing.T) {
	create, _, _, repo := setupRegistryCmd(t)
	ctx := context.Background()

	reg, err := create.Handle(ctx, commands.CreateRegistryCommand{
		Name:     "Production Registry",
		URL:      "registry.prod.example.com",
		Username: "deploy",
		Password: "s3cret",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, reg.ID)
	assert.Equal(t, "Production Registry", reg.Name)

	// Verify persisted
	got, err := repo.FindByID(ctx, reg.ID)
	require.NoError(t, err)
	assert.Equal(t, "deploy", got.Username)
	assert.Equal(t, "s3cret", got.Password)
}

func TestCreateRegistryHandler_MissingName(t *testing.T) {
	create, _, _, _ := setupRegistryCmd(t)
	_, err := create.Handle(context.Background(), commands.CreateRegistryCommand{
		URL:      "r.io",
		Username: "u",
		Password: "p",
	})
	assert.ErrorIs(t, err, domain.ErrRegistryNameRequired)
}

func TestCreateRegistryHandler_MissingURL(t *testing.T) {
	create, _, _, _ := setupRegistryCmd(t)
	_, err := create.Handle(context.Background(), commands.CreateRegistryCommand{
		Name:     "Reg",
		Username: "u",
		Password: "p",
	})
	assert.ErrorIs(t, err, domain.ErrRegistryURLRequired)
}

func TestUpdateRegistryHandler_Valid(t *testing.T) {
	create, update, _, repo := setupRegistryCmd(t)
	ctx := context.Background()

	reg, _ := create.Handle(ctx, commands.CreateRegistryCommand{
		Name: "Old", URL: "old.io", Username: "u", Password: "p",
	})

	err := update.Handle(ctx, commands.UpdateRegistryCommand{
		ID:       reg.ID,
		Name:     "New",
		URL:      "new.io",
		Username: "newuser",
		Password: "newpass",
	})
	require.NoError(t, err)

	got, _ := repo.FindByID(ctx, reg.ID)
	assert.Equal(t, "New", got.Name)
	assert.Equal(t, "newuser", got.Username)
	assert.Equal(t, "newpass", got.Password)
}

func TestUpdateRegistryHandler_KeepsPasswordIfEmpty(t *testing.T) {
	create, update, _, repo := setupRegistryCmd(t)
	ctx := context.Background()

	reg, _ := create.Handle(ctx, commands.CreateRegistryCommand{
		Name: "Reg", URL: "r.io", Username: "u", Password: "original",
	})

	err := update.Handle(ctx, commands.UpdateRegistryCommand{
		ID: reg.ID, Name: "Reg2", URL: "r.io", Username: "u", Password: "",
	})
	require.NoError(t, err)

	got, _ := repo.FindByID(ctx, reg.ID)
	assert.Equal(t, "Reg2", got.Name)
	assert.Equal(t, "original", got.Password)
}

func TestUpdateRegistryHandler_NotFound(t *testing.T) {
	_, update, _, _ := setupRegistryCmd(t)
	err := update.Handle(context.Background(), commands.UpdateRegistryCommand{
		ID: "nonexistent", Name: "x", URL: "y",
	})
	assert.ErrorIs(t, err, domain.ErrRegistryNotFound)
}

func TestDeleteRegistryHandler_Valid(t *testing.T) {
	create, _, delete, repo := setupRegistryCmd(t)
	ctx := context.Background()

	reg, _ := create.Handle(ctx, commands.CreateRegistryCommand{
		Name: "Reg", URL: "r.io", Username: "u", Password: "p",
	})

	require.NoError(t, delete.Handle(ctx, reg.ID))

	_, err := repo.FindByID(ctx, reg.ID)
	assert.ErrorIs(t, err, domain.ErrRegistryNotFound)
}
