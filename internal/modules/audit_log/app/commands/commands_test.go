package commands_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vvs/isp/internal/modules/audit_log/adapters/persistence"
	"github.com/vvs/isp/internal/modules/audit_log/app/commands"
	"github.com/vvs/isp/internal/modules/audit_log/domain"
	auditlogmigrations "github.com/vvs/isp/internal/modules/audit_log/migrations"
	"github.com/vvs/isp/internal/testutil"
)

func setupFull(t *testing.T) (domain.AuditLogRepository, *commands.CreateAuditLogHandler) {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, auditlogmigrations.FS, "goose_audit_log")
	repo := persistence.NewGormAuditLogRepository(db)
	handler := commands.NewCreateAuditLogHandler(repo)
	return repo, handler
}

func TestCreateAuditLogHandler_HappyPath(t *testing.T) {
	repo, handler := setupFull(t)
	ctx := context.Background()

	cmd := commands.CreateAuditLogCommand{
		ActorID:    "user-42",
		ActorName:  "Bob",
		Action:     "customer.created",
		Resource:   "customer",
		ResourceID: "cust-001",
		Changes:    json.RawMessage(`{"company_name":"Acme Corp"}`),
	}

	err := handler.Handle(ctx, cmd)
	require.NoError(t, err)

	// Verify the entry was persisted.
	results, err := repo.ListForResource(ctx, "customer", "cust-001")
	require.NoError(t, err)
	require.Len(t, results, 1)

	got := results[0]
	assert.NotEmpty(t, got.ID)
	assert.Equal(t, "user-42", got.ActorID)
	assert.Equal(t, "Bob", got.ActorName)
	assert.Equal(t, "customer.created", got.Action)
	assert.Equal(t, "customer", got.Resource)
	assert.Equal(t, "cust-001", got.ResourceID)
	assert.JSONEq(t, `{"company_name":"Acme Corp"}`, string(got.Changes))
}

func TestCreateAuditLogHandler_InvalidCommand(t *testing.T) {
	repo, handler := setupFull(t)
	ctx := context.Background()

	// Empty action must return an error — nothing should be saved.
	err := handler.Handle(ctx, commands.CreateAuditLogCommand{
		ActorID:    "user-42",
		ActorName:  "Bob",
		Action:     "", // invalid
		Resource:   "customer",
		ResourceID: "cust-001",
	})
	require.ErrorIs(t, err, domain.ErrMissingAction)

	// Confirm nothing was saved.
	results, listErr := repo.ListAll(ctx, domain.Filter{})
	require.NoError(t, listErr)
	assert.Empty(t, results)
}
