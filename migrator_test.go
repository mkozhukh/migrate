package migrate

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// Mock implementations for testing

type MockLogger struct {
	infoLogs []string
}

func (l *MockLogger) Info(msg string, v ...interface{}) {
	if len(v) == 0 {
		l.infoLogs = append(l.infoLogs, msg)
	} else {
		// Handle the case where we have key-value pairs
		formatted := msg
		for i := 0; i < len(v); i += 2 {
			if i+1 < len(v) {
				formatted += fmt.Sprintf(" %v=%v", v[i], v[i+1])
			}
		}
		l.infoLogs = append(l.infoLogs, formatted)
	}
}

func (l *MockLogger) GetLogs() []string {
	return l.infoLogs
}

func (l *MockLogger) Clear() {
	l.infoLogs = nil
}

type MockSource struct {
	migrations []Migration
	err        error
}

func (s *MockSource) GetMigrations() ([]Migration, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.migrations, nil
}

type MockTx struct {
	execCalled     bool
	commitCalled   bool
	rollbackCalled bool
	execErr        error
	commitErr      error
	rollbackErr    error
}

func (tx *MockTx) Exec(ctx context.Context, query string, args ...interface{}) error {
	tx.execCalled = true
	if tx.execErr != nil {
		return tx.execErr
	}
	return nil
}

func (tx *MockTx) Commit(ctx context.Context) error {
	tx.commitCalled = true
	return tx.commitErr
}

func (tx *MockTx) Rollback(ctx context.Context) error {
	tx.rollbackCalled = true
	return tx.rollbackErr
}

type MockDialect struct {
	createTableCalled     bool
	getAppliedCalled      bool
	storeMigrationCalled  bool
	deleteMigrationCalled bool
	lockCalled            bool
	unlockCalled          bool
	beginTxCalled         bool
	execContextCalled     bool

	appliedMigrations  []string
	createTableErr     error
	getAppliedErr      error
	storeMigrationErr  error
	deleteMigrationErr error
	lockErr            error
	unlockErr          error
	beginTxErr         error
	execContextErr     error

	// For tracking what was stored/deleted
	storedMigrations  []string
	deletedMigrations []string
}

func (d *MockDialect) CreateMigrationsTable(ctx context.Context) error {
	d.createTableCalled = true
	return d.createTableErr
}

func (d *MockDialect) GetAppliedMigrations(ctx context.Context) ([]string, error) {
	d.getAppliedCalled = true
	if d.getAppliedErr != nil {
		return nil, d.getAppliedErr
	}
	return d.appliedMigrations, nil
}

func (d *MockDialect) StoreAppliedMigration(ctx context.Context, tx Tx, version string) error {
	d.storeMigrationCalled = true
	d.storedMigrations = append(d.storedMigrations, version)
	if d.storeMigrationErr != nil {
		return d.storeMigrationErr
	}
	return nil
}

func (d *MockDialect) DeleteAppliedMigration(ctx context.Context, tx Tx, version string) error {
	d.deleteMigrationCalled = true
	d.deletedMigrations = append(d.deletedMigrations, version)
	if d.deleteMigrationErr != nil {
		return d.deleteMigrationErr
	}
	return nil
}

func (d *MockDialect) BeginTx(ctx context.Context) (Tx, error) {
	d.beginTxCalled = true
	if d.beginTxErr != nil {
		return nil, d.beginTxErr
	}
	return &MockTx{}, nil
}

func (d *MockDialect) Lock(ctx context.Context) error {
	d.lockCalled = true
	return d.lockErr
}

func (d *MockDialect) Unlock(ctx context.Context) error {
	d.unlockCalled = true
	return d.unlockErr
}

func (d *MockDialect) ExecContext(ctx context.Context, query string, args ...interface{}) error {
	d.execContextCalled = true
	return d.execContextErr
}

// Helper function to create test migrations
func createTestMigrations() []Migration {
	return []Migration{
		{
			Version:     "001_create_users",
			Content:     []byte("CREATE TABLE users (id INT PRIMARY KEY)"),
			DownContent: []byte("DROP TABLE users"),
		},
		{
			Version:     "002_add_email",
			Content:     []byte("ALTER TABLE users ADD COLUMN email VARCHAR(255)"),
			DownContent: []byte("ALTER TABLE users DROP COLUMN email"),
		},
		{
			Version:     "003_add_index",
			Content:     []byte("CREATE INDEX idx_users_email ON users(email)"),
			DownContent: []byte("DROP INDEX idx_users_email"),
		},
		{
			Version:     "004_add_timestamp",
			Content:     []byte("ALTER TABLE users ADD COLUMN created_at TIMESTAMP"),
			DownContent: []byte("ALTER TABLE users DROP COLUMN created_at"),
		},
	}
}

