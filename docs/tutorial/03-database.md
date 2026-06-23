# Chapter 03 · Database + sqlc

> Goal: by the end of this chapter you understand the 11 tables, can run a query against any of them, can read a migration file, and know how `*.sql` becomes type-safe Go code via sqlc.

This chapter assumes you have the dev stack from Chapter 02 running (`make dev` + `make migrate-up` succeeded). If you skipped it, run those two commands now.

## Why PostgreSQL?

The original MVP used Postgres, and the refactor kept it. Reasons:

- Strong consistency — the rankings page reading the same data the crawler just wrote without race conditions.
- `text[]` array columns for things like `target_tiers` on a crawler run.
- `jsonb` for the match timeline blob.
- Cheap aggregates (`GROUP BY` over hundreds of thousands of `match_participants` rows is fast with the right indexes).

If you've never used Postgres specifically, the SQL you'll read here is mostly standard. The Postgres-isms are flagged when they appear.

## The 11 tables, in two groups

### Group A — Crawler ingest (8 tables)

These are written by `apps/worker` and read by `apps/api`.

| Table | One row =  | Written by | Read by |
|---|---|---|---|
| `game_versions` | one Riot patch (e.g. "14.20.1") | Phase 0 | Phase 1+, rankings filters |
| `players` | one summoner (identified by PUUID) | Phase 1, Phase 3.5 | rankings join, summoner page |
| `player_rank_snapshots` | one player's rank at the time of a crawl | Phase 1, Phase 3.5 | summoner history |
| `runs` | one crawl invocation | every Phase | audit, rollback |
| `matches` | one ranked match | Phase 2 | rankings aggregate |
| `match_participants` | one player in one match (10 per match) | Phase 3 | rankings aggregate |
| `match_timelines` | optional per-match event blob | Phase 5 | (champion detail, Phase E) |
| `items` | static item catalog | Phase 5.5 | (champion detail, Phase E) |

### Group B — User accounts (3 tables)

These are written by `apps/api` (login / refresh / favorites flows).

| Table | One row = |
|---|---|
| `users` | one registered account |
| `user_oauth_identities` | one OAuth provider link per user (one user can have Discord + Google) |
| `user_refresh_tokens` | one active refresh token (revoke-able) |

## Read a migration to understand a table

