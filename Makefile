SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

# ── Tool versions (pinned via tools/tools.go later) ─────────
GOLANGCI_LINT_VERSION ?= v1.62.0
SQLC_VERSION          ?= v1.27.0
MIGRATE_VERSION       ?= v4.18.1
GQLGEN_VERSION        ?= v0.17.55
LEFTHOOK_VERSION      ?= v1.10.0

# ── Local dev DSNs (override via env) ───────────────────────
DEV_PG_DSN  ?= postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable
DEV_REDIS   ?= redis://localhost:6379/0
DEV_TEMPORAL ?= localhost:7233
ifdef GOGG_DB_ROOT
GOGG_DATA_ROOT ?= $(GOGG_DB_ROOT)
else
GOGG_DATA_ROOT ?= /mnt/gogg-db
endif
GOGG_DB_ROOT ?= $(GOGG_DATA_ROOT)
GOGG_RAW_ARCHIVE_ROOT ?= $(GOGG_DATA_ROOT)/riot-raw
GOGG_ASSETS_ROOT ?= $(GOGG_DATA_ROOT)/game-assets
GOGG_TFT_STATIC_ROOT ?= $(GOGG_ASSETS_ROOT)
PERF_SCENARIO ?= rankings_graphql
PERF_VUS      ?= 20
PERF_DURATION ?= 1m
PERF_DIR      ?= $(GOGG_DATA_ROOT)/performance
PERF_VARIANT  ?= baseline
PERF_DB_CONTAINER ?= gogg-perf-postgres
GO_PACKAGES = $(shell go list ./... | grep -v '/node_modules/')
GO_LINT_DIRS = $(shell go list -f '{{.Dir}}' ./... | grep -v '/node_modules/' | sed 's#^$(CURDIR)/#./#')

# ── Compose ─────────────────────────────────────────────────
COMPOSE_FILE ?= deploy/compose/docker-compose.dev.yml
export GOGG_DATA_ROOT GOGG_DB_ROOT GOGG_RAW_ARCHIVE_ROOT GOGG_ASSETS_ROOT GOGG_TFT_STATIC_ROOT PERF_DIR

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make <target>\n\nTargets:\n"} \
		/^[a-zA-Z0-9_.-]+:.*##/ {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ── Local dev ───────────────────────────────────────────────
.PHONY: dev-storage-check
dev-storage-check: ## Verify all durable local data targets use the F-backed ext4 VHDX
	@GOGG_DATA_ROOT="$(GOGG_DATA_ROOT)" bash scripts/gogg-db-preflight.sh

.PHONY: dev
dev: dev-storage-check ## Bring up the local dev stack (postgres + redis + temporal)
	docker compose -f $(COMPOSE_FILE) up -d
	@echo "Postgres: $(DEV_PG_DSN)"
	@echo "Redis:    $(DEV_REDIS)"
	@echo "Temporal: $(DEV_TEMPORAL) (UI on http://localhost:8233)"

.PHONY: dev-down
dev-down: ## Tear down the local dev stack
	docker compose -f $(COMPOSE_FILE) down

.PHONY: dev-reset
dev-reset: ## Tear down AND drop all volumes (data loss!)
	docker compose -f $(COMPOSE_FILE) down -v

.PHONY: observability
observability: dev-storage-check ## Start Prometheus, Grafana, and DB/cache exporters
	docker compose -f $(COMPOSE_FILE) --profile observability up -d --build postgres redis api-observed postgres-exporter redis-exporter prometheus grafana
	docker compose -f $(COMPOSE_FILE) exec -T postgres psql -U gogg -d gogg -c 'CREATE EXTENSION IF NOT EXISTS pg_stat_statements;'
	@echo "Grafana:    http://localhost:3001 (admin/admin)"
	@echo "Prometheus: http://localhost:9090"

.PHONY: observability-down
observability-down: ## Stop only local observability services
	docker compose -f $(COMPOSE_FILE) --profile observability stop grafana prometheus redis-exporter postgres-exporter api-observed

.PHONY: db-slow-queries
db-slow-queries: ## Show top PostgreSQL statements by total execution time
	docker compose -f $(COMPOSE_FILE) exec -T postgres psql -U gogg -d gogg -f /dev/stdin < deploy/observability/postgres/slow-queries.sql