// Test cases for Up method
func TestMigratorUp(t *testing.T) {
	tests := []struct {
		name           string
		migrations     []Migration
		applied        []string
		expectedLogs   []string
		expectedStored []string
		expectError    bool
		dryRun         bool
	}{
		{
			name:           "apply all pending migrations",
			migrations:     createTestMigrations(),
			applied:        []string{},
			expectedLogs:   []string{"migrated file=001_create_users", "migrated file=002_add_email", "migrated file=003_add_index", "migrated file=004_add_timestamp"},
			expectedStored: []string{"001_create_users", "002_add_email", "003_add_index", "004_add_timestamp"},
			expectError:    false,
		},
		{
			name:           "apply only pending migrations",
			migrations:     createTestMigrations(),
			applied:        []string{"001_create_users", "002_add_email"},
			expectedLogs:   []string{"migrated file=003_add_index", "migrated file=004_add_timestamp"},
			expectedStored: []string{"003_add_index", "004_add_timestamp"},
			expectError:    false,
		},
		{
			name:           "no pending migrations",
			migrations:     createTestMigrations(),
			applied:        []string{"001_create_users", "002_add_email", "003_add_index", "004_add_timestamp"},
			expectedLogs:   []string{},
			expectedStored: []string{},
			expectError:    false,
		},
		{
			name:           "dry run mode",
			migrations:     createTestMigrations(),
			applied:        []string{"001_create_users"},
			expectedLogs:   []string{"would migrate file=002_add_email", "would migrate file=003_add_index", "would migrate file=004_add_timestamp"},
			expectedStored: []string{},
			expectError:    false,
			dryRun:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			logger := &MockLogger{}
			source := &MockSource{migrations: tt.migrations}
			dialect := &MockDialect{appliedMigrations: tt.applied}

			migrator := New(source, dialect, logger)

			// Execute
			var err error
			if tt.dryRun {
				err = migrator.Up(context.Background(), WithDryRun())
			} else {
				err = migrator.Up(context.Background())
			}

			// Assertions
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check logs
			if len(logger.GetLogs()) != len(tt.expectedLogs) {
				t.Errorf("expected %d logs, got %d", len(tt.expectedLogs), len(logger.GetLogs()))
			}
			for i, expected := range tt.expectedLogs {
				if i < len(logger.GetLogs()) && logger.GetLogs()[i] != expected {
					t.Errorf("log %d: expected %q, got %q", i, expected, logger.GetLogs()[i])
				}
			}

			// Check stored migrations (only in non-dry-run mode)
			if !tt.dryRun {
				if len(dialect.storedMigrations) != len(tt.expectedStored) {
					t.Errorf("expected %d stored migrations, got %d", len(tt.expectedStored), len(dialect.storedMigrations))
				}
				for i, expected := range tt.expectedStored {
					if i < len(dialect.storedMigrations) && dialect.storedMigrations[i] != expected {
						t.Errorf("stored migration %d: expected %q, got %q", i, expected, dialect.storedMigrations[i])
					}
				}
			}
		})
	}
}

