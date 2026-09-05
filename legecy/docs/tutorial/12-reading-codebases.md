# Chapter 12 · How to read a codebase (meta-skill)

> Goal: by the end of this chapter you have a repeatable process for getting productive in any unfamiliar 30k+ LOC codebase. This is a meta-skill: the techniques apply to GOGG, to your day job, to open source projects you contribute to.

The naive approach — open the repo, alphabetically read every file — fails. A 30k-line project takes a week to read straight through, and at the end you've memorized syntax but understood nothing. This chapter is the alternative.

## The unifying principle: follow the data

Every program is "data comes in, something happens, data goes out." For a web service: HTTP request in, SQL/RPC out, response back. For a CLI: argv in, side effects out. For a worker: queue tick in, job done out.

If you can answer "for one specific input, what files run, in what order, and what data flows between them?" — you understand the project. Everything else is detail.

The end-to-end trace from [Chapter 07](./07-end-to-end.md) is exactly this exercise for GOGG. The first thing you should do in any new codebase is pick **one concrete user action** and trace it.

## The 7-step playbook

Here's the order to learn a new repo.

### Step 1 — Read the README + docs first (10–20 min)

Specifically these, in order:

1. `README.md` — the elevator pitch. What is this? Who's it for? How do I run it?
2. `CONTRIBUTING.md` / `docs/contributing.md` — what tooling, what conventions, what the workflow looks like.
3. `CLAUDE.md` / `NOTES.md` / similar "load-bearing context" doc if it exists — the maintainers' shorthand. GOGG has this.
4. `docs/architecture/` — if there are ADRs, skim them. Don't try to absorb them yet; you'll come back.

Make a one-paragraph mental summary: "This is a $thing that does $thing for $audience by $approach."

If the docs don't answer "how do I run it?" within 5 minutes, that's a red flag — note it as your first contribution opportunity.

### Step 2 — Map the top-level structure (5 min)

```bash
ls -la
tree -L 2 -d         # or use find: find . -maxdepth 2 -type d
```

What directories exist? Are they self-explanatory? In GOGG:
`apps/`, `packages/`, `config/`, `deploy/`, and `docs/`. Already you
have the shape — three processes + shared packages + config + infra +
docs.

Map the layout against any "well-known" convention you recognize:

- `cmd/<bin>/main.go` + `internal/` + `pkg/` — standard Go layout.
- `src/` + `tests/` + `pyproject.toml` — Python project.
- `apps/` + `packages/` — monorepo (turborepo, nx, Go workspaces).

You're not memorizing files. You're answering: "What kind of project is this?"

### Step 3 — Run it locally (15–60 min)

You cannot truly understand code you haven't run. The setup is the first painful step; the docs from step 1 should walk you through it.

When something breaks during setup:

