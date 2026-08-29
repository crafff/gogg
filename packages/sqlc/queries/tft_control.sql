-- name: CreateTFTRun :one
INSERT INTO tft_crawl_runs (
    workflow_id, workflow_run_id, schedule_id, profile_name, platform, routing_region,
    queue_type, queue_id, status, target_patch, target_set,
    window_start, window_end, config, started_at
) VALUES (
    @workflow_id, @workflow_run_id, @schedule_id, @profile_name, @platform, @routing_region,
    @queue_type, @queue_id, @status, sqlc.narg(target_patch), sqlc.narg(target_set),
    @window_start, @window_end, @config, CASE WHEN @status::text = 'running' THEN now() ELSE NULL END
)
RETURNING *;

-- name: GetTFTRunByWorkflowRunID :one
SELECT * FROM tft_crawl_runs WHERE workflow_run_id = $1;

-- name: FailStaleTFTRuns :execrows
UPDATE tft_crawl_runs
SET status = 'failed', desired_state = 'running', stage = 'reconciled',
    last_error = 'stale active run superseded by a newer Temporal execution',
    ended_at = now(), updated_at = now()
WHERE schedule_id = @schedule_id
  AND profile_name = @profile_name
  AND platform = @platform
  AND workflow_run_id <> @workflow_run_id
  AND status IN ('queued','running','pausing','paused','paused_needs_auth');

-- name: UpdateTFTRunState :exec
UPDATE tft_crawl_runs
SET status = @status,
    desired_state = @desired_state,
    stage = @stage,
    last_error = sqlc.narg(last_error),
    started_at = CASE WHEN @status::text = 'running' THEN COALESCE(started_at, now()) ELSE started_at END,
    ended_at = CASE WHEN @status::text IN ('completed','completed_with_errors','cancelled','failed') THEN now() ELSE NULL END,
    updated_at = now()
WHERE id = @id;

-- name: IncrementTFTRunCounts :exec
UPDATE tft_crawl_runs
SET discovered_seeds = discovered_seeds + @seeds,
    discovered_matches = discovered_matches + @matches,
    completed_matches = completed_matches + @completed,
    terminal_matches = terminal_matches + @terminal,
    updated_at = now()
WHERE id = @id;

-- name: RefreshTFTRunCounts :exec
UPDATE tft_crawl_runs run
SET discovered_seeds = (
        SELECT COUNT(*) FROM tft_seed_snapshots seeds WHERE seeds.run_id = run.id
    ),
    discovered_matches = (
        SELECT COUNT(*) FROM tft_match_discoveries discoveries WHERE discoveries.run_id = run.id
    ),
    completed_matches = (
        SELECT COUNT(DISTINCT jobs.match_id)
        FROM tft_match_discoveries discoveries
        JOIN tft_match_jobs jobs
          ON jobs.routing_region = discoveries.routing_region AND jobs.match_id = discoveries.match_id
        WHERE discoveries.run_id = run.id AND jobs.status = 'completed'
    ),
    terminal_matches = (
        SELECT COUNT(DISTINCT jobs.match_id)
        FROM tft_match_discoveries discoveries
        JOIN tft_match_jobs jobs
          ON jobs.routing_region = discoveries.routing_region AND jobs.match_id = discoveries.match_id
        WHERE discoveries.run_id = run.id AND jobs.status = 'terminal'
    ),
    updated_at = now()
WHERE run.id = @id;

-- name: UpsertTFTCheckpoint :exec
INSERT INTO tft_crawl_checkpoints (
    run_id, stage, scope_key, cursor, processed, next_eligible_at, attempt, updated_at
) VALUES (
    @run_id, @stage, @scope_key, @cursor, @processed, @next_eligible_at, @attempt, now()
)
ON CONFLICT (run_id, stage, scope_key) DO UPDATE
SET cursor = EXCLUDED.cursor,
    processed = EXCLUDED.processed,
    next_eligible_at = EXCLUDED.next_eligible_at,
    attempt = EXCLUDED.attempt,
    lease_owner = NULL,
    lease_expires_at = NULL,
    updated_at = now();

-- name: GetTFTCheckpoint :one
SELECT * FROM tft_crawl_checkpoints
WHERE run_id = @run_id AND stage = @stage AND scope_key = @scope_key;

-- name: UpsertTFTSeedSnapshot :exec
INSERT INTO tft_seed_snapshots (
    run_id, platform, queue_type, cohort, tier, division, puuid,
    league_id, league_points, wins, losses, sample_bucket, selected, captured_at
) VALUES (
    @run_id, @platform, @queue_type, @cohort, @tier, sqlc.narg(division), @puuid,
    sqlc.narg(league_id), sqlc.narg(league_points), sqlc.narg(wins), sqlc.narg(losses),
    sqlc.narg(sample_bucket), @selected, now()
)
ON CONFLICT (run_id, platform, queue_type, puuid, tier, (COALESCE(division, ''))) DO UPDATE
SET cohort = EXCLUDED.cohort,
    league_id = EXCLUDED.league_id,
    league_points = EXCLUDED.league_points,
    wins = EXCLUDED.wins,
    losses = EXCLUDED.losses,
    sample_bucket = EXCLUDED.sample_bucket,
    selected = EXCLUDED.selected,
    captured_at = now();

-- name: ListSelectedTFTSeeds :many
SELECT * FROM tft_seed_snapshots
WHERE run_id = @run_id AND selected
ORDER BY cohort, tier, division NULLS FIRST, puuid;

