package persistence_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/adapters/persistence"
	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
	authmigrations "github.com/atvirokodosprendimai/vvs/internal/modules/auth/migrations"
	"github.com/atvirokodosprendimai/vvs/internal/testutil"
)

func setupRepos(t *testing.T) (domain.UserRepository, domain.RoleRepository) {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, authmigrations.FS, "goose_auth")
	return persistence.NewGormUserRepository(db), persistence.NewGormRoleRepository(db)
}

func setupRoleRepo(t *testing.T) domain.RoleRepository {
	_, roleRepo := setupRepos(t)
	return roleRepo
}

func TestGormRoleRepository_ListReturnsBuiltins(t *testing.T) {
	repo := setupRoleRepo(t)
	roles, err := repo.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, roles, 3)
	names := make([]string, len(roles))
	for i, r := range roles {
		names[i] = string(r.Name)
	}
	assert.ElementsMatch(t, []string{"admin", "operator", "viewer"}, names)
}

func TestGormRoleRepository_SaveAndFindCustomRole(t *testing.T) {
	repo := setupRoleRepo(t)
	ctx := context.Background()

	rd := &domain.RoleDefinition{
		Name:        "billing",
		DisplayName: "Billing Team",
		IsBuiltin:   false,
		CanWrite:    true,
	}
	require.NoError(t, repo.Save(ctx, rd))

	got, err := repo.FindByName(ctx, "billing")
	require.NoError(t, err)
	assert.Equal(t, domain.Role("billing"), got.Name)
	assert.Equal(t, "Billing Team", got.DisplayName)
	assert.True(t, got.CanWrite)
	assert.False(t, got.IsBuiltin)
}

func TestGormRoleRepository_Delete_RejectsBuiltin(t *testing.T) {
	repo := setupRoleRepo(t)
	err := repo.Delete(context.Background(), domain.RoleAdmin)
	assert.ErrorIs(t, err, domain.ErrRoleBuiltin)
}

func TestGormRoleRepository_Delete_CustomRole(t *testing.T) {
	repo := setupRoleRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Save(ctx, &domain.RoleDefinition{Name: "temp", CanWrite: false}))
	require.NoError(t, repo.Delete(ctx, "temp"))

	_, err := repo.FindByName(ctx, "temp")
	assert.ErrorIs(t, err, domain.ErrRoleNotFound)
}

func TestGormRoleRepository_FindByName_NotFound(t *testing.T) {
	repo := setupRoleRepo(t)
	_, err := repo.FindByName(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, domain.ErrRoleNotFound)
}

// TestUser_IsWriteRole_PopulatedByJoin verifies that GormUserRepository populates
// User.IsWriteRole from the roles table JOIN.
func TestUser_IsWriteRole_PopulatedByJoin(t *testing.T) {
	ctx := context.Background()
	userRepo, roleRepo := setupRepos(t)

	// Create a custom role with can_write=true
	require.NoError(t, roleRepo.Save(ctx, &domain.RoleDefinition{
		Name:     "billing",
		CanWrite: true,
	}))

	// Create user with custom role
	u, err := domain.NewUser("finance_user", "password123", domain.Role("billing"))
	require.NoError(t, err)
	require.NoError(t, userRepo.Save(ctx, u))

	got, err := userRepo.FindByID(ctx, u.ID)
	require.NoError(t, err)
	assert.True(t, got.IsWriteRole, "custom role with can_write=true should set IsWriteRole")
	assert.True(t, got.CanWrite())

	// Create user with viewer (can_write=false)
	v, err := domain.NewUser("viewer_user", "password123", domain.RoleViewer)
	require.NoError(t, err)
	require.NoError(t, userRepo.Save(ctx, v))

	gotViewer, err := userRepo.FindByID(ctx, v.ID)
	require.NoError(t, err)
	assert.False(t, gotViewer.IsWriteRole, "viewer role should set IsWriteRole=false")
	assert.False(t, gotViewer.CanWrite())
}
