# Local durable data on F

The primary Windows development machine stores all durable GOGG runtime data
inside the ext4 filesystem in `F:\gogg-data\gogg-db.vhdx`. Ubuntu mounts that
filesystem at `/mnt/gogg-db`; services must not write database or archive data
directly to `/mnt/f` because that path is the Windows filesystem rather than
the ext4 filesystem required by PostgreSQL and atomic archive publication.

## Canonical layout

| Path under `/mnt/gogg-db` | Owner | Purpose |
| --- | --- | --- |
| `app-postgres` | `70:70` | Application PostgreSQL |
| `temporal-postgres` | `70:70` | Temporal PostgreSQL |
| `redis` | `999:1000` | Development Redis AOF/RDB |
| `riot-raw` | `1000:1000`, directories `2775` | Shared LoL and TFT Riot response archive |
| `game-assets` | `1000:1000`, directories `2775` | LoL/TFT static assets and profile icons |
| `prometheus` | `65534:65534` | Prometheus TSDB |
| `grafana` | `472:0` | Grafana state |
| `performance` | `1000:1000` | Local performance results |
| `experiments` | `1000:1000` | Ignored experiment datasets and artifacts |

LoL responses use `riot-raw/<REGION>/...`; TFT responses use
`riot-raw/TFT/<PLATFORM>/...`, so both collectors can safely share one root.

`make dev`, `make run-api`, `make run-worker`, `make run-tft-worker`,
`make run-crawler-lite`, and `make sync-assets` fail closed unless the expected
UUID is mounted read-write as ext4 and every target has the expected ownership
and mode. Overrides for the raw, asset, TFT static, and performance roots must
still resolve beneath the same F-backed filesystem. The Makefile exports the
following defaults:

```text
GOGG_DATA_ROOT=/mnt/gogg-db
GOGG_RAW_ARCHIVE_ROOT=/mnt/gogg-db/riot-raw
GOGG_ASSETS_ROOT=/mnt/gogg-db/game-assets
GOGG_TFT_STATIC_ROOT=/mnt/gogg-db/game-assets
```

Run the preflight directly with:

```bash
make dev-storage-check
```

The production API and TFT images run as UID `65532`. Their Compose services
add the configurable supplementary group `GOGG_DATA_GID` (default `1000`);
the setgid, group-writable asset and raw directories keep files created by host
and container workers in that shared group. Static assets remain read-only in
the API, with only the nested `profile-icons` cache mounted read-write.
Initialize another production host with the matching numeric group, directory
modes, and group-readable files before starting the services.

## Migration and recovery rules

Before copying a writable directory, stop the API, all workers and the
corresponding Compose service. Copy into the F-backed filesystem, preserve
numeric ownership, compare file counts/checksums, and only then switch the
bind mount. Never use `make dev-reset` for this operation and never delete the
old Docker named volumes until the replacement has survived a cold restart.

The old `gogg_postgres_data` and crawler-test volumes are not part of the
active stack. Preserve or archive them separately; do not merge them into the
current application database.

Docker images, build caches, container writable layers and stdout/stderr logs
remain Docker Desktop engine data. They are reproducible infrastructure data,
not GOGG's durable application dataset; relocating them requires moving the
Docker Desktop data disk as a separate operation.
