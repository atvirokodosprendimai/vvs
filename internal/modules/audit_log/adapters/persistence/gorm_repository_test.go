package persistence_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vvs/isp/internal/modules/audit_log/adapters/persistence"
	"github.com/vvs/isp/internal/modules/audit_log/domain"
	auditlogmigrations "github.com/vvs/isp/internal/modules/audit_log/migrations"
	"github.com/vvs/isp/internal/testutil"
)

func setupRepo(t *testing.T) domain.AuditLogRepository {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, auditlogmigrations.FS, "goose_audit_log")
	return persistence.NewGormAuditLogRepository(db)
}

func TestGormAuditLogRepository_Save(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	al, err := domain.NewAuditLog("user-1", "Alice", "customer.created", "customer", "cust-001", json.RawMessage(`{"name":"Acme"}`))
	require.NoError(t, err)

	err = repo.Save(ctx, al)
	require.NoError(t, err)

	// Verify the entry exists by listing for that resource.
	results, err := repo.ListForResource(ctx, "customer", "cust-001")
	require.NoError(t, err)
	require.Len(t, results, 1)

	got := results[0]
	assert.Equal(t, al.ID, got.ID)
	assert.Equal(t, "user-1", got.ActorID)
	assert.Equal(t, "Alice", got.ActorName)
	assert.Equal(t, "customer.created", got.Action)
	assert.Equal(t, "customer", got.Resource)
	assert.Equal(t, "cust-001", got.ResourceID)
	assert.JSONEq(t, `{"name":"Acme"}`, string(got.Changes))
}

func TestGormAuditLogRepository_ListAll_NoFilter(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	entries := []struct {
		action     string
		resource   string
		resourceID string
	}{
		{"customer.created", "customer", "cust-001"},
		{"ticket.opened", "ticket", "tick-001"},
		{"invoice.finalized", "invoice", "inv-001"},
	}

	for _, e := range entries {
		al, err := domain.NewAuditLog("user-1", "Alice", e.action, e.resource, e.resourceID, nil)
		require.NoError(t, err)
		require.NoError(t, repo.Save(ctx, al))
	}

	results, err := repo.ListAll(ctx, domain.Filter{})
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestGormAuditLogRepository_ListAll_ResourceFilter(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	// Save two customer entries and one ticket entry.
	for _, e := range []struct {
		action, resource, resourceID string
	}{
		{"customer.created", "customer", "cust-001"},
		{"customer.updated", "customer", "cust-002"},
		{"ticket.opened", "ticket", "tick-001"},
	} {
		al, err := domain.NewAuditLog("", "", e.action, e.resource, e.resourceID, nil)
		require.NoError(t, err)
		require.NoError(t, repo.Save(ctx, al))
	}

	results, err := repo.ListAll(ctx, domain.Filter{Resource: "customer"})
	require.NoError(t, err)
	require.Len(t, results, 2)

	for _, r := range results {
		assert.Equal(t, "customer", r.Resource)
	}
}

func TestGormAuditLogRepository_ListForResource(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	// Two entries for cust-001, one for cust-002.
	for _, e := range []struct {
		action, resource, resourceID string
	}{
		{"customer.created", "customer", "cust-001"},
		{"customer.updated", "customer", "cust-001"},
		{"customer.created", "customer", "cust-002"},
	} {
		al, err := domain.NewAuditLog("", "", e.action, e.resource, e.resourceID, nil)
		require.NoError(t, err)
		// Small sleep so created_at ordering is deterministic.
		al.CreatedAt = al.CreatedAt.Add(time.Duration(len(e.action)) * time.Millisecond)
		require.NoError(t, repo.Save(ctx, al))
	}

	results, err := repo.ListForResource(ctx, "customer", "cust-001")
	require.NoError(t, err)
	require.Len(t, results, 2)

	for _, r := range results {
		assert.Equal(t, "customer", r.Resource)
		assert.Equal(t, "cust-001", r.ResourceID)
	}
}
