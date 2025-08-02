# Go Migrate

A simple and flexible database migration library for Go.

## Features

-   Transactional migrations to ensure atomicity.
-   Support for "up", "down" and targeted migrations.
-   Support for multiple database dialects (PostgreSQL, SQLite, and easily extensible).
-   Migrations can be embedded in the application binary.

## Migrator API

The library provides a object-oriented API through the `Migrator` struct

### Creating a Migrator

```go
// Create a migrator instance
migrator := migrate.New(source, dialect, logger)
```

The migrator encapsulates all migration logic and provides three main methods:

- `Up(ctx, opts...)` - Apply all pending migrations
- `Down(ctx, steps, opts...)` - Rollback a specific number of migrations  
- `To(ctx, version, opts...)` - Migrate to a specific version

### Full Usage Example

Here is a complete example of how to use the library with migrations embedded in your application.

```go
package main

import (
	"context"
	"database/sql"
	"embed"
	"log/slog"
	"os"

	"github.com/mkozhukh/migrate"
	_ "github.com/mattn/go-sqlite3" // Example with SQLite3
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	// 1. Setup your logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// 2. Setup your database connection
	db, err := sql.Open("sqlite3", "./test.db")
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// 3. Create a dialect for your database
	// You can pass an empty string to use the default "schema_migrations" table name
	dialect := migrate.NewSQLiteDialect(db, "")

	// 4. Create a migration source from the embedded filesystem
	source := migrate.NewFsSource(migrationsFS, "migrations")

	// 5. Create a migrator instance
	migrator := migrate.New(source, dialect, logger)

	// 6. Run the migrations
	ctx := context.Background()
	if err := migrator.Up(ctx); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	logger.Info("migrations applied successfully")

}
```


### Available Options

All methods support functional options for configuration:

- `WithDryRun()` - Preview changes without applying them

## Up-Only Migrations

By default, the library can run any `*.sql` files. This is useful for simple, forward-only migration strategies.

Your migration files can be named simply:

`migrations/`
- `001_create_users_table.sql`
- `002_add_email_to_users.sql`

To run these, you would use the `Migrator.Up()` method. Note that rolling back is not supported with this naming scheme.

```go
// Create a source from the OS filesystem
source := migrate.NewOsSource("./migrations")

// Create a migrator instance
migrator := migrate.New(source, dialect, logger)

// Run the migrations
err := migrator.Up(context.Background())
if err != nil {
    logger.Error("failed to run migrations", "error", err)
}
```

## Up and Down Migrations

For full control over migrating and rolling back changes, you should provide both "up" and "down" migration files. The library will automatically pair files based on their version name.

-   `*.up.sql`: Contains the SQL to apply a migration.
-   `*.down.sql`: Contains the SQL to revert a migration.

### File Structure Example

`migrations/`
- `20230101_create_users_table.up.sql`
- `20230101_create_users_table.down.sql`
- `20230102_add_email_to_users.up.sql`
- `20230102_add_email_to_users.down.sql`


### Rolling Back Migrations

To roll back migrations, use the `migrator.Down()` method. The second parameter is the number of steps to roll back. If you pass `-1` it will roll back all of them.

```go
// Rollback the last 2 migrations
err := migrator.Down(ctx, 2)

// Rollback all migrations
err = migrator.Down(ctx, -1)
```

### Targeted Migrations

You can also migrate to a specific version using the `migrator.To()` method. This will automatically determine whether to migrate up or down to reach the target version.

```go
// Migrate to a specific version
err := migrator.To(ctx, "20230102_add_email_to_users")
```

### Dry Run Mode

All migration methods support dry run mode, which shows what would be applied without actually changing the database.

```go
// Dry run for Up migrations
err := migrator.Up(ctx, migrate.WithDryRun())

// Dry run for Down migrations
err := migrator.Down(ctx, 2, migrate.WithDryRun())

// Dry run for targeted migrations
err := migrator.To(ctx, "20230102_add_email_to_users", migrate.WithDryRun())
```

## License

[MIT](LICENSE)