// Test cases for Down method
func TestMigratorDown(t *testing.T) {
	tests := []struct {
		name            string
		migrations      []Migration
		applied         []string
		steps           int
		expectedLogs    []string
		expectedDeleted []string
		expectError     bool
		dryRun          bool
	}{
		{
			name:            "rollback last 2 migrations",
			migrations:      createTestMigrations(),
			applied:         []string{"001_create_users", "002_add_email", "003_add_index", "004_add_timestamp"},
			steps:           2,
			expectedLogs:    []string{"rolled back file=004_add_timestamp", "rolled back file=003_add_index"},
			expectedDeleted: []string{"004_add_timestamp", "003_add_index"},
			expectError:     false,
		},
		{
			name:            "rollback all migrations",
			migrations:      createTestMigrations(),
			applied:         []string{"001_create_users", "002_add_email", "003_add_index", "004_add_timestamp"},
			steps:           -1, // -1 means rollback all
			expectedLogs:    []string{"rolled back file=004_add_timestamp", "rolled back file=003_add_index", "rolled back file=002_add_email", "rolled back file=001_create_users"},
			expectedDeleted: []string{"004_add_timestamp", "003_add_index", "002_add_email", "001_create_users"},
			expectError:     false,
		},
		{
			name:            "rollback none",
			migrations:      createTestMigrations(),
			applied:         []string{"001_create_users", "002_add_email", "003_add_index", "004_add_timestamp"},
			steps:           0,
			expectedLogs:    []string{"no migrations to rollback"},
			expectedDeleted: []string{},
			expectError:     false,
		},
		{
			name:            "rollback more than available",
			migrations:      createTestMigrations(),
			applied:         []string{"001_create_users", "002_add_email"},
			steps:           5, // More than available
			expectedLogs:    []string{"rolled back file=002_add_email", "rolled back file=001_create_users"},
			expectedDeleted: []string{"002_add_email", "001_create_users"},
			expectError:     false,
		},
		{
			name:            "no migrations to rollback",
			migrations:      createTestMigrations(),
			applied:         []string{},
			steps:           2,
			expectedLogs:    []string{"no migrations to rollback"},
			expectedDeleted: []string{},
			expectError:     false,
		},
		{
			name:            "dry run rollback",
			migrations:      createTestMigrations(),
			applied:         []string{"001_create_users", "002_add_email", "003_add_index"},
			steps:           2,
			expectedLogs:    []string{"would rollback file=003_add_index", "would rollback file=002_add_email"},
			expectedDeleted: []string{},
			expectError:     false,
			dryRun:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			logger := &MockLogger{}
			source := &MockSource{migrations: tt.migrations}
			dialect := &MockDialect{appliedMigrations: tt.applied}

			migrator := New(source, dialect, logger)

			// Execute
			var err error
			if tt.dryRun {
				err = migrator.Down(context.Background(), tt.steps, WithDryRun())
			} else {
				err = migrator.Down(context.Background(), tt.steps)
			}

			// Assertions
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check logs
			if len(logger.GetLogs()) != len(tt.expectedLogs) {
				t.Errorf("expected %d logs, got %d", len(tt.expectedLogs), len(logger.GetLogs()))
			}
			for i, expected := range tt.expectedLogs {
				if i < len(logger.GetLogs()) && logger.GetLogs()[i] != expected {
					t.Errorf("log %d: expected %q, got %q", i, expected, logger.GetLogs()[i])
				}
			}

			// Check deleted migrations (only in non-dry-run mode)
			if !tt.dryRun {
				if len(dialect.deletedMigrations) != len(tt.expectedDeleted) {
					t.Errorf("expected %d deleted migrations, got %d", len(tt.expectedDeleted), len(dialect.deletedMigrations))
				}
				for i, expected := range tt.expectedDeleted {
					if i < len(dialect.deletedMigrations) && dialect.deletedMigrations[i] != expected {
						t.Errorf("deleted migration %d: expected %q, got %q", i, expected, dialect.deletedMigrations[i])
					}
				}
			}
		})
	}
}

