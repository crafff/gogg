# crawler-lite collector

This deployment keeps a private PostgreSQL database and the exact successful
Match V5 detail/timeline response bodies on a long-running Linux host. Raw
responses are gzip-compressed and never automatically deleted.

## Initial setup

```bash
sudo mkdir -p /var/lib/gogg-collector/postgres /var/lib/gogg-collector/raw
sudo chown 65532:65532 /var/lib/gogg-collector/raw
cp config/collector.example.yaml config/collector.yaml
chmod 600 config/collector.yaml
```

Set `riot.api_key` in `config/collector.yaml`, and create an untracked `.env`
next to the compose invocation with a strong `POSTGRES_PASSWORD`. Keep
`GOGG_COLLECTOR_ROOT` on a disk with monitoring and backups.

```bash
docker compose --env-file .env -f deploy/compose/docker-compose.collector.yml build crawler-lite
docker compose --env-file .env -f deploy/compose/docker-compose.collector.yml up -d postgres migrate
docker compose --env-file .env -f deploy/compose/docker-compose.collector.yml up -d crawler-lite
```

`continue --profile daily_kr` resumes the newest unfinished lite run for that
profile. If none exists it creates one. It exits when that run completes and
does not start another run automatically. Because the container is detached,
closing SSH does not interrupt it.

Inspect it with:

```bash
docker compose --env-file .env -f deploy/compose/docker-compose.collector.yml logs -f crawler-lite
docker compose --env-file .env -f deploy/compose/docker-compose.collector.yml run --rm --no-deps crawler-lite list-runs --limit 20
```

## Daily API-key rotation

Stop gracefully so the checkpoint becomes `paused`, replace the YAML file
atomically, then start the same service. Do not edit the mounted file in place.

```bash
docker compose --env-file .env -f deploy/compose/docker-compose.collector.yml stop -t 30 crawler-lite
install -m 600 /tmp/collector.yaml.new config/collector.yaml
docker compose --env-file .env -f deploy/compose/docker-compose.collector.yml up -d crawler-lite
```

If the old key is rejected first, crawler-lite pauses automatically; updating
the file and running the final command is sufficient.

## Offline export and import

Create an incremental, self-contained tar bundle. Every file is listed in the
manifest with its uncompressed SHA-256. A later bundle repeats the match detail
when needed so a timeline bundle remains self-contained.

```bash
mkdir -p out
sudo chown 65532:65532 out
docker compose --env-file .env -f deploy/compose/docker-compose.collector.yml run --rm --no-deps \
  -v "$PWD/out:/out" crawler-lite bundle export --output /out/kr-bundle.tar
```

Copy the tar to the main database host. Configure `raw_archive` there as well,
then import it using the main database DSN:

```bash
APP_CONFIG_PATH=config/dev.yaml go run ./apps/worker/cmd/crawler-lite \
  bundle import --input /path/to/kr-bundle.tar
```

Import verifies the complete bundle before writing. Completed target records
are preserved, missing detail/timeline data is inserted, and inferred tier
metadata only fills NULL fields. Imported bundle IDs make repeated imports
safe. After a material import, refresh the existing rollups:

```bash
make refresh-rankings
make refresh-champion-detail
```

Raw archives are permanent. Alert on filesystem usage and back up both
`postgres/` and `raw/`; exporting a bundle is transport, not a backup policy.
