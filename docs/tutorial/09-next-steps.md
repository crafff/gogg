# Chapter 09 · Next steps

> Goal: you know how to start a feature for Phase E or F, where the architectural decisions are documented, and how the contribution workflow works.

If you've read straight through, you now have:

- A mental model of the three binaries + their data flow ([Chapter 01](./01-overview.md))
- A running local stack ([Chapter 02](./02-setup.md))
- Familiarity with the 11 tables + the sqlc pipeline ([Chapter 03](./03-database.md))
- A grasp of the 8-phase crawler + Temporal's role ([Chapter 04](./04-crawler-temporal.md))
- A walk-through of chi → service → sqlc on the API side ([Chapter 05](./05-api-backend.md))
- The vite + React Router + codegen + TanStack Query pipeline ([Chapter 06](./06-frontend.md))
- A full end-to-end trace ([Chapter 07](./07-end-to-end.md))
- Auth + secrets ([Chapter 08](./08-auth-secrets.md))

This chapter is shorter — it's a map to "where next."

## Read these to go deeper

### The plan

```bash
cat /home/zrt/.claude/plans/radiant-wobbling-pizza.md
```

This is **the master document**. It captures the original refactoring proposal: what's being built, why, in what order. Phases A–F are described in detail. Re-read it now that you've seen the code — it'll make a lot more sense than on first read.

### The ADRs

Three short architectural decision records:

```bash
ls docs/architecture/adr/
```

- **0001 — modular monolith over microservices**: why we ship as three binaries on one Postgres rather than 8 microservices. The reasoning is "Stripe / Shopify / Basecamp's evolution path" — go modular first, split only when a service genuinely needs independent scale.
- **0002 — sqlc over ent**: why generated SQL beats an ORM in this domain. Aggregate queries (the rankings CTE chain) would be painful via ORM; SQL-first keeps the query plan visible.
- **0003 — GraphQL + REST dual surface**: why we run two transports on the same service layer. REST stays for the legacy frontend + scripts; GraphQL for the new frontend. Both go through the same error sanitization layer to prevent SQL leakage.

Read them in order. Each is ~3 pages.

### CLAUDE.md

```bash
cat CLAUDE.md
```

The repo's load-bearing context document. It lists:

- What each phase delivered + its key technical decisions
- The architectural rules that get enforced in review
- The commands
- The deprecation policy for the legacy stack

Re-read it now and again every few weeks. The phase summaries especially are gold for understanding "why is this here?" when you're deep in some file.

### The runbooks

```bash
ls docs/runbooks/
```

Phase F populates these:

- `riot-api-outage.md` — how to detect + mitigate Riot's API being down
- `db-failover.md` — recovering from a Postgres incident
- `crawler-stuck.md` — what to do if a workflow hangs
- `deploy.md` — blue/green cutover procedure

They're skeletons today. As Phase F lands, each gets filled with real procedures + dashboard links.

---

## What's actually next: Phase E

Phase E builds the three V1 features. The plan §3 covers them:

| Feature | What lands | Where |
|---|---|---|
| **Champion detail page** | Materialized view `mv_champion_detail` (core items, runes, matchups). Phase 6 activity in the crawl workflow to refresh it. New GraphQL `champion(id: Int!)` query. Rewrite `/champion/:id` from placeholder to real page. | Backend + crawler + frontend |
| **Summoner search** | On-demand `EnrichSummonerWorkflow`: API checks cache, if miss kicks off a Temporal workflow + SSE progress stream. `summoner_search_cache` table. `/summoner/:region/:name` rewritten. | Backend + worker + frontend |
| **User accounts** | Discord + Google OAuth wired end-to-end. `/login` real page (provider buttons). `/me` real page (favorites + bound summoner). RSO behind build tag until Riot approves. | Backend + frontend |

