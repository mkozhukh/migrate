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

// Migrator encapsulates the migration logic and configuration.
type Migrator struct {
	source  Source
	dialect Dialect
	logger  Logger
}

// New creates a new Migrator.
func New(source Source, dialect Dialect, logger Logger) *Migrator {
	return &Migrator{
		source:  source,
		dialect: dialect,
		logger:  logger,
	}
}

// RunOptions holds configuration for a single migration run.
type RunOptions struct {
	DryRun bool
	// Future options like 'Force' could be added here.
}

// Option is a function that configures a RunOptions.
type Option func(*RunOptions)

// WithDryRun is an option that enables dry run mode.
// In this mode, the migrator will print the migrations that would be
// executed without applying them to the database.
func WithDryRun() Option {
	return func(opts *RunOptions) {
		opts.DryRun = true
	}
}

// Up applies all pending "up" migrations.
func (m *Migrator) Up(ctx context.Context, opts ...Option) error {
	if err := m.prepareData(ctx, 0, m.doUp, opts...); err != nil {
		return err
	}

	return nil
}

func (m *Migrator) doUp(ctx context.Context, steps int, applied []string, migrations []Migration, options *RunOptions) error {
	if steps <= 0 || steps > len(migrations) {
		steps = len(migrations)
	}

	logMessage := "migrated"
	if options.DryRun {
		logMessage = "would migrate"
	}

	// Apply pending migrations
	for _, file := range migrations {
		if steps == 0 {
			break
		}
		if slices.Contains(applied, file.Version) {
			continue
		}

		if !options.DryRun {
			if err := m.commitMigration(ctx, file); err != nil {
				return fmt.Errorf("failed to apply migration %s: %w", file.Version, err)
			}
		}

		m.logger.Info(logMessage, "file", file.Version)

		steps--
	}

	return nil
}

// Down applies a specific number of "down" migrations.
func (m *Migrator) Down(ctx context.Context, steps int, opts ...Option) error {
	if err := m.prepareData(ctx, steps, m.doDown, opts...); err != nil {
		return err
	}

	return nil
}

func (m *Migrator) doDown(ctx context.Context, steps int, applied []string, migrations []Migration, options *RunOptions) error {
	if steps < 0 || steps > len(applied) {
		steps = len(applied)
	}

	// Determine the last N migrations to be rolled back.
	if steps == 0 {
		m.logger.Info("no migrations to rollback")
		return nil
	}

	toRollback := applied[len(applied)-steps:]

	logMessage := "rolled back"
	if options.DryRun {
		logMessage = "would rollback"
	}

	// Rollback migrations in reverse order.
	for i := len(toRollback) - 1; i >= 0; i-- {
		version := toRollback[i]
		var migration *Migration
		for _, f := range migrations {
			if f.Version == version {
				migration = &f
				break
			}
		}

		if migration == nil {
			return fmt.Errorf("migration file not found for version: %s", version)
		}

		if !options.DryRun {
			if err := m.rollbackMigration(ctx, *migration); err != nil {
				return fmt.Errorf("failed to rollback migration %s: %w", version, err)
			}
		}

		m.logger.Info(logMessage, "file", version)
	}

	return nil

}

// To migrates the database up or down to a specific version.
func (m *Migrator) To(ctx context.Context, version string, opts ...Option) error {
	if err := m.prepareData(ctx, 0, func(ctx context.Context, steps int, applied []string, migrations []Migration, options *RunOptions) error {

		currentVersion := ""
		apply := true
		if len(applied) > 0 {
			currentVersion = applied[len(applied)-1]
			apply = false
		}
		if currentVersion == version {
			return nil
		}

		appliedIndex := slices.Index(applied, version)
		if appliedIndex != -1 {
			// we need to rollback
			return m.doDown(ctx, len(applied)-appliedIndex-1, applied, migrations, options)
		} else {
			upSteps := 0
			found := false
			for _, f := range migrations {
				if f.Version == currentVersion {
					apply = true
				} else if apply {
					upSteps++
				} else {
					if f.Version == version {
						return fmt.Errorf("applied migration and migrations are not in the same order for version: %s", version)
					}
				}

				if f.Version == version {
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("migration file not found for version: %s", version)
			}

			if upSteps > 0 {
				return m.doUp(ctx, upSteps, applied, migrations, options)
			}
			return nil
		}

	}, opts...); err != nil {
		return err
	}

	return nil
}

func (m *Migrator) prepareData(ctx context.Context, steps int, after func(ctx context.Context, steps int, applied []string, migrations []Migration, options *RunOptions) error, opts ...Option) error {
	options := &RunOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Create migrations table if it doesn't exist
	if err := m.dialect.CreateMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	if !options.DryRun {
		if err := m.dialect.Lock(ctx); err != nil {
			return fmt.Errorf("failed to lock database: %w", err)
		}
		defer m.dialect.Unlock(ctx)
	}

	// Get all migration files from the source.
	migrations, err := m.source.GetMigrations()
	if err != nil {
		return fmt.Errorf("failed to get migration files: %w", err)
	}

	// Get all applied migrations from the dialect.
	applied, err := m.dialect.GetAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	return after(ctx, steps, applied, migrations, options)
}

func (m *Migrator) applyMigrations(ctx context.Context, content []byte, name string, after func(tx Tx) error) error {
	if len(content) == 0 {
		return fmt.Errorf("no content to apply for migration: %s", name)
	}

	// Begin transaction
	tx, err := m.dialect.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Execute migration
	if err = tx.Exec(ctx, string(content)); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	// Record changes
	err = after(tx)
	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Commit transaction
	return tx.Commit(ctx)
}

func (m *Migrator) commitMigration(ctx context.Context, migration Migration) error {
	return m.applyMigrations(ctx, migration.Content, migration.Version, func(tx Tx) error {
		return m.dialect.StoreAppliedMigration(ctx, tx, migration.Version)
	})
}

func (m *Migrator) rollbackMigration(ctx context.Context, migration Migration) error {
	return m.applyMigrations(ctx, migration.DownContent, migration.Version, func(tx Tx) error {
		return m.dialect.DeleteAppliedMigration(ctx, tx, migration.Version)
	})
}