.PHONY: perf-warm
perf-warm: dev-storage-check ## Run a repeatable warm-cache k6 API baseline
	PERF_DIR=$(PERF_DIR) PERF_SCENARIO=$(PERF_SCENARIO) PERF_VUS=$(PERF_VUS) \
		PERF_DURATION=$(PERF_DURATION) PERF_VARIANT=$(PERF_VARIANT) \
		PERF_DB_CONTAINER=$(PERF_DB_CONTAINER) \
		PERF_EXPERIMENT=$(PERF_EXPERIMENT) COMPOSE_FILE=$(COMPOSE_FILE) \
		bash tests/performance/run-api-performance.sh warm

.PHONY: perf-cold
perf-cold: dev-storage-check ## Flush local Compose Redis, then run the k6 baseline
	PERF_DIR=$(PERF_DIR) PERF_SCENARIO=$(PERF_SCENARIO) PERF_VUS=$(PERF_VUS) \
		PERF_DURATION=$(PERF_DURATION) PERF_VARIANT=$(PERF_VARIANT) \
		PERF_DB_CONTAINER=$(PERF_DB_CONTAINER) \
		PERF_EXPERIMENT=$(PERF_EXPERIMENT) COMPOSE_FILE=$(COMPOSE_FILE) \
		bash tests/performance/run-api-performance.sh cold

.PHONY: perf-env-up
perf-env-up: dev-storage-check ## Start the fixed mobile-drive database and observed API
	tests/performance/perf-drive.sh up
	PERF_API_DATABASE_DSN='postgres://gogg:goggpass@host.docker.internal:55434/gogg?sslmode=disable' \
	PERF_EXPORTER_DATABASE_DSN='postgresql://gogg:goggpass@host.docker.internal:55434/gogg?sslmode=disable' \
		docker compose -f $(COMPOSE_FILE) --profile observability up -d --build \
			postgres redis api-observed postgres-exporter redis-exporter prometheus grafana
	@echo "Observed API: http://localhost:18080 (database: mobile drive on :55434)"

# ── Quality gates ───────────────────────────────────────────
.PHONY: lint
lint: ## Run all linters (go + web)
	golangci-lint run $(GO_LINT_DIRS)
	@if [ -f apps/web/package.json ]; then cd apps/web && npm run lint --if-present; fi

.PHONY: fmt
fmt: ## Format all code
	gofmt -w -s .
	@if [ -f apps/web/package.json ]; then cd apps/web && npm run format --if-present; fi

.PHONY: test
test: ## Run all tests
	go test $(GO_PACKAGES)
	@if [ -f apps/web/package.json ]; then cd apps/web && npm run test --if-present --silent; fi

.PHONY: check-no-legacy
check-no-legacy: ## Ensure new apps/packages do not import legacy code
	@if rg -n 'github.com/crafff/gogg/internal/(server|crawler|storage|riotapi|config)|github.com/crafff/gogg/cmd/crawl' apps packages; then \
		echo "legacy imports found in apps/packages"; \
		exit 1; \
	fi

.PHONY: test-int
test-int: ## Run integration tests (requires `make dev` running)
	GOGG_INTTEST=1 go test $(GO_PACKAGES) -tags=integration -count=1

.PHONY: test-e2e
test-e2e: ## Run Playwright e2e tests against apps/web (requires browser deps)
	@cd apps/web && npm run test:e2e

.PHONY: test-e2e-install
test-e2e-install: ## Install Playwright browser binaries (one-time)
	@cd apps/web && npm run test:e2e:install

.PHONY: vet
vet:
	go vet $(GO_PACKAGES)

.PHONY: ci
ci: vet lint test ## Same gates that CI runs

# ── Code generation ─────────────────────────────────────────
.PHONY: gen
gen: gen-sqlc gen-gql gen-web ## Run all codegen

.PHONY: gen-sqlc
gen-sqlc: ## Regenerate sqlc bindings
	@if [ -f packages/sqlc/sqlc.yaml ]; then \
		cd packages/sqlc && sqlc generate; \
	else echo "packages/sqlc/sqlc.yaml not present yet"; fi

.PHONY: gen-gql
gen-gql: ## Regenerate gqlgen resolvers
	@if [ -f apps/api/gqlgen.yml ]; then \
		cd apps/api && go run github.com/99designs/gqlgen generate; \
	else echo "apps/api/gqlgen.yml not present yet"; fi

