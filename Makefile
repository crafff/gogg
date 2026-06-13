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

# ── Compose ─────────────────────────────────────────────────
COMPOSE_FILE ?= deploy/compose/docker-compose.dev.yml

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make <target>\n\nTargets:\n"} \
		/^[a-zA-Z0-9_.-]+:.*##/ {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ── Local dev ───────────────────────────────────────────────
.PHONY: dev
dev: ## Bring up the local dev stack (postgres + redis + temporal)
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

# ── Quality gates ───────────────────────────────────────────
# `npm run <script> --if-present` exits 0 when the script is
# missing from package.json. The legacy web/ package.json doesn't
# define lint/test/format scripts yet (Phase D adds them when
# the new web app lands); --if-present lets the same Makefile
# work today and a year from now.

.PHONY: lint
lint: ## Run all linters (go + web)
	golangci-lint run ./...
	@if [ -f web/package.json ]; then cd web && npm run lint --if-present; fi
	@if [ -f apps/web/package.json ]; then cd apps/web && npm run lint --if-present; fi

.PHONY: fmt
fmt: ## Format all code
	gofmt -w -s .
	@if [ -f web/package.json ]; then cd web && npm run format --if-present; fi
	@if [ -f apps/web/package.json ]; then cd apps/web && npm run format --if-present; fi

.PHONY: test
test: ## Run all tests
	go test ./...
	@if [ -f web/package.json ]; then cd web && npm run test --if-present --silent; fi
	@if [ -f apps/web/package.json ]; then cd apps/web && npm run test --if-present --silent; fi

.PHONY: test-int
test-int: ## Run integration tests (requires `make dev` running)
	GOGG_INTTEST=1 go test ./... -tags=integration -count=1

.PHONY: vet
vet:
	go vet ./...

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
build: build-api build-worker ## Build all binaries

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

# ── Hooks ───────────────────────────────────────────────────
.PHONY: hooks
hooks: ## Install pre-commit hooks via lefthook
	lefthook install

# ── Cleanup ─────────────────────────────────────────────────
.PHONY: clean
clean:
	rm -rf bin/ coverage.out coverage.html
