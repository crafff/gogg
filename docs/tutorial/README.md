# Tutorial: how to read GOGG

This is a hand-held walkthrough that takes you from "what is this repo?" to "I understand how every layer fits together." It assumes you can use a command line and know a bit of programming, but **doesn't assume** Go, React, GraphQL, Temporal, or PostgreSQL. Each chapter introduces the concepts as they come up.

## How to use this

Read each chapter in order, the first time through. Every chapter has:

- **Mental model** — the concepts you need before reading code
- **Read these files** — exact paths in tree order, smallest-to-biggest
- **Try this** — concrete commands you can run while reading
- **Exercises** — small modifications to deepen understanding
- **Why it's built this way** — links to relevant ADRs / plan sections

Time budget per chapter: 30–60 min if you're new to the stack, faster if you already know Go / React.

## Chapters

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
| 09 | [Next steps](./09-next-steps.md) | What Phase E + F look like, how to start contributing |

## Conventions

- Code paths are written from the repo root, e.g. `apps/api/cmd/api/main.go`
- Commands assume you're at the repo root unless noted (`cd apps/web` when needed)
- `→` means "do this next" or "should output"
- 🛠️ marks an exercise; 💡 marks a "why" callout

## Before you start

If you haven't run `make dev` yet, skip ahead to [Chapter 02](./02-setup.md) — most chapters refer back to a running stack. You can read Chapters 01 and 03 cold, but everything from 04 onward is more fun with the services up.

For a quick sanity-check on what's working, see [`docs/manual-verification.md`](../manual-verification.md) — that's the "is everything actually running?" checklist. This tutorial is the "why does it look like this?" companion.
