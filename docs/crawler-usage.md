# Crawler Usage Guide

## Setup

Build the `gogg` binary:

```bash
go build -o gogg .
```

### 1. Config file

```bash
cp config.example.yaml config.yaml
```

**Single-region setup (backward compatible):**

```yaml
riot:
  api_key: "RGAPI-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
  base_url: "https://kr.api.riotgames.com"

database:
  dsn: "postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable"
```

**Multi-region setup:**

```yaml
riot:
  api_key: "RGAPI-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"  # shared fallback

regions:
  - name: KR
    base_url: "https://kr.api.riotgames.com"
  - name: NA1
    base_url: "https://na1.api.riotgames.com"
    api_key: "RGAPI-different-key"  # optional per-region override

database:
  dsn: "postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable"
```

### 2. Start the database

```bash
docker compose up -d
```

Schema and all migrations are applied automatically on first run via `InitSchema`. On subsequent runs (including `--resume`), only new pending migrations are applied.

---

## Database connection

| Field    | Value |
|----------|-------|
| Host     | `localhost:55433` |
| Database | `gogg` |
| User / Password | `gogg` / `goggpass` |
| DSN | `postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable` |

```bash
PGPASSWORD=goggpass psql -h localhost -p 55433 -U gogg -d gogg
```

---

## Running the crawler

```
gogg crawl [subcommand] [flags]
```

Global flag: `--config <path>` (default: `config.yaml`)

### `run` — execute a crawl run

```bash
./gogg crawl run --profile daily_kr
./gogg crawl run --profile daily_kr --tiers CHALLENGER
./gogg crawl run --tiers CHALLENGER,GRANDMASTER --region KR
./gogg crawl run --resume 42
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--profile` | Named profile from `run_profiles` in config |
| `--tiers` | Comma-separated target tiers (overrides profile) |
| `--mode` | `incremental` or `historical` |
| `--version` | Game version for historical mode (e.g. `14.10`) |
| `--execution` | `pipeline` or `sequential` |
| `--region` | Region name (e.g. `KR`, `NA1`); auto-detected if only one region configured |
| `--resume` | Resume a previously interrupted run by ID |

### `status` — show current/last run

```bash
./gogg crawl status
```

### `runs` — list recent runs

```bash
./gogg crawl runs [--limit 20]
```

### `cancel` — cancel a specific run

```bash
./gogg crawl cancel <run_id>
```

### `daemon` — scheduled crawling

```bash
./gogg crawl daemon
```

Reads the `schedule` block from config. Multiple entries with the same cron run in parallel — useful for multi-region:

```yaml
schedule:
  - cron: "0 4 * * *"
    profile: daily_kr
  - cron: "0 4 * * *"     # same time, runs in parallel with daily_kr
    profile: daily_na
  - cron: "0 3 * * 1"
    profile: diamond_weekly_kr
```

---

## Run profiles

```yaml
run_profiles:

  daily_kr:
    region: KR
    mode: incremental
    target_tiers: [CHALLENGER, GRANDMASTER, MASTER]
    rank_prefetch_tiers: [CHALLENGER, GRANDMASTER, MASTER, DIAMOND]
    queue: RANKED_SOLO_5x5
    execution: pipeline

  patch_historical_kr:
    region: KR
    mode: historical
    version: "14.10"
    target_tiers: [CHALLENGER, GRANDMASTER]
    rank_prefetch_tiers: [CHALLENGER, GRANDMASTER, MASTER]
    queue: RANKED_SOLO_5x5
    execution: sequential
```

| Field | Description |
|-------|-------------|
| `region` | Must match a `regions[].name` entry |
| `mode` | `incremental` = since last run; `historical` = full version |
| `version` | Required for historical mode |
| `target_tiers` | Tiers to collect match data for (Phase 2) |
| `rank_prefetch_tiers` | Tiers to snapshot ranks for (Phase 1); should be ≥ target_tiers |
| `execution` | `pipeline` (default) or `sequential`; see below |

**Execution modes:**

- `pipeline` (default) — Phase 0/1 run once upfront; Phases 2–5.5 run per tier. Phase 5/5.5 are tier-agnostic and complete all pending work on the first tier iteration; subsequent tiers skip them immediately.
- `sequential` — all phases run in strict order, one after another.

