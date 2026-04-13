package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

type ModuleMigration struct {
	Name      string
	FS        fs.FS
	TableName string
}

func RunModuleMigrations(db *sql.DB, modules []ModuleMigration) error {
	for _, m := range modules {
		provider, err := goose.NewProvider(
			goose.DialectSQLite3,
			db,
			m.FS,
			goose.WithTableName(m.TableName),
		)
		if err != nil {
			return fmt.Errorf("migration provider %s: %w", m.Name, err)
		}
		if _, err := provider.Up(context.Background()); err != nil {
			return fmt.Errorf("migrate %s: %w", m.Name, err)
		}
	}
	return nil
}
