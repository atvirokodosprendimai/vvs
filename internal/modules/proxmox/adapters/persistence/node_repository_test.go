package persistence_test

import (
	"context"
	"testing"

	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/adapters/persistence"
	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
	proxmoxmigrations "github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/migrations"
	"github.com/atvirokodosprendimai/vvs/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupNodeRepo(t *testing.T, encKey ...[]byte) domain.NodeRepository {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, proxmoxmigrations.FS, "goose_proxmox")
	return persistence.NewGormNodeRepository(db, encKey...)
}

func newTestNode(t *testing.T) *domain.ProxmoxNode {
	t.Helper()
	n, err := domain.NewProxmoxNode("pve-01", "pve", "192.168.1.10", 8006, "root@pam", "vvs", "super-secret", "notes", false)
	require.NoError(t, err)
	return n
}

func TestNodeRepository_SaveAndFindByID(t *testing.T) {
	repo := setupNodeRepo(t)
	ctx := context.Background()
	node := newTestNode(t)

	require.NoError(t, repo.Save(ctx, node))

	found, err := repo.FindByID(ctx, node.ID)
	require.NoError(t, err)
	assert.Equal(t, node.ID, found.ID)
	assert.Equal(t, "pve-01", found.Name)
	assert.Equal(t, "super-secret", found.TokenSecret)
}

func TestNodeRepository_TokenSecretEncryptedAtRest(t *testing.T) {
	// Use a 32-byte encryption key.
	encKey := make([]byte, 32)
	for i := range encKey {
		encKey[i] = byte(i + 1)
	}
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, proxmoxmigrations.FS, "goose_proxmox")

	repo := persistence.NewGormNodeRepository(db, encKey)
	ctx := context.Background()
	node := newTestNode(t)

	require.NoError(t, repo.Save(ctx, node))

	// Read raw value directly from DB — must not be plaintext.
	var rawSecret string
	err := db.W.Raw("SELECT token_secret FROM proxmox_nodes WHERE id = ?", node.ID).Scan(&rawSecret).Error
	require.NoError(t, err)
	assert.NotEqual(t, "super-secret", rawSecret, "token secret must be encrypted at rest")
	assert.NotEmpty(t, rawSecret)

	// But decrypted value via repo must be the original.
	found, err := repo.FindByID(ctx, node.ID)
	require.NoError(t, err)
	assert.Equal(t, "super-secret", found.TokenSecret)
}

func TestNodeRepository_FindAll(t *testing.T) {
	repo := setupNodeRepo(t)
	ctx := context.Background()

	n1, _ := domain.NewProxmoxNode("pve-01", "pve", "192.168.1.1", 0, "root@pam", "vvs", "s1", "", false)
	n2, _ := domain.NewProxmoxNode("pve-02", "pve", "192.168.1.2", 0, "root@pam", "vvs", "s2", "", false)
	require.NoError(t, repo.Save(ctx, n1))
	require.NoError(t, repo.Save(ctx, n2))

	all, err := repo.FindAll(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestNodeRepository_Delete(t *testing.T) {
	repo := setupNodeRepo(t)
	ctx := context.Background()
	node := newTestNode(t)
	require.NoError(t, repo.Save(ctx, node))

	require.NoError(t, repo.Delete(ctx, node.ID))

	_, err := repo.FindByID(ctx, node.ID)
	assert.ErrorIs(t, err, domain.ErrNodeNotFound)
}

func TestNodeRepository_FindByID_NotFound(t *testing.T) {
	repo := setupNodeRepo(t)
	_, err := repo.FindByID(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, domain.ErrNodeNotFound)
}
