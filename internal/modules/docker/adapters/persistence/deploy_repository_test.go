package persistence_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/adapters/persistence"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	dockermigrations "github.com/atvirokodosprendimai/vvs/internal/modules/docker/migrations"
	"github.com/atvirokodosprendimai/vvs/internal/testutil"
)

var testEncKey = []byte("test-encryption-key-32-bytes!!!!") // 32 bytes

func setupDeployDB(t *testing.T) (
	*persistence.GormContainerRegistryRepository,
	*persistence.GormVVSDeploymentRepository,
) {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, dockermigrations.FS, "goose_docker")
	return persistence.NewGormContainerRegistryRepository(db, testEncKey),
		persistence.NewGormVVSDeploymentRepository(db)
}

// ── ContainerRegistry ─────────────────────────────────────────────────────────

func TestContainerRegistryRepository_SaveAndFind(t *testing.T) {
	repo, _ := setupDeployDB(t)
	ctx := context.Background()

	reg, err := domain.NewContainerRegistry("My Reg", "registry.example.com", "user", "secret")
	require.NoError(t, err)

	require.NoError(t, repo.Save(ctx, reg))

	got, err := repo.FindByID(ctx, reg.ID)
	require.NoError(t, err)
	assert.Equal(t, reg.ID, got.ID)
	assert.Equal(t, "My Reg", got.Name)
	assert.Equal(t, "registry.example.com", got.URL)
	assert.Equal(t, "user", got.Username)
	assert.Equal(t, "secret", got.Password) // decrypted back
}

func TestContainerRegistryRepository_PasswordEncryptedAtRest(t *testing.T) {
	// Verify that a repo created without the encKey cannot decrypt the password
	// by checking that the stored bytes are NOT the plaintext password.
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, dockermigrations.FS, "goose_docker")

	repo := persistence.NewGormContainerRegistryRepository(db, testEncKey)
	ctx := context.Background()

	reg, _ := domain.NewContainerRegistry("Reg", "r.io", "u", "mysecret")
	require.NoError(t, repo.Save(ctx, reg))

	// Read raw bytes directly via GORM to confirm they differ from plaintext
	type raw struct {
		Password []byte
	}
	var r raw
	require.NoError(t, db.R.Raw("SELECT password FROM container_registries WHERE id = ?", reg.ID).Scan(&r).Error)
	assert.NotEqual(t, []byte("mysecret"), r.Password, "password must not be stored as plaintext")
}

func TestContainerRegistryRepository_Update(t *testing.T) {
	repo, _ := setupDeployDB(t)
	ctx := context.Background()

	reg, _ := domain.NewContainerRegistry("Old", "old.io", "olduser", "oldpass")
	require.NoError(t, repo.Save(ctx, reg))

	reg.Update("New", "new.io", "newuser", "newpass")
	require.NoError(t, repo.Save(ctx, reg))

	got, err := repo.FindByID(ctx, reg.ID)
	require.NoError(t, err)
	assert.Equal(t, "New", got.Name)
	assert.Equal(t, "newuser", got.Username)
	assert.Equal(t, "newpass", got.Password)
}

func TestContainerRegistryRepository_Delete(t *testing.T) {
	repo, _ := setupDeployDB(t)
	ctx := context.Background()

	reg, _ := domain.NewContainerRegistry("Reg", "r.io", "u", "p")
	require.NoError(t, repo.Save(ctx, reg))
	require.NoError(t, repo.Delete(ctx, reg.ID))

	_, err := repo.FindByID(ctx, reg.ID)
	assert.ErrorIs(t, err, domain.ErrRegistryNotFound)
}

func TestContainerRegistryRepository_FindAll(t *testing.T) {
	repo, _ := setupDeployDB(t)
	ctx := context.Background()

	for i, name := range []string{"Z Reg", "A Reg", "M Reg"} {
		reg, _ := domain.NewContainerRegistry(name, "r.io", "u", "p"+string(rune('0'+i)))
		require.NoError(t, repo.Save(ctx, reg))
	}

	all, err := repo.FindAll(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 3)
	// sorted by name ASC
	assert.Equal(t, "A Reg", all[0].Name)
	assert.Equal(t, "M Reg", all[1].Name)
	assert.Equal(t, "Z Reg", all[2].Name)
}

func TestContainerRegistryRepository_FindAll_Empty(t *testing.T) {
	repo, _ := setupDeployDB(t)
	all, err := repo.FindAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, all)
}

// ── VVSDeployment ─────────────────────────────────────────────────────────────

