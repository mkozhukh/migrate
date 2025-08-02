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
	// Create migrations table if it doesn't exist
	if err := dialect.CreateMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get list of migration files
	files, err := source.GetMigrationFiles()
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

		if err := applyMigration(ctx, file, dialect); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", file.Version, err)
		}

		logger.Info("migrated", "file", file.Version)
	}

	return nil
}

func applyMigration(ctx context.Context, migration Migration, dialect Dialect) error {
	// The content is already in the Migration struct, so no need to read the file.
	content := migration.Content

	// Begin transaction
	tx, err := dialect.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute migration
	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	// Record migration
	if err := dialect.StoreAppliedMigration(ctx, tx, migration.Version); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
