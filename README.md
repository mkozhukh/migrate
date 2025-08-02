# Go Migrate

A simple and flexible database migration library for Go.

## Features

-   Transactional migrations to ensure atomicity.
-   Support for both "up" and "down" migrations.
-   Support for multiple database dialects (PostgreSQL, SQLite, and easily extensible).
-   Migrations can be embedded in the application binary.

## Default Behavior (Up-Only Migrations)

By default, the library can run any `*.sql` files. This is useful for simple, forward-only migration strategies.

Your migration files can be named simply:

`migrations/`
- `001_create_users_table.sql`
- `002_add_email_to_users.sql`

To run these, you would use the `RunMigrations` function. Note that rolling back is not supported with this naming scheme.

```go
// Omitting full main function for brevity

// Create a source from the OS filesystem
source := migrate.NewOsSource("./migrations")

// Run the migrations
err := migrate.RunMigrations(context.Background(), source, dialect, logger)
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

	// 5. Run the migrations
	ctx := context.Background()
	if err := migrate.RunMigrations(ctx, source, dialect, logger); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	logger.Info("migrations applied successfully")

	// 6. (Optional) Rollback the last migration
	logger.Info("rolling back last migration")
	if err := migrate.RollbackMigrations(ctx, source, dialect, logger, 1); err != nil {
		logger.Error("failed to rollback migrations", "error", err)
		os.Exit(1)
	}

	logger.Info("rollback successful")
}
```

### Rolling Back Migrations

To roll back migrations, use the `RollbackMigrations` function. The last parameter is the number of steps to roll back. If you pass `0` or a number greater than the number of applied migrations, it will roll back all of them.

```go
// Rollback the last 2 migrations
err := migrate.RollbackMigrations(ctx, source, dialect, logger, 2)

// Rollback all migrations
err = migrate.RollbackMigrations(ctx, source, dialect, logger, 0)
```

## License

[MIT](LICENSE)

---

MIT License

Copyright Maksim Kozhukh (c) 2025

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.