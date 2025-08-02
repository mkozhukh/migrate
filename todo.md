# TODO

This file contains a list of potential features and improvements for the migration library.

### 1. Go-Based Migrations

Support for migrations written in Go for complex data transformations.

-   [ ] Define a `GoMigration` interface with `Up(ctx context.Context, tx *sql.Tx) error` and `Down(ctx context.Context, tx *sql.Tx) error` methods.
-   [ ] Update the `Source` to discover and register Go migrations.