You're now on `refactor/phase-e-features`. The first chunk to land is up to you; the plan suggests starting with champion detail (cleanest dependency graph: crawler → mat view → API → frontend).

### How to start chunk 1

Use the same chunk-by-chunk discipline that Phases B–D used:

1. Plan a single small chunk in your head (1–2 days of work, one PR).
2. Write the migration (if any): `make migrate-new name=add_champion_detail_view`.
3. Write the sqlc query in `packages/sqlc/queries/champions.sql`. Run `make gen-sqlc`.
4. Write the service in `apps/api/internal/service/champion/`.
5. Wire the REST handler (if needed) + GraphQL resolver.
6. Write tests at each layer.
7. Commit. Push. Open a PR with a "Test plan" checklist (see PR #4–#7 for the template).
8. CI must be green before merge.

The CLAUDE.md "Architectural rules" section spells out the must-follow conventions: service layer owns business logic, sqlc for data access, secrets via SOPS, migrations forward-only, legacy stack sacred until replacement ships.

---

## Phase F preview

Once Phase E ships, Phase F is the production-hardening pass:

- `deploy/k8s/base/` — Kustomize base for the three binaries + Service + Ingress + HPA + PDB + NetworkPolicy.
- `deploy/terraform/modules/aws/` — EKS + RDS + ElastiCache + S3 + CloudFront + ACM.
- `deploy/terraform/modules/aliyun/` — ACK + RDS + Redis + OSS + CDN (domestic China deploy parity).
- `deploy/observability/` — Prometheus rules + Grafana dashboards + Alertmanager routes.
- `docs/runbooks/` — fill the four runbooks with real procedures.
- k6 load tests for SLO validation: rankings 1000rps, summoner 100rps.
- Blue/green deploy procedure.

That's the last major engineering work before "V1 generally available."

---

## Contribute / extend

### Workflow

1. Make sure you're on `refactor/phase-e-features` (or branch off it).
2. Make changes; commit with [Conventional Commits](https://www.conventionalcommits.org/) format (`feat(api): ...`, `fix(web): ...`, `chore(secrets): ...`, etc.). Lefthook will run gofmt, golangci-lint, prettier, eslint, gitleaks, no-trailing-whitespace.
3. Push, open PR with the template (`gh pr create` will pre-fill).
4. CI runs: vet, lint, test (Go + web), build, gitleaks, govulncheck, semgrep, npm audit, migrations parity, docker build.
5. After merge, the branch can be deleted via the PR page or `git push origin --delete <branch>`.

### Tooling reminders

- `make ci` locally mirrors what CI runs server-side. Use it as the pre-push gate.
- `make hooks` installs the lefthook pre-commit gates.
- `make manual-verification.md` (well, just open the file) is the smoke-test checklist after a non-trivial change.

### Asking questions

- Architectural / "why is this here?" → check ADRs first, then the relevant chapter of this tutorial.
- "How does X work in practice?" → grep for the symbol; the codebase has heavy comments on the "why" but minimal "what" explanation in code (the names are supposed to explain the what).
- Stuck in a phase decision → re-read the plan section + CLAUDE.md phase summary. The plan documents alternatives that were considered + rejected.

### When to deviate from the plan

The plan was written before the code existed. Some decisions might not survive contact with reality. When you find one:

1. Document the deviation in a new ADR (`docs/architecture/adr/0004-...`).
2. Update CLAUDE.md if it changes a load-bearing assumption.
3. PR with both code + doc changes in one commit.

The ADRs are how future-you remembers why a tradeoff went the way it did. Be generous with them.

---

## You're done

That's the whole tutorial. Re-read any chapter as you start work in that area. Skip back to [Chapter 07](./07-end-to-end.md) whenever you want to refresh the end-to-end mental model.

If anything in this tutorial is wrong, out of date, or unclear: open a PR fixing it. Tutorial files are the easiest first contribution — no risk of breaking anything, big effect on every future reader.

Good luck with Phase E.
