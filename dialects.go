package migrate

import (
	"context"
	"database/sql"
)

// Dialect is a dialect interface for different SQL flavors
type Dialect interface {
	CreateMigrationsTable(ctx context.Context) error
	GetAppliedMigrations(ctx context.Context) ([]string, error)
	StoreAppliedMigration(ctx context.Context, tx CommonTx, version string) error
	DeleteAppliedMigration(ctx context.Context, tx CommonTx, version string) error

	BeginTx(ctx context.Context) (CommonTx, error)
	Lock(ctx context.Context) error
	Unlock(ctx context.Context) error
}

// CommonTx is a common transaction interface for SQL
type CommonTx interface {
	Rollback() error
	Commit() error
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// CommonDialect is a common dialect for SQL
type CommonDialect struct {
	db                    *sql.DB
	createMigrationsTable string
	getAppliedMigrations  string
	applyMigration        string
	deleteMigration       string
}

// NewCommonDialect creates a new common dialect
func NewCommonDialect(db *sql.DB, table string) *CommonDialect {
	if table == "" {
		table = "schema_migrations"
	}

	return &CommonDialect{db: db,
		createMigrationsTable: `
		CREATE TABLE IF NOT EXISTS ` + table + ` (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`,
		getAppliedMigrations: `SELECT version FROM ` + table,
		applyMigration:       `INSERT INTO ` + table + ` (version) VALUES (?)`,
		deleteMigration:      `DELETE FROM ` + table + ` WHERE version = ?`,
	}
}

// CreateMigrationsTable creates the migrations table
func (d CommonDialect) CreateMigrationsTable(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, d.createMigrationsTable)
	return err
}

// GetAppliedMigrations gets the applied migrations from the database
func (d CommonDialect) GetAppliedMigrations(ctx context.Context) ([]string, error) {
	rows, err := d.db.QueryContext(ctx, d.getAppliedMigrations)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make([]string, 0)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied = append(applied, version)
	}

	return applied, rows.Err()
}

// StoreAppliedMigration stores the applied migration in the database
func (d CommonDialect) StoreAppliedMigration(ctx context.Context, tx CommonTx, version string) error {
	_, err := tx.ExecContext(ctx, d.applyMigration, version)
	return err
}

// DeleteAppliedMigration deletes the applied migration from the database
func (d CommonDialect) DeleteAppliedMigration(ctx context.Context, tx CommonTx, version string) error {
	_, err := tx.ExecContext(ctx, d.deleteMigration, version)
	return err
}

// BeginTx begins a new transaction
func (d CommonDialect) BeginTx(ctx context.Context) (CommonTx, error) {
	return d.db.BeginTx(ctx, nil)
}

// Lock acquires a database-level lock.
func (d CommonDialect) Lock(ctx context.Context) error {
	// This is a no-op for dialects that don't support locking.
	return nil
}

// Unlock releases the database-level lock.
func (d CommonDialect) Unlock(ctx context.Context) error {
	// This is a no-op for dialects that don't support locking.
	return nil
}

// NewSQLiteDialect creates a new SQLite dialect
func NewSQLiteDialect(db *sql.DB, table string) *CommonDialect {
	res := NewCommonDialect(db, table)

	res.createMigrationsTable = `
		CREATE TABLE IF NOT EXISTS ` + table + ` (
			version TEXT PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`

	return res
}

type PostgresDialect struct {
	*CommonDialect
	lockKey int
}

// NewPostgresDialect creates a new Postgres dialect
func NewPostgresDialect(db *sql.DB, table string) *PostgresDialect {
	res := &PostgresDialect{
		CommonDialect: NewCommonDialect(db, table),
		// python3 -c "print(abs(hash('github.com/mkozhukh/migrate/v1')))"
		lockKey: 6492640049987603658,
	}

	res.createMigrationsTable = `
		CREATE TABLE IF NOT EXISTS ` + table + ` (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`
	res.applyMigration = `INSERT INTO ` + table + ` (version) VALUES ($1)`
	res.deleteMigration = `DELETE FROM ` + table + ` WHERE version = $1`

	return res
}

func (d PostgresDialect) Lock(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, "SELECT pg_advisory_lock($1)", d.lockKey)
	return err
}

func (d PostgresDialect) Unlock(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", d.lockKey)
	return err
}