// Test cases for To method
func TestMigratorTo(t *testing.T) {
	tests := []struct {
		name            string
		migrations      []Migration
		applied         []string
		targetVersion   string
		expectedLogs    []string
		expectedStored  []string
		expectedDeleted []string
		expectError     bool
		dryRun          bool
	}{
		{
			name:            "migrate up to specific version",
			migrations:      createTestMigrations(),
			applied:         []string{"001_create_users"},
			targetVersion:   "003_add_index",
			expectedLogs:    []string{"migrated file=002_add_email", "migrated file=003_add_index"},
			expectedStored:  []string{"002_add_email", "003_add_index"},
			expectedDeleted: []string{},
			expectError:     false,
		},
		{
			name:            "migrate down to specific version",
			migrations:      createTestMigrations(),
			applied:         []string{"001_create_users", "002_add_email", "003_add_index", "004_add_timestamp"},
			targetVersion:   "002_add_email",
			expectedLogs:    []string{"rolled back file=004_add_timestamp", "rolled back file=003_add_index"},
			expectedStored:  []string{},
			expectedDeleted: []string{"004_add_timestamp", "003_add_index"},
			expectError:     false,
		},
		{
			name:            "already at target version",
			migrations:      createTestMigrations(),
			applied:         []string{"001_create_users", "002_add_email"},
			targetVersion:   "002_add_email",
			expectedLogs:    []string{},
			expectedStored:  []string{},
			expectedDeleted: []string{},
			expectError:     false,
		},
		{
			name:            "migrate to first version from empty",
			migrations:      createTestMigrations(),
			applied:         []string{},
			targetVersion:   "001_create_users",
			expectedLogs:    []string{"migrated file=001_create_users"},
			expectedStored:  []string{"001_create_users"},
			expectedDeleted: []string{},
			expectError:     false,
		},
		{
			name:            "migrate to last version",
			migrations:      createTestMigrations(),
			applied:         []string{"001_create_users"},
			targetVersion:   "004_add_timestamp",
			expectedLogs:    []string{"migrated file=002_add_email", "migrated file=003_add_index", "migrated file=004_add_timestamp"},
			expectedStored:  []string{"002_add_email", "003_add_index", "004_add_timestamp"},
			expectedDeleted: []string{},
			expectError:     false,
		},
		{
			name:            "target version not found",
			migrations:      createTestMigrations(),
			applied:         []string{"001_create_users"},
			targetVersion:   "999_nonexistent",
			expectedLogs:    []string{},
			expectedStored:  []string{},
			expectedDeleted: []string{},
			expectError:     true,
		},
		{
			name:            "dry run migrate up",
			migrations:      createTestMigrations(),
			applied:         []string{"001_create_users"},
			targetVersion:   "003_add_index",
			expectedLogs:    []string{"would migrate file=002_add_email", "would migrate file=003_add_index"},
			expectedStored:  []string{},
			expectedDeleted: []string{},
			expectError:     false,
			dryRun:          true,
		},
		{
			name:            "dry run migrate down",
			migrations:      createTestMigrations(),
			applied:         []string{"001_create_users", "002_add_email", "003_add_index", "004_add_timestamp"},
			targetVersion:   "002_add_email",
			expectedLogs:    []string{"would rollback file=004_add_timestamp", "would rollback file=003_add_index"},
			expectedStored:  []string{},
			expectedDeleted: []string{},
			expectError:     false,
			dryRun:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			logger := &MockLogger{}
			source := &MockSource{migrations: tt.migrations}
			dialect := &MockDialect{appliedMigrations: tt.applied}

			migrator := New(source, dialect, logger)

			// Execute
			var err error
			if tt.dryRun {
				err = migrator.To(context.Background(), tt.targetVersion, WithDryRun())
			} else {
				err = migrator.To(context.Background(), tt.targetVersion)
			}

			// Assertions
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check logs
			if len(logger.GetLogs()) != len(tt.expectedLogs) {
				t.Errorf("expected %d logs, got %d", len(tt.expectedLogs), len(logger.GetLogs()))
			}
			for i, expected := range tt.expectedLogs {
				if i < len(logger.GetLogs()) && logger.GetLogs()[i] != expected {
					t.Errorf("log %d: expected %q, got %q", i, expected, logger.GetLogs()[i])
				}
			}

			// Check stored/deleted migrations (only in non-dry-run mode)
			if !tt.dryRun {
				if len(dialect.storedMigrations) != len(tt.expectedStored) {
					t.Errorf("expected %d stored migrations, got %d", len(tt.expectedStored), len(dialect.storedMigrations))
				}
				for i, expected := range tt.expectedStored {
					if i < len(dialect.storedMigrations) && dialect.storedMigrations[i] != expected {
						t.Errorf("stored migration %d: expected %q, got %q", i, expected, dialect.storedMigrations[i])
					}
				}

				if len(dialect.deletedMigrations) != len(tt.expectedDeleted) {
					t.Errorf("expected %d deleted migrations, got %d", len(tt.expectedDeleted), len(dialect.deletedMigrations))
				}
				for i, expected := range tt.expectedDeleted {
					if i < len(dialect.deletedMigrations) && dialect.deletedMigrations[i] != expected {
						t.Errorf("deleted migration %d: expected %q, got %q", i, expected, dialect.deletedMigrations[i])
					}
				}
			}
		})
	}
}

