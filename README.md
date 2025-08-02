# Migrate

A simple and flexible database migration library for Go.


## Features

-   No external dependencies
-   Transactional migrations
-   Support for multiple database dialects (PostgreSQL, SQLite, and easily extensible).
-   Migrations can be embedded in the application binary.

## Usage

Here is an example of how to use the library with migrations embedded in your application.

### 1. Create Migration Files

Create a directory for your SQL migration files. The files should be named with a version prefix, for example:

`migrations/`
- `20230101_create_users_table.sql`
- `20230102_add_email_to_users.sql`

### 2. Embed and Run Migrations

In your Go application, use the `//go:embed` directive to embed the migration files. Then, create a `Dialect` and a `Source` to run the migrations.

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
}
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
