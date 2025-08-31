package migrate

import (
	"context"
	"database/sql"
)

// Dialect is a dialect interface for different SQL flavors
type Dialect interface {
	CreateMigrationsTable(ctx context.Context) error
	GetAppliedMigrations(ctx context.Context) ([]string, error)
	StoreAppliedMigration(ctx context.Context, tx Tx, version string) error
	DeleteAppliedMigration(ctx context.Context, tx Tx, version string) error

	BeginTx(ctx context.Context) (Tx, error)
	Lock(ctx context.Context) error
	Unlock(ctx context.Context) error
}

// Tx is a common transaction interface for SQL
type Tx interface {
	Rollback(ctx context.Context) error
	Commit(ctx context.Context) error
	Exec(ctx context.Context, query string, args ...interface{}) error
}

type CommonTx struct {
	db *sql.Tx
}

func (t CommonTx) Rollback(ctx context.Context) error {
	return t.db.Rollback()
}

func (t CommonTx) Commit(ctx context.Context) error {
	return t.db.Commit()
}

func (t CommonTx) Exec(ctx context.Context, query string, args ...interface{}) error {
	_, err := t.db.ExecContext(ctx, query, args...)
	return err
}

// CommonDialect is a common dialect for SQL
type CommonDialect struct {
	db                       *sql.DB
	tableName                string
	executor                 func(ctx context.Context, query string, args ...interface{}) error
	CreateMigrationsTableSQL string
	GetAppliedMigrationsSQL  string
	ApplyMigrationSQL        string
	DeleteMigrationSQL       string
}

// NewCommonDialect creates a new common dialect
func NewCommonDialect(db *sql.DB, table string) *CommonDialect {
	if table == "" {
		table = "schema_migrations"
	}

	return &CommonDialect{db: db,
		tableName: table,
		executor: func(ctx context.Context, query string, args ...interface{}) error {
			_, err := db.ExecContext(ctx, query, args...)
			return err
		},
		CreateMigrationsTableSQL: `
		CREATE TABLE IF NOT EXISTS ` + table + ` (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`,
		GetAppliedMigrationsSQL: `SELECT version FROM ` + table,
		ApplyMigrationSQL:       `INSERT INTO ` + table + ` (version) VALUES (?)`,
		DeleteMigrationSQL:      `DELETE FROM ` + table + ` WHERE version = ?`,
	}
}

func (d *CommonDialect) SetExecutor(executor func(ctx context.Context, query string, args ...interface{}) error) {
	d.executor = executor
}

// CreateMigrationsTable creates the migrations table
func (d *CommonDialect) CreateMigrationsTable(ctx context.Context) error {
	return d.executor(ctx, d.CreateMigrationsTableSQL)
}

// GetAppliedMigrations gets the applied migrations from the database
func (d *CommonDialect) GetAppliedMigrations(ctx context.Context) ([]string, error) {
	rows, err := d.db.QueryContext(ctx, d.GetAppliedMigrationsSQL)
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
func (d *CommonDialect) StoreAppliedMigration(ctx context.Context, tx Tx, version string) error {
	err := tx.Exec(ctx, d.ApplyMigrationSQL, version)
	return err
}

// DeleteAppliedMigration deletes the applied migration from the database
func (d *CommonDialect) DeleteAppliedMigration(ctx context.Context, tx Tx, version string) error {
	err := tx.Exec(ctx, d.DeleteMigrationSQL, version)
	return err
}

// BeginTx begins a new transaction
func (d *CommonDialect) BeginTx(ctx context.Context) (Tx, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return CommonTx{db: tx}, nil
}

// Lock acquires a database-level lock.
func (d *CommonDialect) Lock(ctx context.Context) error {
	// This is a no-op for dialects that don't support locking.
	return nil
}

// Unlock releases the database-level lock.
func (d *CommonDialect) Unlock(ctx context.Context) error {
	// This is a no-op for dialects that don't support locking.
	return nil
}

// NewSQLiteDialect creates a new SQLite dialect
func NewSQLiteDialect(db *sql.DB, table string) *CommonDialect {
	res := NewCommonDialect(db, table)

	res.CreateMigrationsTableSQL = `
		CREATE TABLE IF NOT EXISTS ` + res.tableName + ` (
			version TEXT PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`

	return res
}

type PostgresDialect struct {
	*CommonDialect
	LockKey int
}

// NewPostgresDialect creates a new Postgres dialect
func NewPostgresDialect(db *sql.DB, table string) *PostgresDialect {
	res := &PostgresDialect{
		CommonDialect: NewCommonDialect(db, table),
		// python3 -c "print(abs(hash('github.com/mkozhukh/migrate/v1')))"
		LockKey: 6492640049987603658,
	}

	res.CreateMigrationsTableSQL = `
		CREATE TABLE IF NOT EXISTS ` + res.tableName + ` (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`
	res.ApplyMigrationSQL = `INSERT INTO ` + res.tableName + ` (version) VALUES ($1)`
	res.DeleteMigrationSQL = `DELETE FROM ` + res.tableName + ` WHERE version = $1`

	return res
}

func (d *PostgresDialect) Lock(ctx context.Context) error {
	return d.executor(ctx, "SELECT pg_advisory_lock($1)", d.LockKey)
}

func (d *PostgresDialect) Unlock(ctx context.Context) error {
	return d.executor(ctx, "SELECT pg_advisory_unlock($1)", d.LockKey)
}
