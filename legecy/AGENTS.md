# GOGG agent guide

This file is the repository-level source of truth for coding agents. Keep it
short, operational, and aligned with the code. Read `docs/ai/project-memory.md`
at the start of non-trivial work and update it only when a durable project fact
or lesson changes.

## Mission and scope

GOGG is a production-oriented League of Legends statistics and summoner-search
modular monolith. The active product targets KR and NA1, serves a bilingual
zh-CN/en-US UI, and consists of:

- `apps/api`: Go, chi, gqlgen GraphQL BFF, and REST compatibility endpoints.
- `apps/worker`: Go Temporal workers for crawl and enrichment workflows.
- `apps/web`: React, TypeScript, Vite, TanStack Query, and Tailwind.
- `packages/sqlc`: migrations, SQL queries, and generated Go bindings.
- `packages/riotapi`: Riot API client and routing behavior.

Do not recreate dependencies on the archived top-level `internal/`,
`cmd/crawl`, root `main.go`, or the old `web/` application.

## Source-of-truth order

1. The user's current request and acceptance criteria.
2. This `AGENTS.md` and more specific nested agent guidance, if present.
3. `docs/ai/project-memory.md` for durable project context.
4. ADRs under `docs/architecture/adr/` and `docs/contributing.md`.
5. The current implementation and tests.

When documentation and code disagree, establish the actual behavior from code
and tests, then update stale documentation as part of the same change when it
is in scope.

## Autonomous working agreement

- Proceed without asking for confirmation for reversible, in-scope inspection,
  edits, generation, local builds, tests, and local development services.
- Make a reasonable, explicitly stated assumption when it keeps work aligned
  with the request. Ask only when a missing choice would materially change the
  product or requires new authority.
- Never infer authority for destructive data operations, production changes,
  publishing, purchases, credential access, or external messages.
- Inspect `git status --short` before editing. The worktree may contain user
  changes; preserve them and avoid unrelated cleanup.
- Do not commit, push, rewrite history, reset files, or delete material data
  unless the user explicitly requests it.
- Never expose or commit secrets. Use SOPS-encrypted files or ignored local
  configuration.

## Engineering boundaries

- Business logic belongs in service packages. GraphQL resolvers and REST
  handlers translate transport concerns and call services; they do not contain
  SQL or workflow policy.
- Data access normally goes through sqlc. A truly dynamic hand-written pgx
  query requires `// sqlc-skip: <reason>`.
- Temporal workflows must remain deterministic. Network, database, clock, and
  other side effects belong in activities. Activities must tolerate retries;
  long-running user work must have stable IDs and reconnectable status.
- Treat Riot 404 and other terminal domain outcomes explicitly. Do not retry a
  terminal not-found response as a transient infrastructure error.
- Schema changes ship matching up/down migrations, updated queries, generated
  bindings, and tests in one cohesive change.
- GraphQL schema changes require gqlgen output and frontend generated types to
  remain synchronized.
- Frontend server state uses TanStack Query. Preserve route-addressable state,
  zh-CN/en-US strings, accessibility, and reload/reconnect behavior.
- CommunityDragon assets are cached locally and served through
  `/game-assets`; do not add runtime dependencies on third-party image hosts.

## Work loop

1. Restate the observable outcome and inspect the actual execution path.
2. For a complex change, keep a short plan with one active step. Do not create
   planning artifacts for a simple edit.
3. Implement the smallest coherent change across all affected layers.
4. Add or update tests for behavior, including terminal and retry paths.
5. Run the narrow checks first, then broader checks in proportion to risk.
6. Review the final diff for regressions, generated-file drift, secrets, and
   unrelated changes.
7. Update project memory only for durable facts, decisions, or repeated lessons.

## Verification matrix

- Go package change: `go test` for the affected package tree.
- API or worker boundary change: `make check-no-legacy` plus affected tests.
- Shared Go package change: test direct consumers when practical.
- GraphQL change: `make gen-gql`, `make gen-web`, then affected Go and web tests.
- SQL change: `make gen-sqlc`, migration validation, and affected integration
  tests when the dev stack is available.
- Web change: from `apps/web`, run affected Vitest tests, `npm run type-check`,
  and `npm run lint`; use Playwright for user-critical flows when available.
- Broad or release-sensitive change: `make ci` after focused checks pass.
- Always finish with `git diff --check` and a scoped `git diff` review.

If a broad check fails because of an unrelated pre-existing change, report the
exact failure and still run the strongest relevant focused checks.

## Multi-agent policy

The primary agent owns requirements, architectural decisions, edits that cross
components, and the final verification result. Prefer multi-agent collaboration
whenever it is likely to materially improve response quality, correctness,
coverage, or verification confidence. Default to one agent only for small or
tightly coupled work where subagents would not materially improve the result.

Use subagents when there are concrete, bounded lanes that benefit from an
independent perspective or can proceed in parallel. Quality gain alone is a
sufficient reason; reduced latency is an additional benefit, not a requirement.
Common triggers include independent execution-path exploration, focused failure
reproduction, security or regression review, and cross-checking high-risk
changes. Prefer these roles:

- `gogg_explorer`: read-only execution-path and dependency mapping.
- `gogg_verifier`: focused tests, reproduction, and failure triage.
- `gogg_reviewer`: read-only correctness, security, and regression review.

Keep at most three subagents active. Give each a bounded question and expected
summary. Prefer parallel read-heavy work; have at most one writer for any file
set. The primary agent must wait for relevant results, validate their evidence,
resolve disagreements, and integrate the final change. Do not delegate trivial
edits or use subagents merely to add ceremony.

## Memory protocol

- Native Codex Memories hold cross-chat preferences and useful personal
  context. They are not authoritative project documentation.
- This file holds mandatory behavior that must load every session.
- `docs/ai/project-memory.md` holds stable product state, decisions, and proven
  failure modes. Do not store raw logs, temporary task status, speculation, or
  secrets there.
- ADRs hold consequential architectural decisions and their alternatives.
- Keep task-local details in the current thread or plan. At handoff, summarize
  outcome, verification, remaining risk, and any durable memory update.

## Primary commands

```bash
make dev
make migrate-up
make run-api
make run-worker
make run-web
make test
make ci
make gen
```

`make dev-reset` destroys local volumes. It is never an automatic cleanup step.
