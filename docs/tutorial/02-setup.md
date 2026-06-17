# Chapter 02 · Setup — boot the world

> Goal: by the end of this chapter you have postgres, redis, and Temporal running locally; you've applied the migrations; you've started `gogg-api` and seen it answer a request; and the rankings page renders in your browser.

The full setup runs in about 5 minutes on a warm machine. The first time, allow 15–20 min because Docker pulls images.

## Prerequisites

Check each tool. If something's missing, install it before moving on.

```bash
go version              # need 1.26.4 or newer
node --version          # need 22.12 or newer
docker compose version  # need v2.x
sops --version          # need 3.x (encrypts deploy/secrets/dev.enc.yaml)
age --version           # any 1.x
```

If `go version` is older than 1.26.4: the toolchain is pinned in `go.mod`, so Go will auto-download the right version on the first build. You just need *some* Go installed.

If `sops` and `age` are missing: macOS `brew install sops age`; Ubuntu/Debian `sudo apt install age` (sops install instructions: <https://github.com/getsops/sops/releases>). The dev stack will work without sops — the API binary falls back to env-only config — but you'll get HTTP 401 from Riot because the API key won't be loaded.

Optional (CI runs these too):

```bash
golangci-lint version    # v1.62+
sqlc version             # v1.27+
migrate -version         # v4.18+
```

The first two are needed only when you change Go code or `*.sql`. `migrate` is needed for `make migrate-up`. macOS: `brew install golangci-lint sqlc golang-migrate`.

## Step 1 · Clone and look around

```bash
git clone https://github.com/crafff/gogg
cd gogg
```

First, just look:

```bash
ls -la
```

You should see `apps/`, `packages/`, `deploy/`, `docs/`, `internal/`, plus `go.mod`, `Makefile`, `CLAUDE.md`, `README.md`. The structure was covered in Chapter 01.

Run `make help` to see every available command:

```bash
make help
```

You'll see roughly 20 targets. Three you'll use immediately: `dev`, `migrate-up`, `run-api`.

## Step 2 · Boot the dev stack

```bash
make dev
```

This runs `docker compose -f deploy/compose/docker-compose.dev.yml up -d` and starts five containers:

| Container    | Host port    | Why                                                |
|--------------|--------------|----------------------------------------------------|
| postgres     | 55433        | Application database                               |
| redis        | 6379         | Cache + rate-limit + session state                 |
| temporal     | 7233         | Workflow engine (gRPC)                             |
| temporal-ui  | 8233         | Temporal Web UI (HTTP)                             |
| mailhog      | 1025 / 8025  | Local SMTP capture, used by future OAuth flows     |

Wait ~30 seconds, then check that everything is healthy:

```bash
docker compose -f deploy/compose/docker-compose.dev.yml ps
```

Every service should say "Up" or "Up (healthy)". If something is exiting in a loop, run `docker compose -f deploy/compose/docker-compose.dev.yml logs <service>` to see why.

💡 **Why port 55433 instead of 5432?** So a host-side postgres install doesn't fight with the container. The convention: dev port = standard port + 50000.

🛠️ **Try this**: open <http://localhost:8233> in your browser. That's the Temporal Web UI. It's empty now (no workflows ever ran). We'll come back here in Chapter 04.

## Step 3 · Apply the database migrations

```bash
make migrate-up
```

This runs `migrate` against `packages/sqlc/migrations/` and applies every file in order. You'll see lines like `1/u initial_schema (3.456ms)` for each migration.

Verify the schema:

```bash
psql 'postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable' -c '\dt'
```

Expected output (subset):

```
                  List of relations
 Schema |          Name           | Type  | Owner
--------+-------------------------+-------+--------
 public | champions               | table | gogg
 public | game_versions           | table | gogg
 public | match_participants      | table | gogg
 public | matches                 | table | gogg
 public | players                 | table | gogg
 public | player_rank_snapshots   | table | gogg
 public | runs                    | table | gogg
 public | schema_migrations       | table | gogg
 public | user_oauth_identities   | table | gogg
 public | user_refresh_tokens     | table | gogg
 public | users                   | table | gogg
```

11 tables. We'll look at the meaning of each in Chapter 03.

🛠️ **Try this**: `psql 'postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable' -c 'SELECT * FROM schema_migrations'` shows which migration is currently the head.

## Step 4 · Start the API binary

In a separate terminal (keep the dev stack running):

```bash
make run-api
```

You should see structured logs like:

```
time=... level=INFO msg="config_loaded" ...
time=... level=INFO msg="postgres_connected" ...
time=... level=INFO msg="redis_connected" ...
time=... level=INFO msg="http_server_started" addr=:8080
```

If you see `secrets_decrypt_failed`, sops isn't installed or can't find your age key. The binary still starts; it just won't have the Riot API key.

Now hit it from another terminal:

```bash
curl -s http://localhost:8080/healthz
# → ok

curl -s http://localhost:8080/readyz
# → ok

curl -s http://localhost:8080/api/v1/versions | head
# → []  (empty array — no data yet, because the crawler hasn't run)

curl -s http://localhost:8080/api/v1/regions | head
# → []  (same)
```

Empty arrays are expected — the database is fresh. The crawler hasn't ingested any data, so there's nothing to return.

Open <http://localhost:8080/graphql/playground>. That's a GraphQL IDE. Run:

```graphql
query {
  versions
  regions
}
```

You'll get `{"data": {"versions": [], "regions": []}}` — same empty answer, different envelope.

🛠️ **Try this**: hit `http://localhost:8080/metrics`. You'll see hundreds of lines of Prometheus metrics: HTTP request counters, latency histograms, Go runtime stats. That surface lights up Grafana dashboards in Phase F.

## Step 5 · Start the frontend

In yet another terminal (or stop the API and run it after — your call):

```bash
make run-web
```

You should see:

```
> @gogg/web@0.1.0 dev
> vite

  VITE v8.0.16  ready in 213 ms

  ➜  Local:   http://localhost:5173/
```

Open <http://localhost:5173/>. The page should:

1. Redirect from `/` to `/rankings`
2. Show a header with "GOGG" + tagline + nav + language switcher + a "Sign in" CTA
3. Render an empty state ("No data") because the rankings table has no rows

You can navigate to:

- `/champion/99` → placeholder page showing "championId = 99"
- `/summoner/kr/Faker` → placeholder showing "KR / Faker"
- `/login` → placeholder
- `/me` → placeholder
- `/intentionally-bogus` → 404 page from `RouteErrorBoundary`

🛠️ **Try this**: click the language switcher and toggle between zh-CN and en-US. Every visible string should update. That's react-i18next at work — Chapter 06 explains how.

🛠️ **Try this**: open browser DevTools → Network tab. Refresh. You'll see POST requests to `/graphql`. Click one and look at the request body: `{"query": "query Versions { versions }", "variables": null}`. That's the codegen'd hook posting to the GraphQL endpoint. Vite proxies `/graphql` to `:8080`, so this works in dev without CORS.

## Step 6 · (Optional) Start the worker

You don't strictly need the worker to read these chapters, but to see data in the rankings table you need a successful crawl. Without a Riot API key, the crawl will retry-then-fail at Phase 1.

```bash
make run-worker
```

Expected logs:

```
time=... level=INFO msg="temporal_client_connected" addr=localhost:7233
time=... level=INFO msg="schedule_created" id=gogg-crawl-daily_kr ...
time=... level=INFO msg="worker_polling" queue=crawl-kr
```

If you have a Riot dev key (request one at <https://developer.riotgames.com>), the Schedule will fire on its cron expression and you'll see workflow events flowing through. Chapter 04 covers this in detail.

For now, the worker can sit idle — we're just establishing that it boots cleanly.

## Step 7 · Confirm everything stops cleanly

In the terminal running `make run-api`, hit `Ctrl+C`. You should see:

```
time=... level=INFO msg="shutdown_signal" signal=interrupt
time=... level=INFO msg="http_server_stopped"
time=... level=INFO msg="bye"
```

Same for the worker. The frontend (vite) just exits on `Ctrl+C` with no ceremony.

The dev stack stays up. Bring it down explicitly when you're done:

```bash
make dev-down       # stops containers, keeps the data volume
make dev-reset      # stops + wipes the volume (next migrate-up starts from scratch)
```

## Quick reference — every URL you'll use

| URL | Purpose |
|---|---|
| <http://localhost:5173> | apps/web dev server (vite) |
| <http://localhost:8080/healthz> | API liveness |
| <http://localhost:8080/readyz> | API readiness (DB + Redis probes) |
| <http://localhost:8080/api/v1/versions> | REST: ingested patch list |
| <http://localhost:8080/api/v1/regions> | REST: regions with data |
| <http://localhost:8080/api/v1/rankings/champions?region=KR> | REST: rankings |
| <http://localhost:8080/graphql> | GraphQL endpoint (POST) |
| <http://localhost:8080/graphql/playground> | GraphQL IDE |
| <http://localhost:8080/metrics> | Prometheus metrics |
| <http://localhost:8233> | Temporal Web UI |
| <http://localhost:8025> | Mailhog Web UI |

## Up next

[Chapter 03 — Database + sqlc](./03-database.md) explores the 11 tables you just saw, what each row means, and how the `*.sql` files in `packages/sqlc/queries/` get turned into type-safe Go.