.PHONY: gen-web
gen-web: ## Regenerate GraphQL client types
	@if [ -d apps/web ] && [ -f apps/web/codegen.ts ]; then \
		cd apps/web && npm run codegen; \
	else echo "apps/web/codegen.ts not present yet"; fi

# ── Migrations ──────────────────────────────────────────────
.PHONY: migrate-up
migrate-up: ## Apply all pending migrations to local dev DB
	migrate -path packages/sqlc/migrations -database "$(DEV_PG_DSN)" up

.PHONY: migrate-down
migrate-down: ## Roll back the last migration
	migrate -path packages/sqlc/migrations -database "$(DEV_PG_DSN)" down 1

.PHONY: migrate-new
migrate-new: ## Create a new migration; usage: make migrate-new name=add_users
	@if [ -z "$(name)" ]; then echo "usage: make migrate-new name=<snake_case>"; exit 1; fi
	migrate create -ext sql -dir packages/sqlc/migrations -seq $(name)

# ── Build ───────────────────────────────────────────────────
.PHONY: build
build: build-api build-worker build-tft-worker build-crawlctl build-tft-quota-probe build-crawler-lite ## Build all binaries

.PHONY: build-api
build-api:
	@mkdir -p bin
	@if [ -d apps/api/cmd/api ] && [ -f apps/api/cmd/api/main.go ]; then \
		go build -trimpath -o bin/gogg-api ./apps/api/cmd/api; \
	else echo "apps/api/cmd/api/main.go not present yet"; fi

.PHONY: build-worker
build-worker:
	@mkdir -p bin
	@if [ -d apps/worker/cmd/worker ] && [ -f apps/worker/cmd/worker/main.go ]; then \
		go build -trimpath -o bin/gogg-worker ./apps/worker/cmd/worker; \
	else echo "apps/worker/cmd/worker/main.go not present yet"; fi

.PHONY: build-crawler-lite
build-crawler-lite:
	@mkdir -p bin
	@if [ -d apps/worker/cmd/crawler-lite ] && [ -f apps/worker/cmd/crawler-lite/main.go ]; then \
		go build -trimpath -o bin/gogg-crawler-lite ./apps/worker/cmd/crawler-lite; \
	else echo "apps/worker/cmd/crawler-lite/main.go not present yet"; fi

.PHONY: build-tft-worker
build-tft-worker:
	@mkdir -p bin
	go build -trimpath -o bin/gogg-tft-worker ./apps/worker/cmd/tft-worker

.PHONY: build-crawlctl
build-crawlctl:
	@mkdir -p bin
	go build -trimpath -o bin/gogg-crawlctl ./apps/worker/cmd/crawlctl

.PHONY: build-tft-quota-probe
build-tft-quota-probe:
	@mkdir -p bin
	go build -trimpath -o bin/gogg-tft-quota-probe ./apps/worker/cmd/tft-quota-probe

.PHONY: run-api
run-api: dev-storage-check ## Run gogg-api locally with SOPS or config/dev.yaml
	@if [ -f deploy/secrets/dev.enc.yaml ] && command -v sops >/dev/null 2>&1; then \
		tmp=$$(mktemp -t gogg-api.XXXXXX.yaml); \
		trap "rm -f $$tmp" EXIT; \
		sops --decrypt deploy/secrets/dev.enc.yaml > $$tmp; \
		APP_CONFIG_PATH=$$tmp go run ./apps/api/cmd/api; \
	elif [ -f config/dev.yaml ]; then \
		APP_CONFIG_PATH=config/dev.yaml go run ./apps/api/cmd/api; \
	else \
		echo "missing config/dev.yaml; copy config/dev.example.yaml and fill local secrets"; \
		exit 1; \
	fi

.PHONY: run-web
run-web: ## Run the apps/web vite dev server (proxies /api + /graphql to :8080)
	@cd apps/web && npm run dev

.PHONY: build-web
build-web: ## Type-check + production build of apps/web → apps/web/dist
	@cd apps/web && npm run build

