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
```

On resume, crawler-lite restarts from the recorded phase/tier/division. It
does not store cursors such as `last_puuid` or `last_match_id`; existing
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
