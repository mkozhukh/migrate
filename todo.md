# TODO

This file contains a list of potential features and improvements for the migration library.

### 1. Go-Based Migrations

Support for migrations written in Go for complex data transformations.

-   [ ] Define a `GoMigration` interface with `Up(ctx context.Context, tx *sql.Tx) error` and `Down(ctx context.Context, tx *sql.Tx) error` methods.
-   [ ] Update the `Source` to discover and register Go migrations.

### 3. Targeted Migrations

Allow migrating up or down to a specific version, not just all the way or by a number of steps.

-   [ ] Update `RunMigrations` and `RollbackMigrations` to accept an optional `version` string.

### 4. Dry Run Mode

Add a "dry run" mode to show which migrations would be applied without actually changing the database.

-   [ ] Add a `dryRun` boolean flag to `RunMigrations` and `RollbackMigrations`.
