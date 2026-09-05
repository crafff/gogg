# Tutorial: how to read GOGG

This is a hand-held walkthrough that takes you from "what is this repo?" to "I understand how every layer fits together, I can apply the patterns elsewhere, and I have a process for learning the next codebase." It assumes you can use a command line and know a bit of programming, but **doesn't assume** Go, React, GraphQL, Temporal, or PostgreSQL. Each chapter introduces the concepts as they come up.

## How to use this

Read each chapter in order, the first time through. Every chapter has:

- **Mental model** — the concepts you need before reading code
- **Read these files** — exact paths in tree order, smallest-to-biggest
- **Try this** — concrete commands you can run while reading
- **Exercises** — small modifications to deepen understanding
- **Why it's built this way** — links to relevant ADRs / plan sections

Time budget per chapter: 30–60 min if you're new to the stack, faster if you already know Go / React.

## Chapters

### Part I — Understanding GOGG (read in order)

| # | Topic | What you'll understand by the end |
|---|---|---|
| 01 | [Overview](./01-overview.md) | What GOGG does, which binaries exist, how data flows at 10 000 ft |
| 02 | [Setup](./02-setup.md) | Bring up the dev stack from a fresh clone; see the rankings page render |
| 03 | [Database + sqlc](./03-database.md) | How the schema is shaped, how migrations work, how `*.sql` becomes Go code |
| 04 | [Crawler + Temporal](./04-crawler-temporal.md) | What the 8-phase crawler does, why it runs as a Temporal workflow |
| 05 | [API backend](./05-api-backend.md) | How an HTTP request walks through chi → service → sqlc; REST vs GraphQL |
| 06 | [Frontend](./06-frontend.md) | How `apps/web` is structured; codegen → TanStack Query → page |
| 07 | [End-to-end trace](./07-end-to-end.md) | Follow a single `winRate` value from Riot's API into the browser |
| 08 | [Auth + secrets](./08-auth-secrets.md) | JWT + OAuth + how SOPS keeps secrets out of git |

### Part II — Transferable knowledge

These chapters take the patterns you saw in Part I and generalize them. Read after you've done at least 01 + 02 + one feature chapter; the examples make more sense with concrete code in your head.

| # | Topic | What you'll understand by the end |
|---|---|---|
| 10 | [Go essentials](./10-go-essentials.md) | Context, interfaces, errors, slog, DI without a framework, testing — the Go you need to be productive *anywhere* |
| 11 | [Frontend essentials](./11-frontend-essentials.md) | TypeScript inference, React rendering model, hooks rules, state management spectrum, accessibility, testing philosophy |
| 12 | [Reading codebases](./12-reading-codebases.md) | The meta-skill: how to onboard quickly to any unfamiliar 30k+ LOC project |
| 13 | [Annotated code tour](./13-annotated-tour.md) | Six classic snippets from this codebase walked line-by-line with the why |

### Part III — Going further

| # | Topic | What you'll get |
|---|---|---|
| 09 | [Next steps](./09-next-steps.md) | What Phase E + F look like, how to start contributing |

## Suggested reading paths

### Just want to understand the project

01 → 02 → 03 → 04 → 05 → 06 → 07. ~5 hours. Skip 08 (auth) until needed.

### New to Go AND new to React

10 first (Go essentials), then 01 → 02 → 03 → 04 → 05. Then 11 (Frontend essentials), then 06 → 07. Then 13 (annotated tour) for worked examples in both. ~10 hours total.

### Experienced developer, unfamiliar codebase

01 → 02 → 07 (end-to-end trace gives the whole picture fast). Then 12 (reading codebases) to recognize the strategy. Then pick chapters by what you'll touch. ~3-4 hours.

### "I'm about to write a feature that touches X"

Jump to the chapter for X, then look up the matching tour in 13.

- New SQL query: 03 → 13 Tour 2 (cache pattern)
- New Temporal workflow: 04 → 13 Tour 3
- New page: 06 → 13 Tour 4 + 5 + 6
- Auth-protected route: 05 → 08

## Conventions

- Code paths are written from the repo root, e.g. `apps/api/cmd/api/main.go`
- Commands assume you're at the repo root unless noted (`cd apps/web` when needed)
- `→` means "do this next" or "should output"
- 🛠️ marks an exercise; 💡 marks a "why" callout

## Before you start

If you haven't run `make dev` yet, skip ahead to [Chapter 02](./02-setup.md) — most chapters refer back to a running stack. You can read Chapters 01 and 03 cold, but everything from 04 onward is more fun with the services up.

For a quick sanity-check on what's working, see [`docs/manual-verification.md`](../manual-verification.md) — that's the "is everything actually running?" checklist. This tutorial is the "why does it look like this?" companion.
