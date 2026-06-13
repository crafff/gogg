# ADR-0002: sqlc for data access (over ent / GORM / hand-written)

- Status: Accepted
- Date: 2026-06-12
- Deciders: GOGG maintainers

## Context

The pre-refactor codebase uses raw `pgx` with hand-written SQL
in `internal/server/rankings.go` and `internal/storage/*.go`.
Two queries in `rankings.go` (`GetOverallRankings` and
`GetRankingsByPosition`) share ~80% of their CTE logic; the SQL
is correct but the duplication is a maintenance hazard. We need
to settle on a data access pattern before adding the dozen
queries Phase B will introduce.

Options:

1. **Continue hand-written `pgx`.** Pros: no abstraction,
   maximum SQL visibility. Cons: every query needs hand-written
   `Scan` boilerplate; types drift from the schema; CTE reuse
   is informal.
2. **ent (Facebook's ORM).** Pros: graph-friendly, code-first
   schema. Cons: heavy abstraction; the generated SQL is opaque
   when you're tuning a 200-line CTE; doesn't compose well with
   PG features (LATERAL joins, window functions, materialized
   views) we already rely on.
3. **GORM.** Pros: familiar to Rails refugees. Cons: ORM-shaped
   query language is a poor fit for the analytical workload
   (rankings aggregation across millions of match_participants
   rows); query plans become hard to predict.
4. **sqlc.** Write SQL files in `packages/sqlc/queries/`; sqlc
   reads them, validates against the schema reconstructed from
   migrations, and generates strongly typed Go method signatures.

## Decision

**Adopt sqlc.** All query code lives in
`packages/sqlc/queries/*.sql` and is regenerated via `make
gen-sqlc`. The generated package is committed.

## Rationale

- **SQL stays first-class.** Every query is a real, readable
  SQL file we can EXPLAIN, lint, and copy-paste into psql. No
  surprise generated SQL.
- **Types stay in sync.** sqlc reads the migrations and produces
  Go types that match the columns. A `DROP COLUMN` migration
  surfaces as a Go compile error in every query that referenced
  it.
- **Boilerplate gone.** No more `rows.Scan(&champion.ID,
  &champion.Name, &champion.WinRate, ...)` per query.
- **Plays nice with PG features.** sqlc understands CTEs, window
  functions, arrays, JSONB, LATERAL — the things ORMs choke on.
  This matters: our rankings query is exactly that kind of
  query.
- **Compatible with the existing pgxpool setup.** sqlc emits
  code that takes a `*pgxpool.Pool` (or any `DBTX`), so we keep
  our connection pool tuning and add nothing at runtime.

## Consequences

### Positive

- Query duplication becomes obvious (two files), so refactoring
  shared CTEs into views or query fragments is incentivised.
- Migration discipline tightens: schema changes that break
  queries fail `make gen-sqlc`, which CI runs.
- Newcomers can read `queries/rankings.sql` to learn the data
  model; no need to grep through Go for hand-rolled scans.

### Negative

- sqlc cannot express dynamic queries (variable WHERE clauses
  built from filter params). Workaround: write multiple
  query variants (e.g. `GetRankingsByPosition` vs
  `GetOverallRankings`), or use `sqlc.narg` patterns where
  appropriate, or fall back to hand-written queries for the
  truly dynamic case (acceptable; we already do this).
- The generated `gen/` directory is committed, producing larger
  PRs on schema changes. Mitigation: CI checks that
  `make gen-sqlc` is a no-op on every commit, so generated code
  drift is caught at PR review.

## Alternatives considered

- **ent**: rejected because the schema is mostly "wide,
  analytical, denormalised match_participants" and the workload
  is "complex aggregation". Both are anti-fits for an
  entity-graph ORM.
- **GORM**: rejected for the same reason plus an ecosystem
  preference (sqlc has a sharper community and clearer
  trajectory in the Go data space).
- **Hand-written pgx**: rejected because we are about to write
  ~30 new queries in Phase B, and the boilerplate cost
  compounds.
