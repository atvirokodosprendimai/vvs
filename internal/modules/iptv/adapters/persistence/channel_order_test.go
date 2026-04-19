package persistence_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vvs/isp/internal/modules/iptv/adapters/persistence"
	"github.com/vvs/isp/internal/modules/iptv/domain"
	iptvmigrations "github.com/vvs/isp/internal/modules/iptv/migrations"
	"github.com/vvs/isp/internal/testutil"
)

func setupIPTVDB(t *testing.T) (*persistence.ChannelRepository, *persistence.PackageRepository) {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, iptvmigrations.FS, "goose_iptv")
	return persistence.NewChannelRepository(db), persistence.NewPackageRepository(db)
}

func TestChannel_DVRUrl_SaveAndFind(t *testing.T) {
	chRepo, _ := setupIPTVDB(t)
	ctx := context.Background()

	ch := newTestChannel("ch-1", "BBC One")
	ch.DVRUrl = "http://dvr.example.com/bbc"
	require.NoError(t, chRepo.Save(ctx, ch))

	got, err := chRepo.FindByID(ctx, "ch-1")
	require.NoError(t, err)
	assert.Equal(t, "http://dvr.example.com/bbc", got.DVRUrl)
}

func TestChannel_DVRUrl_EmptyByDefault(t *testing.T) {
	chRepo, _ := setupIPTVDB(t)
	ctx := context.Background()

	ch := newTestChannel("ch-2", "CNN")
	require.NoError(t, chRepo.Save(ctx, ch))

	got, err := chRepo.FindByID(ctx, "ch-2")
	require.NoError(t, err)
	assert.Equal(t, "", got.DVRUrl)
}

func TestPackage_SetChannelOrder_ReturnsInOrder(t *testing.T) {
	chRepo, pkgRepo := setupIPTVDB(t)
	ctx := context.Background()

	// Create 3 channels
	for _, id := range []string{"ch-a", "ch-b", "ch-c"} {
		require.NoError(t, chRepo.Save(ctx, newTestChannel(id, id)))
	}

	// Create package and assign channels
	pkg := newTestPackage("pkg-1")
	require.NoError(t, pkgRepo.Save(ctx, pkg))
	require.NoError(t, pkgRepo.AddChannel(ctx, "pkg-1", "ch-a"))
	require.NoError(t, pkgRepo.AddChannel(ctx, "pkg-1", "ch-b"))
	require.NoError(t, pkgRepo.AddChannel(ctx, "pkg-1", "ch-c"))

	// Set order: c, a, b
	require.NoError(t, pkgRepo.SetChannelOrder(ctx, "pkg-1", []string{"ch-c", "ch-a", "ch-b"}))

	// FindByPackage should respect the order
	channels, err := chRepo.FindByPackage(ctx, "pkg-1")
	require.NoError(t, err)
	require.Len(t, channels, 3)
	assert.Equal(t, "ch-c", channels[0].ID)
	assert.Equal(t, "ch-a", channels[1].ID)
	assert.Equal(t, "ch-b", channels[2].ID)
}

func TestPackage_SetChannelOrder_OnlyKnownChannels(t *testing.T) {
	chRepo, pkgRepo := setupIPTVDB(t)
	ctx := context.Background()

	require.NoError(t, chRepo.Save(ctx, newTestChannel("ch-x", "X")))
	pkg := newTestPackage("pkg-2")
	require.NoError(t, pkgRepo.Save(ctx, pkg))
	require.NoError(t, pkgRepo.AddChannel(ctx, "pkg-2", "ch-x"))

	// SetChannelOrder with an unknown ID — should not error, just skip
	err := pkgRepo.SetChannelOrder(ctx, "pkg-2", []string{"ch-unknown", "ch-x"})
	require.NoError(t, err)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestChannel(id, name string) *domain.Channel {
	return &domain.Channel{ID: id, Name: name, Active: true}
}

func newTestPackage(id string) *domain.Package {
	return &domain.Package{ID: id, Name: id}
}
