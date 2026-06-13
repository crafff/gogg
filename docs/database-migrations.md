# Database Migrations

Gogg uses [golang-migrate](https://github.com/golang-migrate/migrate) to manage database schema changes. Migrations are plain SQL files embedded into the binary at compile time.

## How it works

On every startup (`gogg serve` or `gogg crawl run`), `store.InitSchema()` calls `migrate.Up()`, which:

1. Reads the `schema_migrations` table to find the current version
2. Applies any `.up.sql` files with a higher sequence number
3. Does nothing if already up to date

Migrations are **idempotent** — safe to run on every startup.

## File layout

```
internal/storage/migrations/
├── 001_init.up.sql       ← applied going forward
├── 001_init.down.sql     ← applied to roll back
├── 002_xxx.up.sql
└── 002_xxx.down.sql
```

Files must follow the naming pattern: `{version}_{description}.{direction}.sql`

- `version` — zero-padded integer (001, 002, …), determines order
- `description` — snake_case label, for humans only
- `direction` — `up` or `down`

Both `up` and `down` files are required for every migration.

## Adding a new migration

1. Pick the next sequence number (check existing files):

```bash
ls internal/storage/migrations/
```

2. Create the two files:

```bash
touch internal/storage/migrations/002_add_champion_alias.up.sql
touch internal/storage/migrations/002_add_champion_alias.down.sql
```

3. Write the SQL:

**`002_add_champion_alias.up.sql`**
```sql
ALTER TABLE players ADD COLUMN IF NOT EXISTS champion_alias text;
```

**`002_add_champion_alias.down.sql`**
```sql
ALTER TABLE players DROP COLUMN IF EXISTS champion_alias;
```

4. Restart the server or run the crawler — the migration applies automatically.

## Rolling back

golang-migrate does not auto-rollback. To manually step down one version, use the CLI tool:

```bash
# Install once
go install -tags 'pgx5' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Roll back one step
migrate -source file://internal/storage/migrations \
        -database "pgx5://gogg:goggpass@localhost:55433/gogg?sslmode=disable" \
        down 1
```

## Checking current version

```bash
migrate -source file://internal/storage/migrations \
        -database "pgx5://gogg:goggpass@localhost:55433/gogg?sslmode=disable" \
        version
```

Or query directly:

```sql
SELECT version, dirty FROM schema_migrations;
```

## Dirty state

If a migration fails mid-way, `schema_migrations.dirty` is set to `true` and future runs will refuse to proceed. Fix the underlying SQL error, then force-set the version to clear the dirty flag:

```bash
migrate -source file://internal/storage/migrations \
        -database "pgx5://gogg:goggpass@localhost:55433/gogg?sslmode=disable" \
        force 1
```

Replace `1` with the version number shown in the `dirty` row.

## Migration history

| # | Name | Key changes |
|---|------|-------------|
| 001 | init | 全部基础表：`players`, `runs`, `matches`, `match_participants`, `match_perks`, `match_bans`, `match_teams`, `match_timelines`, `player_rank_snapshots`, `player_match_sync`, `game_versions` |
| 002 | game_versions_patch_start | `game_versions` 加 `patch_start_at` 列 |
| 003 | runs_full_config | `runs` 加 `version`, `rank_prefetch_tiers`, `queue`, `execution` |
| 004 | region | `runs`/`matches`/`player_rank_snapshots` 加 `region`；`player_match_sync` 主键改为 `(puuid, region)` |
| 005 | players_region | `players` 加 `region` |
| 006 | matches_version | `matches` 加 `version`（Phase 2 写入 run 绑定版本） |
| 007 | matches_avg_tier | `matches` 加 `avg_tier`, `avg_division` |
| 008 | timeline | `matches` 加 `timeline_status`；新建 `match_item_events`, `match_skill_events`, `match_participant_snapshots` |
| 009 | item_catalog | `matches` 加 `items_status`；新建 `item_catalog`, `match_completed_items` |
| 010 | timeline_retry | `matches` 加 `timeline_retry_count`, `items_retry_count` |
| 011 | starter_boots | `match_completed_items` 加 `is_boots`；新建 `match_starter_items`；`item_catalog` 加 `is_skippable` |
| 012 | match_boots | 新建 `match_boots`（每参与者一行，最终鞋子） |

## Guidelines

- Never edit or delete an already-deployed migration file — create a new one instead.
- Keep each migration focused on one change.
- Always write a working `down` migration; it is required even if rollback is unlikely.
- Use `IF EXISTS` / `IF NOT EXISTS` guards where PostgreSQL supports them to make migrations re-runnable during development.
