package testutil

import (
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewTestDB creates a temporary SQLite database with WAL mode for testing.
// Returns a *gormsqlite.DB with separate reader/writer pools, matching production.
// Each call creates a fresh database. Cleanup is automatic via t.Cleanup.
func NewTestDB(t *testing.T) *gormsqlite.DB {
	t.Helper()

	dir := t.TempDir()
	file := filepath.Join(dir, "test.db")

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Silent,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  false,
		},
	)

	cfg := &gorm.Config{
		PrepareStmt: true,
		Logger:      newLogger,
	}

	pragmas := "?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"

	// Writer — single connection, serialises all writes (must be opened first to create the file)
	wdb, err := gorm.Open(sqlite.Open(file+pragmas), cfg)
	if err != nil {
		t.Fatalf("testutil: open writer db: %v", err)
	}
	wsql, _ := wdb.DB()
	wsql.SetMaxOpenConns(1)
	wsql.SetMaxIdleConns(1)
	wsql.SetConnMaxLifetime(-1)

	// Reader — read-only, multiple connections
	rdb, err := gorm.Open(sqlite.Open(file+pragmas+"&mode=ro"), cfg)
	if err != nil {
		t.Fatalf("testutil: open reader db: %v", err)
	}
	rsql, _ := rdb.DB()
	rsql.SetMaxOpenConns(2)
	rsql.SetMaxIdleConns(2)
	rsql.SetConnMaxLifetime(-1)

	db := &gormsqlite.DB{R: rdb, W: wdb}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}
