# Contributing to GOGG

This document is the source of truth for how we work. If you find
yourself disagreeing with anything here mid-task, update the doc
in the same PR.

## TL;DR

- Branch off `main` as `feat/<slug>`, `fix/<slug>`,
  `refactor/<slug>`, or `chore/<slug>`.
- Commit messages follow [Conventional
  Commits](https://www.conventionalcommits.org/) — enforced by
  `commitlint` via `lefthook`.
- Run `make ci` before pushing; CI runs the same gates.
- Open a PR against `main`; squash-merge when green.

## Branch strategy

We use **trunk-based development** with short-lived feature
branches. There are two long-running exceptions during the
refactor:

| Branch | Lifetime | Purpose |
|---|---|---|
| `main` | permanent | Always green; what's deployed |
| `refactor/phase-a-foundation` | Phase A only | Scaffolding for the refactor |
| `refactor/phase-b-backend` | Phase B only | Backend rewrite |
| `refactor/phase-c-temporal` | Phase C only | Crawler migration |
| `refactor/phase-d-web` | Phase D only | Frontend rewrite |

Refactor branches accept PRs from short-lived sub-branches and
merge into `main` at phase boundaries.

Branch naming for feature work:

```
feat/champion-detail-page
fix/rankings-position-threshold-off-by-one
refactor/extract-rankings-cte-into-view
chore/bump-pgx-to-v5.6.0
docs/adr-0004-redis-key-conventions
```

## Commit messages

Conventional Commits, lowercase type, optional scope:

```
<type>(<scope>): <subject>

<body — explains the why, references PR / ADR / issue as needed>
```

Types we use: `feat`, `fix`, `refactor`, `chore`, `docs`, `ci`,
`test`, `perf`, `style`, `build`.

Examples that pass commitlint:

- `feat(rankings): add positionThreshold filter`
- `fix(crawler): respect Retry-After on 429`
- `refactor: extract RankingService from rankings handler`
- `docs(adr): record decision on sqlc over ent`
- `ci: cache go modules across jobs`

## Pre-commit hooks (lefthook)

```bash
make hooks   # installs lefthook
```

The hooks run on staged files only:

- `go fmt` + `go vet` + `golangci-lint --new-from-rev=HEAD`
- `prettier` + `eslint` on JS/TS
- `gitleaks` (block any committed secret)
- `commitlint` (block non-conventional commits)

To bypass in an emergency: `git commit --no-verify`. This shows
up in code review and must be justified.

## Code style

### Go

- gofmt + golangci-lint. `errcheck`, `revive`, `gocritic`,
  `gosec`, `nilerr` are all on; see `.golangci.yml` for the full
  set when it lands.
- Errors are typed (`packages/domain/errors.go`) and wrapped
  with `fmt.Errorf("... %w", err)` at boundaries. Never return
  raw DB errors to HTTP.
- Logging via `log/slog`. Use the request-scoped logger from
  middleware; never `slog.Default()` inside a handler.
- Tests use `testify/require` for assertions and
  `testcontainers-go` for anything that needs Postgres / Redis.
  Mocks live next to the interface (`mock_<name>.go`), generated
  via `mockery` or hand-written when small.

### TypeScript / React

- Strict mode (`tsconfig.strict: true`); no `any` without a
  `// TODO(x): reason` comment.
- One component per file, named the same as the file.
- Co-locate hooks, styles, and tests with the component:
  `Rankings/Rankings.tsx`, `Rankings/Rankings.test.tsx`,
  `Rankings/useRankings.ts`.
- Data layer: TanStack Query for server state, Zustand for
  client global state. Don't reach for Redux.
- Forms: react-hook-form + zod.

### SQL

- All migrations in `packages/sqlc/migrations/`.
- Every `*.up.sql` ships a matching `*.down.sql`, even if the
  down is a no-op (`SELECT 1;`).
- Schema changes that break existing queries must update both
  `queries/*.sql` and regenerate `gen/` in the same commit.

## Pull request checklist

Reviewers will check:

- [ ] Conventional commit subject; body explains the **why**
- [ ] No secrets, no `.env` / `config.yaml` content
- [ ] `make ci` is green locally
- [ ] New behaviour has tests (unit and/or integration)
- [ ] Public API change is reflected in GraphQL SDL + OpenAPI
- [ ] Touched ADR territory? Record it as a new ADR rather than
      relitigating an old one
- [ ] No drive-by refactors mixed into a feature PR (split the
      PR; small ones merge faster)

## Testing strategy

- **Unit tests** — the service layer. Mock the repository
  interface, exercise the business logic.
- **Integration tests** — go through transport → service →
  testcontainers-go-backed Postgres. Use the `crawl_inttest`
  schema for crawler tests so we don't pollute the dev DB.
- **E2E tests** — Playwright against a `make dev` stack.
  Smoke-level only; not every interaction.
- **Load tests** — `scripts/loadtest/*.js` (k6). Run before
  every prod deploy in Phase F.

Coverage thresholds (enforced in CI from Phase B):

| Layer | Target |
|---|---|
| `apps/api/internal/service` | ≥ 80% lines |
| `apps/api/internal/transport` | ≥ 60% lines |
| `apps/worker/internal/activity` | ≥ 70% lines |
| `apps/web/src/features/*/hooks` | ≥ 70% lines |

## Local development workflow

```bash
# 1. Start the dev stack
make dev

# 2. Apply migrations
make migrate-up

# 3. Regenerate codegen
make gen

# 4. Run the API
go run ./apps/api/cmd/api

# 5. Run the worker (separate terminal)
go run ./apps/worker/cmd/worker

# 6. Run the web app (separate terminal)
cd apps/web && npm run dev
```

While Phase A is in progress, replace steps 4–6 with the
legacy equivalents (`go run .` for the old server,
`cd web && npm run dev` for the old frontend) — see
`CLAUDE.md`.

## Reporting bugs / proposing features

GitHub issues. Use the templates (incoming) and include:

- Affected component (api, worker, web, infra)
- Steps to reproduce or a precise feature description
- Why now (what's blocked, what changes if we ship this)
- Any relevant ADRs or runbooks