- **Look at the error message before googling.** Half the time it tells you exactly what's wrong.
- **Check assumptions.** Wrong Go version? Wrong Node? Missing tool?
- **Try the troubleshooting section** if there is one. (Our `docs/manual-verification.md` has a Troubleshooting tail; many projects don't.)
- **If genuinely stuck, ask.** Stalling on setup for >2h without asking is a productivity bug. Write up what you tried + what you saw and ping someone.

By the end of this step, you can interact with the running thing — hit the endpoints, navigate the UI, watch the logs. That's the foundation everything else builds on.

### Step 4 — Find the entry points (15 min)

Every binary / process has one. Find them and read them.

For Go:

```bash
find . -name 'main.go' -not -path '*/vendor/*'
```

For Node:

```bash
grep -l '"main":' package.json apps/*/package.json
# or
grep -l '"scripts":' package.json apps/*/package.json   # the "dev" / "start" scripts
```

For Python:

```bash
grep -rl 'if __name__ == ' --include='*.py'
```

Now read each `main()` start to finish. In GOGG's case:

- `apps/api/cmd/api/main.go` → 90 lines, builds the dependency graph + starts the server.
- `apps/worker/cmd/worker/main.go` → 80 lines, builds the runtime + registers workflows + starts the worker.
- `apps/web/src/main.tsx` → 15 lines, mounts `<App />`.

Reading `main` tells you:

- What dependencies exist (DB, cache, queue, secrets)
- How they're wired
- What lifecycle events matter (startup probes, graceful shutdown)
- Where to put a breakpoint if you need to debug startup

**This is the single highest-leverage 15 minutes of new-project onboarding.**

### Step 5 — Pick one feature, trace it end-to-end (1–2 hours)

This is the [Chapter 07](./07-end-to-end.md) exercise. Pick an action you can perform manually:

- "User opens the rankings page" → trace browser → React → GraphQL → service → SQL → response
- "User signs in" → trace OAuth redirect → callback → token issue → cookie set
- "Cron fires" → trace Schedule → workflow → activities → DB writes

Use these techniques as you trace:

#### Grep aggressively

When you have a string from the UI (a label, an error message, an endpoint path):

```bash
grep -rn 'No data found' apps/ packages/
grep -rn '/api/v1/rankings/champions' .
grep -rn 'CrawlRegionWorkflow' .
```

That jumps you to the source of the user-visible thing.

#### Follow imports

Once you've found a function:

1. Read its body.
2. Find what it calls (other functions, methods).
3. Use your editor's "go to definition" or `grep -rn 'func.*FunctionName'` to find them.
4. Repeat 1–3 until you hit a "leaf" (an SQL query, an HTTP call, a third-party API).

You're building a function-call tree in your head. Don't read details yet — read signatures + comments. Once you have the tree, the details matter.

#### Read tests to learn intent

Tests document what code is *supposed* to do. Often clearer than the code itself.

When you don't understand a function, find its test:

```bash
grep -rn 'func TestFunctionName\|describe.*ComponentName' .
```

Read the test cases. Each case is a worked example with expected input + expected output. Patterns emerge quickly.

In GOGG, look at `apps/api/internal/auth/jwt_test.go` to understand the Issuer. Look at `apps/web/src/features/rankings/hooks/useFadeTransition.test.ts` to understand the state machine. Tests as documentation.

### Step 6 — Build the dependency graph mentally (30 min)

After tracing one feature, you know one slice. Now broaden.

Look at `main.go` again with the trace in your head. Notice every other dependency that you didn't touch in your trace. Each is a different feature's slice.

```
                 main()
                ┌──┴──┬──────┬─────────┬─────┐
                ▼     ▼      ▼         ▼     ▼
            chi    pgxpool redisClient temporal  oauth
              │       │        │         │       │
              │     queries  cache    workflow  providers
              │       │        │         │       │
              transport    service    activities
              │     │
              REST  GraphQL  ...
```

Draw it on paper if you must. Or just hold the rough shape. Each branch is a future trace.

### Step 7 — Make a small change (variable time)

The first PR is the proof of understanding. Pick something tiny:

- Typo in a comment
- Add a log line
- Fix an obvious bug you noticed
- Improve a doc

The point isn't the change. The point is going through the full lifecycle: branch, edit, build, test, format, lint, push, PR, CI, review, merge. Now you know the full developer loop. The next change can be bigger.

## Anti-patterns to avoid

### ❌ Reading alphabetically

`ls apps/api/internal/` then opening file 1, file 2, file 3... You'll read for a week and learn nothing because you don't have context for any file individually.

The fix: trace from `main()`. Files become meaningful in the order they're used.

### ❌ Reading the build system before the code

`Makefile`, `package.json`, `go.work`, CI configs — fascinating eventually, useless at first. They tell you *how to build*, not *what is being built*.

The fix: defer build-system reading until you have a concrete frustration ("why does this command exist?").

### ❌ Trying to understand every abstraction

When you hit a function that calls a thing that calls a thing through 5 layers of interfaces, the temptation is "let me understand the whole abstraction first." Don't.

The fix: trust the abstraction at first. Read the interface (it tells you what's possible), not every implementation. You'll understand details when you need to modify them.

### ❌ Memorizing instead of bookmarking

You can't memorize a 30k-line codebase. Don't try.

The fix:

- Use editor bookmarks (or just open tabs for the 5 files you keep returning to).
- Write a personal `NOTES.md` for things you keep re-discovering ("auth flow is in `auth/jwt.go` line ~80").
- Trust grep + go-to-definition over recall.

### ❌ Avoiding the debugger

In Go and TypeScript both, `printf`/`console.log` debugging is fast and effective. But sometimes the real debugger (`delve` for Go, browser DevTools for JS) is the only way to see the actual runtime values.

The fix: learn the basics of the debugger for your stack. Set a breakpoint at `main()`, step over the first few calls. The first time you see the DI wiring happen at runtime is a revelation.

### ❌ Ignoring git history

Why is this code shaped this way? The commit message often explains it.

```bash
git log -p -- path/to/file
git blame path/to/file
git log --all --oneline -- path/to/file
```

For GOGG specifically, every Phase B/C/D commit explains the chunk's rationale. Read the commit messages — they're documentation.

`gh pr view` is great too:

```bash
gh pr list --state merged --search 'in:title rankings'
gh pr view 4
```

PRs often have "test plan" + design rationale that didn't make it into the code.

## Grep + tooling cheatsheet

A few one-liners that pay off forever:

```bash
# Find every TODO comment
grep -rn 'TODO\|FIXME\|XXX' --exclude-dir=node_modules .

# Find usages of a symbol across languages
grep -rn 'CrawlRegionWorkflow' .

# Find where an endpoint is registered (Go-ish)
grep -rn 'Route\|HandleFunc\|Get\(\|Post\(' apps/

# Find every place env vars are read
grep -rEn 'os\.Getenv|os\.LookupEnv|process\.env\.' .

# Recursively grep with context
grep -rn -B 2 -A 5 'panic\(' apps/

# Find big files (often god-files)
find apps/ -name '*.go' -size +10k -exec wc -l {} \; | sort -n

# Find recently changed files
git log --since='2 weeks ago' --name-only --pretty=format: | sort -u
```

For deeper navigation:

- **`ripgrep` (rg)** — 10x faster grep, smarter defaults. `rg pattern path` is the everyday workhorse.
- **`fd`** — faster, friendlier `find`.
- **Editor go-to-definition + find-references** — vscode + gopls + ts-server + eslint-language-server. Make sure they're working before you read code.

## When to ask vs when to read

Junior heuristic: read everything yourself, never ask.
Senior heuristic: read for an hour; if stuck, ask.

The question is "what's the cost?" If a teammate can answer in 5 minutes what would take you 2 hours, asking is the right move. Just:

1. **State what you tried.** "I looked at X, Y, Z. The unclear part is W."
2. **Ask a specific question.** "Why does X depend on Y?" not "How does this work?"
3. **Listen to the answer.** Take notes.
4. **Confirm the new model.** Restate what you understood, ask if you got it right.

This is also how you learn the team's vocabulary — words like "the runtime," "the resolver," "phase 3.5" mean specific things in this project. You absorb them by talking with maintainers.

## How to read this specific project

Applying the playbook to GOGG:

1. **Docs**: `README.md`, `CLAUDE.md`, this tutorial's Chapter 01.
2. **Structure**: `ls apps/ packages/ config/ deploy/ docs/`.
3. **Run**: Chapter 02. Bring up the dev stack, render the rankings page.
4. **Entry points**: `apps/api/cmd/api/main.go`, `apps/worker/cmd/worker/main.go`, `apps/web/src/main.tsx`.
5. **Trace**: pick `GET /api/v1/versions`. Walk it through chi → service → sqlc. Then re-do for `championRankings` GraphQL.
6. **Graph**: Chapter 01's architecture diagram. Note the three binaries + Postgres + Redis + Temporal + Riot.
7. **Change**: add a `slog.Info` to the catalog service per Chapter 05's exercise.

If you did Chapters 02 + 05 + 07 in that order, you've executed the playbook.

## Habits that compound

As you do this more (in this project + others):

- **Keep a "questions log."** A file where you write down "I don't understand X." Every time you understand one, cross it out. Patterns emerge — you spot the architectural confusions vs the syntactic ones.
- **Re-read your own code from 3 months ago.** It feels foreign. That's calibration — you've grown. Try to bring the same critical eye to today's code.
- **Onboard someone else.** Teaching is the fastest way to solidify your own understanding. If you can walk a new person through GOGG (using this tutorial), you understand it more than you think.

## Going further

- [How to Read a Paper (S. Keshav)](https://web.stanford.edu/class/ee384m/Handouts/HowtoReadPaper.pdf) — academic-paper-focused, but the "three passes" framework (skim → grasp → reproduce) applies to code too.
- [Erin Kissane — Code Reading](https://incident.io/blog/code-reading-tactics) — practical tactics for understanding others' code.
- [Marianne Bellotti — Kill It With Fire](https://nostarch.com/kill-it-fire) — book on legacy system understanding; even if your codebase isn't legacy, the techniques transfer.
- [Julia Evans' debugging zines](https://jvns.ca/) — short, illustrated explanations of debugging tools. Excellent for systems-level understanding.

## Up next

[Chapter 13 — Annotated code tour](./13-annotated-tour.md) walks through 6 classic snippets from this codebase line-by-line, explaining each construct as it appears. After reading it, you'll have model examples of how each layer looks in idiomatic form.
