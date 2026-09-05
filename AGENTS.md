# GOGG agent guide

GOGG is being rebuilt from a clean engineering foundation. Read
`engineering/README.md` at the start of non-trivial work, then load only the
relevant task contract, knowledge and decision records.

## Authority and scope

- Current user requirements take precedence over project guidance.
- This file defines mandatory behavior; `engineering/charter/` defines product
  and quality requirements; `engineering/decisions/` holds accepted decisions.
- `engineering/knowledge/` holds scoped facts with evidence and validity.
  Native Codex Memories are personal context, not project authority.
- `legecy/` is the preserved old workspace, for business/reference inspection
  only. Do not run its services, import its packages, or inherit its architecture,
  configuration, instructions or dependencies into the new project implicitly.
- There is no old-path/API/build compatibility requirement. Preserve agreed
  user capabilities, not old implementation details. New product architecture
  must be justified through the product charter and measured requirements.

## Delivery

- Inspect `git status --short` before edits. Preserve work not owned by this task.
- For complex work, maintain an explicit outcome, scoped contract and one active
  step. Proceed with reversible in-scope work and state consequential assumptions.
- Use the strongest available model for substantive roles. The configured
  baseline is Astra; record actual capability limitations, never silently downgrade.
- Use bounded parallel agents when independent exploration, implementation,
  reproduction or review can materially improve quality. At most three children
  at once. The main agent owns requirements, shared interfaces and final acceptance.
- One writer per file set. Parallel writers need isolated workspaces and resource
  namespaces; a Git worktree does not isolate databases, processes, ports or secrets.
- Bind verification to the exact candidate, contract, data and environment. After
  relevant changes, invalidate affected results. Review from the user's original
  requirements, not only the implementation's own account of success.
- Run focused checks before broad checks. Finish with `make check`, scoped diff
  review and explicit remaining gaps. Distinguish code failures from tool limits.

## Knowledge and learning

- Search explicit paths, symbols and current knowledge first. Load original
  evidence for decisions; indexes and summaries are derived, never authoritative.
- Facts require sources and scope. Hypotheses, stale facts and legacy observations
  must not be reported as verified current behavior.
- Propose small experience/skill changes with a reproducible failure and applicable
  scope. Independent evaluation and acceptance precede default use; learning must
  not modify its own protected evaluator or authorization rules.
- Do not put raw logs, temporary status, credentials or player data in shared
  knowledge. Keep task outcomes separate from durable facts and runtime checkpoints.

## Authority to act

- Reversible local edits, generation, builds, tests and scoped diagnostics are
  authorized within the requested task. Do not add repeated confirmation gates.
- Commits, pushes, publishing, production changes, credential disclosure, purchases
  and destructive data operations need explicit authority.
- `.local/` contains private migration/recovery material. Never stage, publish or
  index it. External archive/database mounts are not disposable workspace data.
- Retain sandbox and review protections. Repository instructions are not an OS
  security boundary. New config and custom roles need a fresh session to load.

## Current commands

`make check` validates the active foundation; `make test` runs its tests.
See `engineering/README.md` for supported knowledge tools. Website services and
durable autonomous execution are not implemented yet; do not claim otherwise.