---

## Crawl phases

| Phase | Name | What it does |
|-------|------|-------------|
| 0 | Version Sync | Fetches patch list from CommunityDragon; pins `version` on the run |
| 1 | Rank Snapshot | Pulls Challenger/GM/Master/Diamond entries → `player_rank_snapshots` |
| 2 | Match IDs | Fetches match IDs for each player within the version time window |
| 3 | Match Details | Downloads full match data; infers participant tiers from snapshots |
| 3.5 | Tier Backfill | On-demand rank lookup for participants with `tier_at_match = NULL`; marks permanently unranked players as `UNRANKED` |
| 4 | Avg Tier Calc | Computes `avg_tier`, `avg_division`, `avg_tier_score` per match using dynamic apex thresholds from current run |
| 5 | Timeline | Fetches timeline per match; extracts item events, skill level-ups, and 5-min snapshots |
| 5.5 | Item Classification | Classifies item events into completed items (大件), starter items (出门装), and boots using CDragon catalog |

---

## Retry behavior

Phases 3, 5, and 5.5 use a shared retry strategy:

| `retry_count` | Status after failure | Next run |
|---|---|---|
| 0 → 1 | `pending` | Retried |
| 1 → 2 | `pending` | Retried |
| 2 → 3 | `error` | Not retried |

---

## Resume

Press `Ctrl+C` at any time — the crawler stops cleanly after the current API call. Resume with:

```bash
./gogg crawl run --resume <run_id>
```

**Skip behavior on resume:**
- Interrupted during Phase 0 or 1 → restarts from Phase 0 (fast, clean slate)
- Interrupted during Phase 2 or later → skips already-completed phases, resumes from checkpoint

---

## Clearing match data

```sql
TRUNCATE matches, match_participants, match_perks, match_bans, match_teams,
         match_item_events, match_skill_events, match_participant_snapshots,
         match_completed_items, match_starter_items, match_boots
  RESTART IDENTITY CASCADE;
TRUNCATE player_match_sync;
```

---

## Testing

### Run unit tests (requires local DB)

```bash
go test ./internal/crawler/phase3/
```

### Full pipeline integration test (requires Riot API key)

```bash
RIOT_API_KEY=<key> go test -tags integration -v -timeout 20m \
  ./internal/crawler/ -run TestFullPipelineReduced
```

Phase 1 keeps 3 players per tier, Phase 2 runs normally then prunes to ~9 matches, Phases 3–5.5 run on those matches. Schema `crawl_inttest` is preserved for inspection.

```bash
go run ./cmd/inttest-cleanup/   # drop the test schema when done
```

---

## Database tables

| Table | Written by | Description |
|-------|-----------|-------------|
| `game_versions` | Phase 0 | Patch version history |
| `players` | Phase 1/3 | Player registry (puuid, region) |
| `runs` | Runner | Crawl run history and checkpoints |
| `player_rank_snapshots` | Phase 1/3.5 | Rank snapshots per player per run (region-scoped) |
| `player_match_sync` | Phase 2 | Last match-sync timestamp per player per region |
| `matches` | Phase 2/3/4/5/5.5 | Match headers with status columns for each phase |
| `match_participants` | Phase 3/3.5 | Per-player per-match stats (~100 columns); `tier_at_match='UNRANKED'` for unranked players |
| `match_perks` | Phase 3 | Rune selections |
| `match_bans` | Phase 3 | Champion bans |
| `match_teams` | Phase 3 | Team-level objectives |
| `item_catalog` | Phase 5.5 | CDragon item metadata per patch |
| `match_item_events` | Phase 5 | Item purchase sequence with `removal_type` (null/undo/sold) |
| `match_skill_events` | Phase 5 | Skill level-up sequence (Q/W/E/R + EVOLVE) |
| `match_participant_snapshots` | Phase 5 | 5-min interval stats (gold, CS, damage, wards...) |
| `match_completed_items` | Phase 5.5 | Completed items (大件) slots 1-6, `is_boots` flagged |
| `match_starter_items` | Phase 5.5 | Starter items (出门装, cumulative ≤500g, within 90s) |
| `match_boots` | Phase 5.5 | Latest completed boots per participant |
