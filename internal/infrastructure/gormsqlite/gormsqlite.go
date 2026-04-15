package gormsqlite

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	R *gorm.DB // reader: NumCPU connections, WAL read-only
	W *gorm.DB // writer: 1 connection, serialises all writes
}

type Tx struct {
	*gorm.DB
}

type cbfn func(tx *Tx) error

func (db *DB) ReadTX(ctx context.Context, fn cbfn) error {
	return db.R.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Tx{tx})
	})
}

func (db *DB) WriteTX(ctx context.Context, fn cbfn) error {
	return db.W.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Tx{tx})
	})
}

// Close releases both connection pools.
func (db *DB) Close() error {
	rdb, err := db.R.DB()
	if err != nil {
		return fmt.Errorf("get reader sql.DB: %w", err)
	}
	wdb, err := db.W.DB()
	if err != nil {
		return fmt.Errorf("get writer sql.DB: %w", err)
	}
	_ = rdb.Close()
	return wdb.Close()
}

func Open(file string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Silent,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  true,
		},
	)

	cfg := &gorm.Config{
		PrepareStmt: true,
		Logger:      newLogger,
	}

	// Shared pragmas: WAL journal, 5 s busy timeout, foreign keys
	pragmas := "?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"

	// Reader — read-only, many connections
	rdb, err := gorm.Open(sqlite.Open(file+pragmas+"&mode=ro"), cfg)
	if err != nil {
		return nil, fmt.Errorf("open reader: %w", err)
	}
	rsql, _ := rdb.DB()
	rsql.SetMaxOpenConns(runtime.NumCPU())
	rsql.SetMaxIdleConns(runtime.NumCPU())
	rsql.SetConnMaxLifetime(-1)

	// Writer — single connection, serialises all writes
	wdb, err := gorm.Open(sqlite.Open(file+pragmas), cfg)
	if err != nil {
		return nil, fmt.Errorf("open writer: %w", err)
	}
	wsql, _ := wdb.DB()
	wsql.SetMaxOpenConns(1)
	wsql.SetMaxIdleConns(1)
	wsql.SetConnMaxLifetime(-1)

	return &DB{R: rdb, W: wdb}, nil
}