-- name: ListTFTSeedsForSampling :many
SELECT * FROM tft_seed_snapshots
WHERE run_id = $1
ORDER BY tier, division NULLS FIRST, league_points DESC NULLS LAST, puuid;

-- name: SetTFTSeedSelected :exec
UPDATE tft_seed_snapshots SET selected = @selected WHERE id = @id;

-- name: ListSelectedTFTSeedsPage :many
SELECT * FROM tft_seed_snapshots
WHERE run_id = @run_id AND platform = @platform AND selected
ORDER BY cohort, tier, division NULLS FIRST, puuid
OFFSET @row_offset LIMIT @row_limit;

-- name: UpsertTFTPlayerMatchSync :exec
INSERT INTO tft_player_match_sync (
    platform, puuid, queue_type, window_start, window_end, last_synced_at, last_match_id
) VALUES (
    @platform, @puuid, @queue_type, @window_start, @window_end, @last_synced_at, sqlc.narg(last_match_id)
)
ON CONFLICT (platform, puuid, queue_type) DO UPDATE
SET window_start = LEAST(tft_player_match_sync.window_start, EXCLUDED.window_start),
    window_end = GREATEST(tft_player_match_sync.window_end, EXCLUDED.window_end),
    last_synced_at = EXCLUDED.last_synced_at,
    last_match_id = EXCLUDED.last_match_id,
    updated_at = now();

-- name: GetTFTPlayerMatchSync :one
SELECT * FROM tft_player_match_sync
WHERE platform = @platform AND puuid = @puuid AND queue_type = @queue_type;

-- name: InsertTFTMatchDiscovery :execrows
INSERT INTO tft_match_discoveries (
    run_id, platform, routing_region, seed_puuid, match_id, cohort
) VALUES (@run_id, @platform, @routing_region, @seed_puuid, @match_id, @cohort)
ON CONFLICT DO NOTHING;

-- name: EnqueueTFTMatchJob :execrows
INSERT INTO tft_match_jobs (routing_region, match_id, platform)
VALUES (@routing_region, @match_id, @platform)
ON CONFLICT (routing_region, match_id) DO NOTHING;

-- name: RequeueTFTMatchesForPatch :execrows
UPDATE tft_match_jobs jobs
SET status = 'pending', attempt = 0, next_eligible_at = now(),
    lease_owner = NULL, lease_expires_at = NULL,
    last_status_code = NULL, last_error = NULL, completed_at = NULL,
    updated_at = now()
FROM tft_matches matches
WHERE matches.match_id = jobs.match_id
  AND matches.patch = @target_patch
  AND matches.exclusion_reason = 'out_of_scope_patch'
  AND jobs.status = 'completed';

-- name: ClaimTFTMatchJobs :many
WITH candidates AS (
    SELECT pending.routing_region, pending.match_id
    FROM tft_match_jobs pending
    WHERE pending.routing_region = sqlc.arg(route_filter)
      AND (
        (pending.status IN ('pending','retry') AND pending.next_eligible_at <= now()
          AND (pending.lease_expires_at IS NULL OR pending.lease_expires_at <= now()))
        OR (pending.status = 'leased' AND pending.lease_expires_at <= now())
      )
    ORDER BY COALESCE(pending.lease_expires_at, pending.next_eligible_at), pending.discovered_at
    LIMIT @row_limit
    FOR UPDATE SKIP LOCKED
)
UPDATE tft_match_jobs j
SET status = 'leased',
    lease_owner = @lease_owner,
    lease_expires_at = now() + make_interval(secs => @lease_seconds::int),
    attempt = j.attempt + 1,
    updated_at = now()
FROM candidates c
WHERE j.routing_region = c.routing_region AND j.match_id = c.match_id
RETURNING j.*;

-- name: CompleteTFTMatchJob :execrows
UPDATE tft_match_jobs
SET status = @status,
    last_status_code = sqlc.narg(last_status_code),
    last_error = sqlc.narg(last_error),
    lease_owner = NULL,
    lease_expires_at = NULL,
    completed_at = CASE WHEN @status::text IN ('completed','terminal') THEN now() ELSE NULL END,
    updated_at = now()
WHERE routing_region = @routing_region AND match_id = @match_id
  AND lease_owner = @lease_owner AND lease_expires_at > now();

-- name: RetryTFTMatchJob :execrows
UPDATE tft_match_jobs
SET status = 'retry',
    next_eligible_at = @next_eligible_at,
    last_status_code = sqlc.narg(last_status_code),
    last_error = @last_error,
    lease_owner = NULL,
    lease_expires_at = NULL,
    updated_at = now()
WHERE routing_region = @routing_region AND match_id = @match_id
  AND lease_owner = @lease_owner AND lease_expires_at > now();

-- name: ReleaseTFTMatchLeases :exec
UPDATE tft_match_jobs
SET status = 'retry', next_eligible_at = now(), lease_owner = NULL,
    lease_expires_at = NULL, updated_at = now()
WHERE routing_region = @routing_region AND lease_owner = @lease_owner AND status = 'leased';

-- name: GetTFTMatchQueueState :one
SELECT COUNT(*)::bigint AS remaining,
       MIN(CASE WHEN status = 'leased' THEN lease_expires_at ELSE next_eligible_at END)::timestamptz AS next_eligible_at
FROM tft_match_jobs
WHERE routing_region = @routing_region AND status IN ('pending','retry','leased');
