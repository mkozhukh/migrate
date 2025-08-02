package migrate

import (
	"context"
	"fmt"
	"slices"
)

// Logger is a logger interface, slog compatible
type Logger interface {
	Info(msg string, v ...interface{})
}

// RunMigrations executes all pending migrations
func RunMigrations(ctx context.Context, source Source, dialect Dialect, logger Logger) error {
	err := initSelf(ctx, dialect)
	if err != nil {
		return err
	}

	// Get list of migration files
	files, err := source.GetMigrations()
	if err != nil {
		return fmt.Errorf("failed to get migration files: %w", err)
	}

	// Get applied migrations
	applied, err := dialect.GetAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Apply pending migrations
	for _, file := range files {
		if slices.Contains(applied, file.Version) {
			continue
		}

		if err := commitMigration(ctx, file, dialect); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", file.Version, err)
		}

		logger.Info("migrated", "file", file.Version)
	}

	return nil
}

// RollbackMigrations executes the last N applied migrations in reverse order.
func RollbackMigrations(ctx context.Context, source Source, dialect Dialect, logger Logger, steps int) error {
	err := initSelf(ctx, dialect)
	if err != nil {
		return err
	}

	// Get all migration files from the source.
	files, err := source.GetMigrations()
	if err != nil {
		return fmt.Errorf("failed to get migration files: %w", err)
	}

	// Get all applied migrations from the dialect.
	applied, err := dialect.GetAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	if steps <= 0 || steps > len(applied) {
		steps = len(applied)
	}

	// Determine the last N migrations to be rolled back.
	if steps == 0 {
		logger.Info("no migrations to rollback")
		return nil
	}

	toRollback := applied[len(applied)-steps:]

	// Rollback migrations in reverse order.
	for i := len(toRollback) - 1; i >= 0; i-- {
		version := toRollback[i]
		var migration *Migration
		for _, f := range files {
			if f.Version == version {
				migration = &f
				break
			}
		}

		if migration == nil {
			return fmt.Errorf("migration file not found for version: %s", version)
		}

		if err := rollbackMigration(ctx, *migration, dialect); err != nil {
			return fmt.Errorf("failed to rollback migration %s: %w", version, err)
		}

		logger.Info("rolled back", "file", version)
	}

	return nil
}

func initSelf(ctx context.Context, dialect Dialect) error {
	// Create migrations table if it doesn't exist
	if err := dialect.CreateMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	return nil
}

func applyMigrations(ctx context.Context, dialect Dialect, content []byte, name string, after func(tx CommonTx) error) error {
	if len(content) == 0 {
		return fmt.Errorf("no content to apply for migration: %s", name)
	}

	// Begin transaction
	tx, err := dialect.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute migration
	if _, err = tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	// Record changes
	err = after(tx)
	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Commit transaction
	return tx.Commit()
}

func commitMigration(ctx context.Context, migration Migration, dialect Dialect) error {
	return applyMigrations(ctx, dialect, migration.Content, migration.Version, func(tx CommonTx) error {
		return dialect.StoreAppliedMigration(ctx, tx, migration.Version)
	})
}

func rollbackMigration(ctx context.Context, migration Migration, dialect Dialect) error {
	return applyMigrations(ctx, dialect, migration.DownContent, migration.Version, func(tx CommonTx) error {
		return dialect.DeleteAppliedMigration(ctx, tx, migration.Version)
	})
}
