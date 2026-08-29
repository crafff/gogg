# GOGG AI development system

This repository uses Codex as an autonomous but bounded engineering teammate.
The system is designed around GPT-5.6 Sol for high-value reasoning, durable
repository context, focused verification, and selective parallelism.

## System shape

```text
user goal
   |
   v
primary agent: GPT-5.6 Sol
requirements -> decisions -> integration -> final verification
   |                 |                    |
   |                 |                    +-- gogg_reviewer (Sol, read-only)
   |                 +-- gogg_verifier (Sol, tests and triage)
   +-- gogg_explorer (Sol, read-only mapping)

context layers
prompt/thread -> AGENTS.md -> project-memory.md -> ADRs/code/tests
```

The primary agent stays responsible for the whole outcome. Subagents reduce
context pollution and wall-clock time on independent work; they do not replace
ownership or final review.

## Why these model roles

- GPT-5.6 Sol is the main agent because GOGG changes often cross React,
  GraphQL, Go services, Temporal workflows, Riot semantics, and PostgreSQL.
  These tasks benefit from deeper planning, judgment, and integration.
- GPT-5.6 Sol also handles exploration and routine verification so every agent
  uses the same high-capability model and can follow cross-layer implications.
- The reviewer uses Sol for broad or high-risk changes. Small changes should
  remain single-agent to avoid unnecessary coordination and token overhead.

All agents use `xhigh` reasoning effort. The repository caps spawned agents at
three. Parallel agents should normally read or test; only one agent should own
a given set of edits.

## Automatic approval model

The repository uses `workspace-write`, interactive approval boundaries, and
Codex Auto-review. Routine work inside the repository proceeds directly.
Actions that cross the sandbox are reviewed automatically against the policy
in `.codex/config.toml` instead of being sent to the user one by one.

This is intentionally different from disabling the sandbox. Destructive data
loss, production mutations, credentials, publishing, purchases, and unrelated
external side effects still require explicit authority or are denied. In
particular, `make dev-reset` is never routine automation.

Configuration changes are loaded at the start of a new Codex session. The
repository must be trusted for project-local `.codex` configuration and custom
agents to load.

## Three memory layers

| Layer | Purpose | Update rule |
|---|---|---|
| Native Codex Memories | Personal preferences and useful cross-chat recall | Generated locally after eligible idle chats |
| `AGENTS.md` | Mandatory working rules and validation expectations | Change when the engineering contract changes |
| `project-memory.md` | Shared, versioned product facts and learned failure modes | Change only after evidence or an accepted decision |

ADRs remain the home for consequential architecture decisions. Temporary plans,
raw logs, hypotheses, and incomplete task status should remain in the current
thread instead of becoming durable memory.

Native memory is configured not to learn from chats that use external web or
MCP context. That reduces the chance that untrusted external text becomes
persistent guidance. Important verified facts from those sessions can still be
added deliberately to project memory with their source or code evidence.

## Task routing

| Task shape | Recommended execution |
|---|---|
| Small, well-scoped change | Primary agent only |
| Unknown cross-layer execution path | Explorer, then primary agent |
| Independent backend and frontend investigation | Two explorers in parallel |
| Implementation plus expensive tests | Primary agent writes while verifier runs independent checks |
| Broad diff or release-sensitive work | Verifier and reviewer in parallel after implementation |
| Multiple agents would edit the same files | Keep a single writer; do not parallelize |

## Continuous improvement

When the same failure happens twice, the primary agent should perform a short
retrospective:

1. Determine whether the cause was missing context, a weak invariant, a missing
   test, or an environment problem.
2. Fix the product or test first when possible.
3. Update `AGENTS.md` only for a generally applicable rule.
4. Update project memory only for a durable fact or proven lesson.
5. Create a skill or hook only after the workflow is stable and repeated.

This keeps the instruction surface compact and prevents stale process from
crowding out the actual task.
