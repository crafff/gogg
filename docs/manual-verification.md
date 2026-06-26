# Manual verification guide

A step-by-step walkthrough for confirming the post-Phase-D stack
works end-to-end on a fresh clone. Each section is self-contained:
run them in order on the first pass, then individually as a smoke
when you suspect drift.

If a step fails, jump to **Troubleshooting** at the bottom and look
up the symptom — most of the rough edges have known fixes.

---

## 0 · Prerequisites

```bash
go version          # 1.26.4 or newer
node --version      # 22.12 or newer
docker compose version
sops --version      # any 3.x
age --version       # any 1.x
```

Optional but recommended for the full quality-gate experience:

```bash
golangci-lint version    # v1.62+
sqlc version             # v1.27+
migrate -version         # v4.18+
lefthook version         # any v1.x
```

If `sops` isn't installed, `make run-api` / `make run-worker`
silently fall back to env-only config — the binaries still start,
but the Riot API key won't be loaded so any crawl will fail with
HTTP 401.

---

## 1 · Boot the dev stack

```bash
make dev
```

That brings up five containers via `deploy/compose/docker-compose.dev.yml`:

| Service   | Host port | Purpose                              |
|-----------|-----------|--------------------------------------|
| postgres  | 55433     | Application database                 |
| redis     | 6379      | Cache + rate limit + sessions        |
| temporal  | 7233      | Workflow engine (gRPC)               |
| temporal-ui | 8233    | Temporal Web UI                      |
| mailhog   | 1025 / 8025 | SMTP + Web UI for dev mail capture |

Verify each one is responsive:

```bash
# Postgres
docker exec -it $(docker ps -qf name=postgres) psql -U gogg -d gogg -c '\dt' | head

# Redis
docker exec -it $(docker ps -qf name=redis) redis-cli ping        # → PONG

# Temporal UI
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8233    # → 200

# Mailhog UI
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8025    # → 200
```

If any container failed to start, `docker compose -f deploy/compose/docker-compose.dev.yml logs <service>` shows the reason.

---

## 2 · Apply migrations

```bash
make migrate-up
```

Then confirm the schema is present:

```bash
psql 'postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable' \
    -c '\dt'
```

Expected tables (subset): `champions`, `match_participants`, `matches`, `players`, `player_rank_snapshots`, `runs`, `game_versions`, `users`, `user_oauth_identities`, `user_refresh_tokens`, plus the `schema_migrations` audit table.

To wipe and re-apply (useful when iterating on migrations):

```bash
make dev-reset    # destroys volumes — data loss!
make dev
make migrate-up
```

---

## 3 · Smoke the apps/api binary

In one terminal:

```bash
make run-api
```

Expected: structured slog lines ending in `http_server_started addr=:8080`. In another terminal, walk the surface:

```bash
# Liveness — process is up
curl -s http://localhost:8080/healthz                          # → ok

# Readiness — DB + Redis reachable
curl -s http://localhost:8080/readyz                           # → ok

# Catalog
curl -s http://localhost:8080/api/v1/versions | jq
curl -s http://localhost:8080/api/v1/regions  | jq

# Rankings (will return empty arrays until the worker has crawled)
curl -s 'http://localhost:8080/api/v1/rankings/champions?region=KR' | jq '.items | length'

# Prometheus metrics
curl -s http://localhost:8080/metrics | head -20

# GraphQL playground in the browser
open http://localhost:8080/graphql/playground
```

Inside the playground, run:

```graphql
query { versions  regions }
```

The response should mirror the REST output for the same fields.
ADR-0003 covers the one intentional divergence.

If `/readyz` returns 503, Redis or Postgres dropped — re-check `make dev`'s container health.

---

## 4 · Smoke the apps/worker binary

In another terminal:

```bash
make run-worker
```

Expected logs:

- `temporal_client_connected addr=localhost:7233`
- `schedule_created` or `schedule_updated` (once per profile in the unified worker config)
- worker pollers registered on `crawl-kr` and `crawl-na1` task queues

