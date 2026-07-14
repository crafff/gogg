# crawler-lite

`crawler-lite` runs the crawler without Temporal. It stores coarse progress
directly on the existing `runs` row and resumes by re-running the current
phase.

Apply migrations first:

```bash
make migrate-up
```

Run a profile:

```bash
make run-crawler-lite args='run --profile daily_kr'
```

List runs:

```bash
make run-crawler-lite args='list-runs --limit 20'
```

Resume a paused/failed lite run:

```bash
make run-crawler-lite args='resume --run-id 21'
```

Show one run:

```bash
make run-crawler-lite args='show-run --run-id 21'
```

## Progress Model

`runs.id` remains the only run id. crawler-lite marks its rows with:

```text
runner_type = lite
```

Temporal-owned runs keep:

```text
runner_type = temporal
```

crawler-lite refuses to resume non-lite runs.

Sequential mode records the current phase only:

```text
current_phase = 3
current_tier = NULL
current_division = NULL
```

Pipeline mode records the current tier and phase:

```text
current_tier = CHALLENGER
current_phase = 5
```

Phase1 division-sliced work also records the division:

```text
current_phase = 1
current_tier = DIAMOND
current_division = III
current_page = 4
```

On resume, crawler-lite restarts from the recorded phase/tier/division/page.
Phase 1 writes the page checkpoint before each Riot request. It does not store
cursors such as `last_puuid` or `last_match_id`; existing
upsert and pending/status queries make re-running the current phase safe.

## Pause Behavior

Pressing `Ctrl+C` cancels the process context. The current phase returns at
its next context check, then crawler-lite marks:

```text
status = paused
```

The current in-flight HTTP request is not interrupted with exact persistence.
After restart, `resume --run-id` re-runs the recorded phase and continues from
database state.

## API Key Rotation

`crawler-lite` builds Riot clients at process startup. To rotate a local Riot
API key:

1. Stop crawler-lite with `Ctrl+C`.
2. Edit `deploy/secrets/dev.enc.yaml` or `config/dev.yaml`.
3. Start `resume --run-id <id>`.

The resumed process loads the new key because it rebuilds the runtime from the
current config.

## Outage Behavior

`crawler-lite` waits through transient connectivity failures instead of
failing or skipping the current player/match. This includes DNS and connection
errors, request timeouts, Riot `408`, `425`, `429`, and `5xx` responses. Retry
delays use exponential backoff with jitter, capped at two minutes. A successful
request resets the delay and processing continues from the current operation.

The wait has no time limit. `Ctrl+C` still cancels the pending request or retry
timer immediately and the run is marked `paused`. Database connection failures
at startup and safe-to-retry pgx connection errors during a phase use the same
behavior. If the database itself is unreachable when the process is stopped,
the last successfully persisted checkpoint remains the resume point.

Permanent errors are not retried indefinitely:

- Riot `401` and `403` fail the run so an invalid API key can be corrected.
- Riot `404` remains an item-level failure where the phase supports item work.
- malformed successful responses are attempted three times.
- configuration, SQL, schema, and constraint errors fail immediately.

Temporal workers retain their finite client retry policy. Infinite outage
waiting is enabled only when `crawler-lite` builds its phase set.

The outage backoff is configurable; these defaults preserve infinite waiting:

```yaml
crawler_lite:
  outage_initial_interval: 1s
  outage_max_interval: 2m
  outage_jitter: 0.2
```

Match-detail and timeline failures which are specific to one match use durable
retry scheduling in the `matches` row. Attempts are delayed by 30 seconds, 2
minutes, and 5 minutes, then the fourth failure marks that item `error`. A
`404` is marked terminal immediately. While delayed work is
pending, the phase waits rather than reporting completion. A run that finishes
with exhausted match or timeline items is marked `completed_with_errors` and
keeps the error count in `runs.last_error`.
