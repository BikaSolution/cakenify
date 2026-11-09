package cakenifydb

import (
	"context"
	"database/sql"
	_ "embed"
)

//go:embed migrate.sql
var migrateSQL string

// RunMigration executes the migration SQL to set up cakenify's tables.
// It's idempotent (CREATE TABLE/INDEX IF NOT EXISTS) so it's safe to run on
// every startup instead of tracking a migration version.
func RunMigration(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, migrateSQL)
	return err
}