Trigger a workflow manually via Temporal CLI from the host:

```bash
# Get the temporal container's internal IP because Temporal binds to
# its container address, not localhost
TEMPORAL_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' \
    $(docker ps -qf name=temporal | head -1))

temporal --address $TEMPORAL_IP:7233 workflow start \
    --task-queue crawl-kr \
    --type CrawlRegionWorkflow \
    --workflow-id manual-test-kr-$(date +%s) \
    --input '{"ProfileName":"daily_kr"}'
```

Then watch it execute in the Temporal Web UI at <http://localhost:8233>:

- Workflow should walk Phase 0 → Phase 1 → Phase 2 → Phase 3 → Phase 3.5 → Phase 4 → Phase 5 → Phase 5.5 → CompleteRun.
- Each activity records its own input + output in the event history.
- If your dev Riot key expired, Phase 1 will retry 5 times (5s → 10s → 20s → 40s → 80s) then FailRun stamps `runs.status = 'failed'`.

To cancel a stuck workflow:

```bash
temporal --address $TEMPORAL_IP:7233 workflow cancel \
    --workflow-id manual-test-kr-<timestamp>
```

---

## 5 · Smoke the apps/web frontend

With `make run-api` still running, in a third terminal:

```bash
make run-web
```

Open <http://localhost:5173>. Expected:

