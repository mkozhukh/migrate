  1. Introduce a Migrator Struct

  Instead of passing source, dialect, and logger to every function, you can create a Migrator object. This becomes the central point for all migration
  operations.

    1 // In lib.go
    2
    3 // Migrator encapsulates the migration logic and configuration.
    4 type Migrator struct {
    5 // unexported fields for source, dialect, logger
    6 }
    7
    8 // New creates a new Migrator.
    9 func New(source Source, dialect Dialect, logger Logger) *Migrator {
   10 // ...
   11 }

  2. Define Clear, Action-Oriented Methods

  The Migrator would have methods for each primary action. This is where you can introduce the new targeted migration feature.

    1 // In lib.go (methods on *Migrator)
    2
    3 // Up applies all pending "up" migrations.
    4 func (m *Migrator) Up(ctx context.Context, opts ...Option) error {
    5 // ...
    6 }
    7
    8 // Down applies a specific number of "down" migrations.
    9 func (m *Migrator) Down(ctx context.Context, steps int, opts ...Option) error {
   10 // ...
   11 }
   12
   13 // To migrates the database up or down to a specific version.
   14 func (m *Migrator) To(ctx context.Context, version string, opts ...Option) error {
   15 // ...
   16 }
  This design introduces To() as a new, explicit method for targeted migrations, which is much cleaner than overloading Up or Down.

  3. Use Functional Options for "Dry Run" and Future Extensions

  Instead of adding boolean flags like dryRun, you can use the "functional options" pattern. This is a common and highly extensible pattern in Go that keeps the
  method signatures clean.

    1 // In lib.go
    2
    3 // RunOptions holds configuration for a single migration run.
    4 type RunOptions struct {
    5 	DryRunbool
    6 // Future options like 'Force' could be added here.
    7 }
    8
    9 // Option is a function that configures a RunOptions.
   10 type Option func(*RunOptions)
   11
   12 // WithDryRun is an option that enables dry run mode.
   13 // In this mode, the migrator will print the migrations that would be
   14 // executed without applying them to the database.
   15 func WithDryRun() Option {
   16 // ...
   17 }
  How it would be used:

   1 migrator := migrate.New(source, dialect, logger)
   2
   3 // A normal run:
   4 migrator.Up(ctx)
   5
   6 // A dry run:
   7 migrator.Up(ctx, migrate.WithDryRun())

  4. Extend the Dialect for Locking

  For database locking, you can extend the Dialect interface as planned. The Migrator's methods (Up, Down, To) would be responsible for calling these Lock and
  Unlock methods internally.

    1 // In dialects.go
    2
    3 type Dialect interface {
    4 // ... existing methods
    5
    6 // Lock acquires a database-level lock to prevent concurrent migrations.
    7 	Lock(ctx context.Context)error
    8
    9 // Unlock releases the database-level lock.
   10 	Unlock(ctx context.Context)error
   11 }

  Summary of Proposed API

  This approach gives you:
   * A clean, object-oriented API with the Migrator struct.
   * Explicit methods for each action (Up, Down, To), avoiding confusion.
   * A scalable options pattern (WithDryRun) that doesn't clutter your method signatures and is ready for future features.
   * A clear home for locking logic within the Dialect interface, managed by the Migrator.

  This design is more robust and easier to extend in the future than simply adding more parameters to the existing functions.

╭───────────────────────────────────────────────────────────────────────────────╮
│  > show small snippet how the migrator will be initialised from consumer app  │
╰───────────────────────────────────────────────────────────────────────────────╯

✦ Of course. Here is a small snippet showing how a consumer app would initialize and use the proposed Migrator.

  This example assumes it's part of a main.go file where the db, logger, and source have already been configured.

    1 // --- In your application's main.go ---
    2
    3 // 1. Initialize the migration library components.
    4 dialect := migrate.NewPostgresDialect(db, "schema_migrations")
    5 source := migrate.NewFsSource(migrationsFS, "migrations")
    6
    7 // 2. Create a new Migrator instance.
    8 //    This object is now your single point of entry for all migration tasks.
    9 migrator := migrate.New(source, dialect, logger)
   10
   11 // 3. Use the Migrator to perform actions.
   12 ctx := context.Background()
   13
   14 // Example 1: Apply all pending migrations.
   15 fmt.Println("Applying all pending migrations...")
   16 if err := migrator.Up(ctx); err != nil {
   17     log.Fatalf("Failed to apply migrations: %v", err)
   18 }
   19
   20 // Example 2: Roll back to a specific version.
   21 fmt.Println("Rolling back to version '20230101_create_users_table'...")
   22 if err := migrator.To(ctx, "20230101_create_users_table"); err != nil {
   23     log.Fatalf("Failed to migrate to version: %v", err)
   24 }
   25
   26 // Example 3: Dry run of applying all migrations.
   27 fmt.Println("Performing a dry run of all migrations...")
   28 if err := migrator.Up(ctx, migrate.WithDryRun()); err != nil {
   29     log.Fatalf("Failed to perform dry run: %v", err)
   30 }