.PHONY: run-worker
run-worker: dev-storage-check ## Run gogg-worker locally with SOPS or config/dev.yaml
	@if [ -f deploy/secrets/dev.enc.yaml ] && command -v sops >/dev/null 2>&1; then \
		tmp=$$(mktemp -t gogg-worker.XXXXXX.yaml); \
		trap "rm -f $$tmp" EXIT; \
		sops --decrypt deploy/secrets/dev.enc.yaml > $$tmp; \
		APP_CONFIG_PATH=$$tmp go run ./apps/worker/cmd/worker; \
	elif [ -f config/dev.yaml ]; then \
		APP_CONFIG_PATH=config/dev.yaml go run ./apps/worker/cmd/worker; \
	else \
		echo "missing config/dev.yaml; copy config/dev.example.yaml and fill riot.api_key"; \
		exit 1; \
	fi

.PHONY: run-tft-worker
run-tft-worker: dev-storage-check ## Run the isolated TFT worker; schedules are created paused
	@if [ -f deploy/secrets/dev.enc.yaml ] && command -v sops >/dev/null 2>&1; then \
		tmp=$$(mktemp -t gogg-tft-worker.XXXXXX.yaml); \
		trap "rm -f $$tmp" EXIT; \
		sops --decrypt deploy/secrets/dev.enc.yaml > $$tmp; \
		APP_CONFIG_PATH=$$tmp go run ./apps/worker/cmd/tft-worker; \
	elif [ -f config/dev.yaml ]; then \
		APP_CONFIG_PATH=config/dev.yaml go run ./apps/worker/cmd/tft-worker; \
	else \
		echo "missing config/dev.yaml; copy config/dev.example.yaml and fill riot.api_key"; \
		exit 1; \
	fi

.PHONY: sync-assets
sync-assets: dev-storage-check ## Sync latest CommunityDragon assets; optionally pass args='--version 16.14'
	@go run ./apps/worker/cmd/asset-sync $(args) --root "$(GOGG_ASSETS_ROOT)"

.PHONY: refresh-champion-detail
refresh-champion-detail: ## Rebuild champion detail rollups; optionally pass args='--timeout 1h'
	@go run ./apps/worker/cmd/champion-detail-rollup --database-dsn "$(DEV_PG_DSN)" $(args)

.PHONY: refresh-rankings
refresh-rankings: ## Rebuild rankings rollups; optionally pass args='--timeout 1h'
	@go run ./apps/worker/cmd/rankings-rollup --database-dsn "$(DEV_PG_DSN)" $(args)

.PHONY: backfill-perks
backfill-perks: ## Repair historical six-rune data; pass args='--dry-run' or '--region KR'
	@if [ -f deploy/secrets/dev.enc.yaml ] && command -v sops >/dev/null 2>&1; then \
		tmp=$$(mktemp -t gogg-perk-backfill.XXXXXX.yaml); \
		trap "rm -f $$tmp" EXIT; \
		sops --decrypt deploy/secrets/dev.enc.yaml > $$tmp; \
		APP_CONFIG_PATH=$$tmp go run ./apps/worker/cmd/perk-backfill $(args); \
	elif [ -f config/dev.yaml ]; then \
		APP_CONFIG_PATH=config/dev.yaml go run ./apps/worker/cmd/perk-backfill $(args); \
	else \
		echo "missing config/dev.yaml or decryptable deploy/secrets/dev.enc.yaml"; \
		exit 1; \
	fi

.PHONY: run-crawler-lite
run-crawler-lite: dev-storage-check ## Run crawler-lite; pass args='run --profile daily_kr'
	@if [ -f deploy/secrets/dev.enc.yaml ] && command -v sops >/dev/null 2>&1; then \
		tmp=$$(mktemp -t gogg-crawler-lite.XXXXXX.yaml); \
		trap "rm -f $$tmp" EXIT; \
		sops --decrypt deploy/secrets/dev.enc.yaml > $$tmp; \
		APP_CONFIG_PATH=$$tmp go run ./apps/worker/cmd/crawler-lite $(args); \
	elif [ -f config/dev.yaml ]; then \
		APP_CONFIG_PATH=config/dev.yaml go run ./apps/worker/cmd/crawler-lite $(args); \
	else \
		echo "missing config/dev.yaml; copy config/dev.example.yaml and fill riot.api_key"; \
		exit 1; \
	fi

# ── Hooks ───────────────────────────────────────────────────
.PHONY: hooks
hooks: ## Install pre-commit hooks via lefthook
	lefthook install

# ── Cleanup ─────────────────────────────────────────────────
.PHONY: clean
clean:
	rm -rf bin/ coverage.out coverage.html