1. The brand header shows "GOGG" + tagline + nav + language switcher + login CTA.
2. `/` redirects to `/rankings`.
3. The rankings table renders (loading skeleton first, then either real rows or an empty state if the crawler hasn't run yet).
4. The language switcher toggles between zh-CN and en-US; the table column headers + nav labels update.
5. Click a position chip (e.g. "Top" / "上单") → the table fades out, refetches, fades back in with the filtered slice.
6. Scroll to the bottom of the table → "Loading more…" appears → 40 more rows load (assuming the dataset has them).
7. Navigate to `/champion/99`, `/summoner/kr/Faker`, `/login`, `/me` → each shows the placeholder page with the URL param echoed.
8. Visit a bogus path like `/this/does/not/exist` → `RouteErrorBoundary` renders the localized 404 panel.

Browser DevTools network tab should show `/graphql` POST requests for `Versions`, `Regions`, and `ChampionRankings` — all proxied to `:8080` by vite.

---

## 6 · Run the test suites

### 6.1 Go tests

```bash
go test ./...                                    # ~30s
go test -race ./apps/worker/internal/workflow/...  # race-detector pass
```

All packages should report `ok` or `cached`. The worker workflow tests need `-race` to pass (CI runs them that way).

### 6.2 Integration tests

These require the compose stack from step 1 to be running:

```bash
GOGG_INTTEST=1 make test-int
```

Integration tests are responsible for their own test data setup and cleanup.

### 6.3 Web unit tests

```bash
cd apps/web
npm test
```

Expected: 13 files / 39 cases pass.

### 6.4 Web type-check + lint + build

```bash
cd apps/web
npm run type-check
npm run lint
npm run build
```

Build dist is roughly 11.7 kB CSS gzipped 3.3 kB + 354 kB JS gzipped 115 kB.

### 6.5 Web codegen

```bash
cd apps/web
npm run codegen
git diff --stat src/shared/api/generated/
```

Diff should be empty — if the gqlgen schema in `apps/api/internal/transport/graphql/schema/*.graphql` changed without re-running codegen, this surfaces it.

### 6.6 Playwright e2e

```bash
# One-time: install chromium binary
make test-e2e-install

# If on Linux without sudo, you may also need the system libraries:
sudo apt-get install -y libnspr4 libnss3 libatk1.0-0 libatk-bridge2.0-0 \
    libcups2 libxcomposite1 libxdamage1 libxfixes3 libxrandr2 libgbm1 \
    libpango-1.0-0 libcairo2 libasound2

make test-e2e
```

The golden path test boots vite on port 5173 (via the `webServer` config), stubs `/graphql` via `page.route("**/graphql")`, and walks: `/` → `/rankings` → 40 rows → click TOP chip → 40 TOP rows → scroll → 80 rows.

If your CI doesn't have the apt libraries, run e2e inside the `mcr.microsoft.com/playwright:v1.61.0-focal` Docker image instead.

### 6.7 Quality gate parity with CI

```bash
make ci
```

That's `vet + lint + test` together — exactly what `.github/workflows/ci.yml` runs server-side.

---

## 7 · Secrets workflow

The dev stack works without sops, but the Riot API key + JWT secret require it. To verify:

```bash
# Decrypt and inspect (does not write to disk)
sops -d deploy/secrets/dev.enc.yaml | head -20

# Add yourself as a recipient (if your age public key isn't in .sops.yaml yet)
sops -r -i deploy/secrets/dev.enc.yaml
```

After re-encryption, commit the updated `dev.enc.yaml`. Local plaintext
`config/*.yaml` files are gitignored — never check them in.

---

## 8 · Build the production binaries

```bash
make build         # both binaries
ls bin/
# bin/gogg-api  bin/gogg-worker

./bin/gogg-api -h     # confirms the binary's flags surface
```

`make build-web` is the equivalent for the SPA — output lands in `apps/web/dist/`.

---

## Troubleshooting

### `make dev` hangs / containers exit immediately

Stale data in the temporal volume can crash 1.25.2 on boot. Reset:

```bash
make dev-reset
make dev
```

### `temporal` CLI from the host can't dial 127.0.0.1:7233

Temporal binds inside the container to the container IP, not the host loopback. Use the container's IP:

```bash
docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' \
    $(docker ps -qf name=temporal | head -1)
```

Pass that as `--address <ip>:7233`.

### `go test ./apps/worker/...` reports a race

Pipeline mode is tier-first: each configured tier runs Phase 2 through Phase 5.5 before the next tier starts. If tests count activity callbacks, keep counters concurrency-safe because the Temporal testsuite still invokes activity callbacks from worker goroutines. See `apps/worker/internal/workflow/crawl/workflow_test.go` for the pattern.

### `npm run codegen` emits TS2300 duplicate types

The graphql-codegen `typescript` + `typescript-operations` 6.x duo re-emits schema input types. Use `typescript-operations` alone for `types.ts` (already the default in `codegen.ts`).

### Lefthook pre-commit hook fails with "go: command not found"

The lefthook subprocess doesn't inherit the parent shell's PATH. Prefix your commit:

```bash
export PATH=/usr/local/go/bin:$HOME/go/bin:$PATH
git commit -m "…"
```

### Prettier / ESLint flags `apps/web/src/shared/api/generated/*`

These files are emitted by graphql-codegen and intentionally ignored:

- `apps/web/.prettierignore` excludes `src/**/generated/**`
- `apps/web/eslint.config.js` ignores the same path
- `lefthook.yml`'s `no-trailing-whitespace` hook skips `generated/` and `gen/`

If a fresh hook fails on generated output, check that all three ignore files are still in place after your branch.

### Playwright reports "libnspr4.so: cannot open shared object file"

Run the apt-install line in step 6.6. Without sudo, use the official Playwright Docker image to run the suite.

### `go.work.sum` drifts on every `go build`

Newer Go toolchains rewrite `go.work.sum` to include extra checksums. The drift is harmless — `git checkout -- go.work.sum` discards it cleanly between branch switches. If you see real new dependencies in the diff, commit them.

### `/readyz` returns 503

One of the dependency probes failed:

- `db` — postgres not reachable on `:55433`
- `redis` — redis not reachable on `:6379`

Inspect `make dev` containers and the API logs for the underlying error.

### Rankings table is empty

The crawler hasn't ingested data yet. Either trigger a `CrawlRegionWorkflow` (step 4) with a valid Riot API key, or seed test data via `psql` directly.