func TestVVSDeploymentRepository_SaveAndFind(t *testing.T) {
	_, repo := setupDeployDB(t)
	ctx := context.Background()

	dep, err := domain.NewVVSDeployment(
		"cluster-1", "node-1",
		domain.VVSComponentPortal, domain.VVSDeployImage,
		"nats://10.0.0.1:4222", 8080,
	)
	require.NoError(t, err)
	dep.ImageURL = "registry.example.com/vvs-portal:latest"
	dep.RegistryID = "reg-1"

	require.NoError(t, repo.Save(ctx, dep))

	got, err := repo.FindByID(ctx, dep.ID)
	require.NoError(t, err)
	assert.Equal(t, dep.ID, got.ID)
	assert.Equal(t, domain.VVSComponentPortal, got.Component)
	assert.Equal(t, domain.VVSDeployImage, got.Source)
	assert.Equal(t, "registry.example.com/vvs-portal:latest", got.ImageURL)
	assert.Equal(t, "reg-1", got.RegistryID)
	assert.Equal(t, "nats://10.0.0.1:4222", got.NATSUrl)
	assert.Equal(t, 8080, got.Port)
	assert.Equal(t, domain.VVSDeploymentPending, got.Status)
	assert.Nil(t, got.LastDeployedAt)
}

func TestVVSDeploymentRepository_StatusUpdate(t *testing.T) {
	_, repo := setupDeployDB(t)
	ctx := context.Background()

	dep, _ := domain.NewVVSDeployment("c", "n", domain.VVSComponentSTB, domain.VVSDeployGit, "nats://x:4222", 0)
	dep.GitURL = "https://github.com/org/vvs-stb.git"
	dep.GitRef = "v1.2.3"
	require.NoError(t, repo.Save(ctx, dep))

	dep.MarkRunning()
	require.NoError(t, repo.Save(ctx, dep))

	got, err := repo.FindByID(ctx, dep.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.VVSDeploymentRunning, got.Status)
	assert.NotNil(t, got.LastDeployedAt)
	assert.Empty(t, got.ErrorMsg)
}

func TestVVSDeploymentRepository_ErrorStatus(t *testing.T) {
	_, repo := setupDeployDB(t)
	ctx := context.Background()

	dep, _ := domain.NewVVSDeployment("c", "n", domain.VVSComponentPortal, domain.VVSDeployImage, "nats://x:4222", 0)
	require.NoError(t, repo.Save(ctx, dep))

	dep.MarkError("docker pull failed: image not found")
	require.NoError(t, repo.Save(ctx, dep))

	got, _ := repo.FindByID(ctx, dep.ID)
	assert.Equal(t, domain.VVSDeploymentError, got.Status)
	assert.Equal(t, "docker pull failed: image not found", got.ErrorMsg)
}

func TestVVSDeploymentRepository_EnvVars(t *testing.T) {
	_, repo := setupDeployDB(t)
	ctx := context.Background()

	dep, _ := domain.NewVVSDeployment("c", "n", domain.VVSComponentPortal, domain.VVSDeployImage, "nats://x:4222", 0)
	dep.EnvVars = map[string]string{
		"LOG_LEVEL": "debug",
		"BASE_URL":  "https://portal.example.com",
	}
	require.NoError(t, repo.Save(ctx, dep))

	got, _ := repo.FindByID(ctx, dep.ID)
	assert.Equal(t, "debug", got.EnvVars["LOG_LEVEL"])
	assert.Equal(t, "https://portal.example.com", got.EnvVars["BASE_URL"])
}

func TestVVSDeploymentRepository_Delete(t *testing.T) {
	_, repo := setupDeployDB(t)
	ctx := context.Background()

	dep, _ := domain.NewVVSDeployment("c", "n", domain.VVSComponentPortal, domain.VVSDeployImage, "nats://x:4222", 0)
	require.NoError(t, repo.Save(ctx, dep))
	require.NoError(t, repo.Delete(ctx, dep.ID))

	_, err := repo.FindByID(ctx, dep.ID)
	assert.ErrorIs(t, err, domain.ErrDeploymentNotFound)
}

func TestVVSDeploymentRepository_FindAll_OrderedByCreatedDesc(t *testing.T) {
	_, repo := setupDeployDB(t)
	ctx := context.Background()

	for _, comp := range []domain.VVSComponentType{
		domain.VVSComponentPortal,
		domain.VVSComponentSTB,
		domain.VVSComponentPortal,
	} {
		dep, _ := domain.NewVVSDeployment("c", "n", comp, domain.VVSDeployImage, "nats://x:4222", 0)
		require.NoError(t, repo.Save(ctx, dep))
	}

	all, err := repo.FindAll(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 3)
	// newest first
	for i := 1; i < len(all); i++ {
		assert.False(t, all[i].CreatedAt.After(all[i-1].CreatedAt))
	}
}