// Test error conditions
func TestMigratorErrors(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func(*MockSource, *MockDialect)
		operation   func(*Migrator) error
		expectError bool
	}{
		{
			name: "source GetMigrations error",
			setupMocks: func(source *MockSource, dialect *MockDialect) {
				source.err = errors.New("source error")
			},
			operation: func(m *Migrator) error {
				return m.Up(context.Background())
			},
			expectError: true,
		},
		{
			name: "dialect GetAppliedMigrations error",
			setupMocks: func(source *MockSource, dialect *MockDialect) {
				dialect.getAppliedErr = errors.New("dialect error")
			},
			operation: func(m *Migrator) error {
				return m.Up(context.Background())
			},
			expectError: true,
		},
		{
			name: "dialect CreateMigrationsTable error",
			setupMocks: func(source *MockSource, dialect *MockDialect) {
				dialect.createTableErr = errors.New("create table error")
			},
			operation: func(m *Migrator) error {
				return m.Up(context.Background())
			},
			expectError: true,
		},
		{
			name: "dialect Lock error",
			setupMocks: func(source *MockSource, dialect *MockDialect) {
				dialect.lockErr = errors.New("lock error")
			},
			operation: func(m *Migrator) error {
				return m.Up(context.Background())
			},
			expectError: true,
		},
		{
			name: "migration file not found for rollback",
			setupMocks: func(source *MockSource, dialect *MockDialect) {
				// Applied migrations that don't exist in source
				dialect.appliedMigrations = []string{"001_create_users", "999_nonexistent"}
				source.migrations = createTestMigrations()
			},
			operation: func(m *Migrator) error {
				return m.Down(context.Background(), 1)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			logger := &MockLogger{}
			source := &MockSource{migrations: createTestMigrations()}
			dialect := &MockDialect{appliedMigrations: []string{}}

			tt.setupMocks(source, dialect)

			migrator := New(source, dialect, logger)

			// Execute
			err := tt.operation(migrator)

			// Assertions
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Test the order of operations
func TestMigratorOperationOrder(t *testing.T) {
	t.Run("Up operation order", func(t *testing.T) {
		logger := &MockLogger{}
		source := &MockSource{migrations: createTestMigrations()}
		dialect := &MockDialect{appliedMigrations: []string{"001_create_users"}}

		migrator := New(source, dialect, logger)

		err := migrator.Up(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify operation order
		if !dialect.createTableCalled {
			t.Error("CreateMigrationsTable should be called first")
		}
		if !dialect.lockCalled {
			t.Error("Lock should be called after CreateMigrationsTable")
		}
		if !dialect.getAppliedCalled {
			t.Error("GetAppliedMigrations should be called after Lock")
		}
		if !dialect.unlockCalled {
			t.Error("Unlock should be called at the end")
		}

		// Verify migrations are applied in order
		expectedOrder := []string{"002_add_email", "003_add_index", "004_add_timestamp"}
		if len(dialect.storedMigrations) != len(expectedOrder) {
			t.Errorf("expected %d migrations, got %d", len(expectedOrder), len(dialect.storedMigrations))
		}
		for i, expected := range expectedOrder {
			if i < len(dialect.storedMigrations) && dialect.storedMigrations[i] != expected {
				t.Errorf("migration %d: expected %q, got %q", i, expected, dialect.storedMigrations[i])
			}
		}
	})

	t.Run("Down operation order", func(t *testing.T) {
		logger := &MockLogger{}
		source := &MockSource{migrations: createTestMigrations()}
		dialect := &MockDialect{appliedMigrations: []string{"001_create_users", "002_add_email", "003_add_index"}}

		migrator := New(source, dialect, logger)

		err := migrator.Down(context.Background(), 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify operation order
		if !dialect.createTableCalled {
			t.Error("CreateMigrationsTable should be called first")
		}
		if !dialect.lockCalled {
			t.Error("Lock should be called after CreateMigrationsTable")
		}
		if !dialect.getAppliedCalled {
			t.Error("GetAppliedMigrations should be called after Lock")
		}
		if !dialect.unlockCalled {
			t.Error("Unlock should be called at the end")
		}

		// Verify migrations are rolled back in reverse order
		expectedOrder := []string{"003_add_index", "002_add_email"}
		if len(dialect.deletedMigrations) != len(expectedOrder) {
			t.Errorf("expected %d migrations, got %d", len(expectedOrder), len(dialect.deletedMigrations))
		}
		for i, expected := range expectedOrder {
			if i < len(dialect.deletedMigrations) && dialect.deletedMigrations[i] != expected {
				t.Errorf("migration %d: expected %q, got %q", i, expected, dialect.deletedMigrations[i])
			}
		}
	})
}
