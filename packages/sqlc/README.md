# packages/sqlc

Single source of truth for SQL — both **migrations** (DDL) and
**queries** (DML). Type-safe Go bindings are generated into
`gen/` via [sqlc](https://sqlc.dev).

## Layout

```
sqlc.yaml          — codegen config
migrations/        — golang-migrate up/down pairs (NNN_name.up.sql / .down.sql)
queries/           — *.sql files; each statement annotated with `-- name: …` and a sqlc verb (:one, :many, :exec)
gen/               — generated code (committed; do not edit by hand)
```

## Common tasks

```bash
# Apply migrations to local dev DB
make migrate-up

# Roll back the most recent migration
make migrate-down

# Create a new migration
make migrate-new name=add_user_favorites

# Regenerate Go bindings after editing queries
make gen-sqlc
```

## Migration ownership

`packages/sqlc/migrations/` is the only migration tree. Add every new
schema change here, regenerate bindings with `make gen-sqlc`, and let
application code consume the generated `packages/sqlc/gen` package.
