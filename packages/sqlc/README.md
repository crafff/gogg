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

## Migration transition note

During the Phase A–B refactor, the migrations also live at
`internal/storage/migrations/` because the legacy
`internal/storage/schema.go` embeds them via `//go:embed`. Both
copies must stay byte-identical until the legacy storage layer
is deleted in Phase B. **Always create new migrations in
`packages/sqlc/migrations/` first**, then `cp` to the legacy
path in the same commit. A CI check enforces this (see
`.github/workflows/ci.yml`).