Migrations live in `packages/sqlc/migrations/` and use [golang-migrate](https://github.com/golang-migrate/migrate) naming: `NNN_label.up.sql` to apply, `NNN_label.down.sql` to roll back. Each file is **forward-only in spirit** — the down migrations exist for local iteration only; production rolls forward.

Open `packages/sqlc/migrations/001_init.up.sql`. The very first table is `game_versions`:

```sql
CREATE TABLE IF NOT EXISTS game_versions (
    id         serial primary key,
    version    text not null unique,
    fetched_at timestamptz not null,
    is_latest  boolean not null default false
);
```

That's it. `version` is the human string like `"14.20.1"`. `is_latest` is true for exactly one row at any time (Phase 0 flips it).

Now look at `players`:

```sql
CREATE TABLE IF NOT EXISTS players (
    puuid      text primary key,
    game_name  text,
    tag_line   text,
    ...
);
```

`puuid` is Riot's stable player identifier (a long opaque string). `game_name` + `tag_line` are the display name + region tag the user picks ("Faker#KR1"). Players can rename, so we don't trust those; PUUID is the join key everywhere.

### Try this

```bash
# Look at the 001 migration to see what tables existed at MVP time
head -100 packages/sqlc/migrations/001_init.up.sql

# Compare with the 013_users migration that Phase B added
cat packages/sqlc/migrations/013_users.up.sql
```

`013_users` is the only Phase-introduced migration so far — the user system tables. Everything 001–012 came from the MVP schema.

🛠️ **Exercise**: `cat` the matching `.down.sql` for `013_users`. Notice it's a `DROP TABLE` plus index drops. The down migration is the inverse of the up; both must be kept in sync.

## Connect with psql and poke around

```bash
# Replace the DSN if you've overridden it; default is in the Makefile
DEV_DSN='postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable'

# List every table
psql "$DEV_DSN" -c '\dt'

# Describe one table
psql "$DEV_DSN" -c '\d+ matches'

# Count rows in each (will be 0 until the crawler runs)
psql "$DEV_DSN" -c 'SELECT
  (SELECT count(*) FROM game_versions) AS versions,
  (SELECT count(*) FROM players)       AS players,
  (SELECT count(*) FROM matches)       AS matches,
  (SELECT count(*) FROM runs)          AS runs;'
```

🛠️ **Exercise**: insert a fake row manually and read it back:

```sql
psql "$DEV_DSN" <<EOF
INSERT INTO game_versions (version, fetched_at, is_latest)
VALUES ('14.99.1', now(), true);

SELECT * FROM game_versions;
EOF
```

You just simulated what Phase 0 does. Now hit the API:

```bash
curl -s http://localhost:8080/api/v1/versions | head
```

The endpoint should return `["14.99.1"]`. That's a complete data-plane test without the crawler.

When you're done, clean up:

```bash
psql "$DEV_DSN" -c "DELETE FROM game_versions WHERE version = '14.99.1';"
```

## The sqlc pipeline: SQL in, Go out

Hand-writing `pgx` query code for every operation is repetitive and error-prone. sqlc lets you write the SQL once and generates a type-safe Go function for it.

### The three pieces

1. **Schema** (`packages/sqlc/migrations/*.up.sql`): all `CREATE TABLE` statements. sqlc parses these to understand columns + types.
2. **Queries** (`packages/sqlc/queries/*.sql`): SQL statements annotated with `-- name: Foo :one|:many|:exec` directives.
3. **Generated code** (`packages/sqlc/gen/*.go`): typed structs + functions emitted by `sqlc generate`.

### Walk through one query

Open `packages/sqlc/queries/versions.sql`:

```bash
cat packages/sqlc/queries/versions.sql
```

You'll see something like:

```sql
-- name: ListVersionsWithData :many
SELECT version
FROM matches
WHERE fetch_status = 'done'
GROUP BY version
ORDER BY version DESC;
```

The `-- name: ListVersionsWithData :many` comment tells sqlc:

- Function name → `ListVersionsWithData`
- Return cardinality → `:many` (a slice). Other options: `:one`, `:exec`.

Now look at what sqlc produced:

```bash
grep -A 20 'func .* ListVersionsWithData' packages/sqlc/gen/versions.sql.go
```

You'll see:

```go
func (q *Queries) ListVersionsWithData(ctx context.Context) ([]string, error) {
    rows, err := q.db.Query(ctx, listVersionsWithData)
    if err != nil { return nil, err }
    defer rows.Close()
    var items []string
    for rows.Next() {
        var version string
        if err := rows.Scan(&version); err != nil { return nil, err }
        items = append(items, version)
    }
    if err := rows.Err(); err != nil { return nil, err }
    return items, nil
}
```

No magic, just safe boilerplate. The caller does:

```go
versions, err := queries.ListVersionsWithData(ctx)
```

and gets a `[]string` back, with `pgxpool` connection handling, scan errors, etc., all done correctly.

### When inputs are present

Look at `regions.sql`:

```bash
cat packages/sqlc/queries/regions.sql
```

You'll see a query that filters by something. sqlc converts the `@name::type` placeholder pattern into a typed struct argument. The generated Go signature becomes something like:

```go
func (q *Queries) ListRegionsWithData(ctx context.Context, arg ListRegionsWithDataParams) ([]string, error)
```

with `ListRegionsWithDataParams` being a generated struct holding the named parameters.

### When to *not* use sqlc

sqlc requires statically knowable SQL. If you need to build a `WHERE` clause dynamically (e.g. "if filter.position is set, AND on it; otherwise don't"), sqlc can't help — you'd write raw `pgx` instead and leave a `// sqlc-skip: <reason>` comment so a reviewer knows it wasn't a copy-paste oversight.

Phase B chunk 4 has exactly one such case in `apps/worker/internal/activity/crawl/phase0.go`. Search for `sqlc-skip` to find it:

```bash
grep -rn 'sqlc-skip' apps/ packages/ 2>/dev/null
```

That convention is in [ADR-0002](../architecture/adr/0002-sqlc-over-ent.md): sqlc is the default, raw pgx is the documented escape hatch.

## The rankings query — the most complex one

Open `packages/sqlc/queries/rankings.sql`. This is the query that answers "which champions have the best win rate in this slice?" It's a CTE chain:

1. `filtered_matches`: which matches match the filter (queue, version, region, tier)?
2. `champion_position_games`: count games per (champion, position) over those matches.
3. `champion_totals`: roll up to per-champion totals + bans.
4. Top-level `SELECT`: join everything, compute rates, apply `min_games` floor + `row_limit`.

The same CTE shape repeats for `ListOverallRankings` and `ListRankingsByPosition`. Yes, there's duplication — the comment at the top of the file explains why we kept it for parity with the legacy code:

> The filtered_matches CTE is duplicated between the two queries because sqlc emits one Go function per query and has no cross-query CTE sharing. Folding it into a postgres view is a future optimisation; doing it during Phase B is out of scope — we ship parity first, refactor later.

This is a recurring pattern in the codebase: optimal architecture vs ship-now pragmatism, with a comment explaining the tradeoff so future-you can do better.

🛠️ **Exercise**: run `ListOverallRankings` directly against psql, mocking the named params:

```bash
psql "$DEV_DSN" <<'EOF'
WITH filtered_matches AS (
    SELECT m.match_id
    FROM matches m
    WHERE m.queue_id = 420
      AND m.fetch_status = 'done'
)
SELECT count(*) FROM filtered_matches;
EOF
```

It returns 0 (no matches ingested yet). The full query expands from there.

## Regenerating after a schema or query change

When you add a new query or change a table:

```bash
make gen-sqlc
```

That regenerates `packages/sqlc/gen/`. **Commit the generated files** — treat them like protobuf output: read-only, regenerated on change.

If you forget, CI will fail because generated code will differ from
what is checked in. `packages/sqlc/migrations/` is the only migration
tree; do not mirror migrations anywhere else.

## Exercises before moving on

1. **Identify the join keys**: open three migration files at random. For each table, find the foreign keys (lines with `REFERENCES other_table`). Sketch the relationship diagram on paper. Compare with what you see in `\d+ <table>` output.

2. **Write a fake report**: with the database empty, run:
   ```sql
   INSERT INTO matches (match_id, region, version, queue_id, fetch_status, fetched_at)
     VALUES ('FAKE_1', 'KR', '14.20.1', 420, 'done', now());
   ```
   Now hit `/api/v1/versions`. It should return `["14.20.1"]`. Delete the row.

3. **Trace `is_latest`**: `git grep is_latest packages/sqlc/queries/` to see which queries select on it. Then `git grep is_latest apps/worker/` to see which crawler activity sets it. That's how the rankings page decides "version: latest" resolution.

## Up next

[Chapter 04 — Crawler + Temporal](./04-crawler-temporal.md) shows what fills these tables. You'll learn the 8-phase pipeline, watch a workflow execute in the Temporal UI, and understand why "Phase 1 failed at 401" results in 5 automatic retries with exponential backoff.
