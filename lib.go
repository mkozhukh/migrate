package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// Logger is a logger interface, slog compatible
type Logger interface {
	Info(msg string, v ...interface{})
}

// RunMigrations executes all pending migrations
func RunMigrations(ctx context.Context, dialect Dialect, migrationsPath string, logger Logger) error {
	// Create migrations table if it doesn't exist
	if err := dialect.CreateMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get list of migration files
	files, err := getMigrationFiles(migrationsPath)
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
		if slices.Contains(applied, file) {
			continue
		}

		if err := applyMigration(ctx, dialect, migrationsPath, file); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", file, err)
		}

		logger.Info("migrated", "file", file)
	}

	return nil
}

func getMigrationFiles(migrationsPath string) ([]string, error) {
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}

	sort.Strings(files)
	return files, nil
}

func applyMigration(ctx context.Context, dialect Dialect, migrationsPath, filename string) error {
	// Read migration file
	content, err := os.ReadFile(filepath.Join(migrationsPath, filename))
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

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
	if err := dialect.StoreAppliedMigration(ctx, tx, filename); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
