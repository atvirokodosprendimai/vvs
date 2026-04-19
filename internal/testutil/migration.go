package testutil

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
)

// RunMigrations runs goose migrations for a given module against the test DB.
// migrationsFS is the embedded filesystem containing the SQL files.
// tableName is the per-module goose version table (e.g. "goose_customer").
func RunMigrations(t *testing.T, db *gormsqlite.DB, migrationsFS fs.FS, tableName string) {
	t.Helper()

	sqlDB, err := db.W.DB()
	if err != nil {
		t.Fatalf("testutil: get sql.DB from writer: %v", err)
	}

	provider, err := goose.NewProvider(
		goose.DialectSQLite3,
		sqlDB,
		migrationsFS,
		goose.WithTableName(tableName),
	)
	if err != nil {
		t.Fatalf("testutil: create migration provider %s: %v", tableName, err)
	}

	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatalf("testutil: run migrations %s: %v", tableName, err)
	}
}
