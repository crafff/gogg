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

-- name: GetTFTRunByID :one
SELECT * FROM tft_crawl_runs WHERE id = $1;

-- name: GetLatestTFTRunByScheduleID :one
SELECT *
FROM tft_crawl_runs
WHERE schedule_id = $1
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetTFTRunProgress :one
SELECT
    (SELECT COUNT(*)::bigint FROM tft_seed_snapshots WHERE run_id = run.id) AS discovered_seeds,
    (SELECT COUNT(*)::bigint FROM tft_seed_snapshots WHERE run_id = run.id AND selected) AS selected_seeds,
    (
        SELECT COUNT(DISTINCT checkpoints.scope_key)::bigint
        FROM tft_crawl_checkpoints checkpoints
        WHERE checkpoints.run_id = run.id
          AND checkpoints.stage IN ('match_discovery', 'match_candidates')
          AND checkpoints.processed >= (
              SELECT COUNT(*)
              FROM tft_seed_snapshots seeds
              WHERE seeds.run_id = run.id
                AND seeds.platform = checkpoints.scope_key
                AND seeds.selected
          )
    ) AS completed_platforms
FROM tft_crawl_runs run
WHERE run.id = @run_id;

-- name: ListTFTRunRouteProgress :many
WITH routes AS (
    SELECT 'AMERICAS'::text AS routing_region
    UNION ALL SELECT 'ASIA'::text
    UNION ALL SELECT 'EUROPE'::text
    UNION ALL SELECT 'SEA'::text
), discovered AS (
    SELECT DISTINCT routing_region, match_id
    FROM tft_match_discoveries
    WHERE run_id = @run_id
), run_counts AS (
    SELECT
        discovered.routing_region,
        COUNT(*)::bigint AS discovered_matches,
        COUNT(*) FILTER (WHERE jobs.status = 'completed')::bigint AS completed_matches,
        COUNT(*) FILTER (WHERE jobs.status = 'terminal')::bigint AS terminal_matches,
        COUNT(*) FILTER (WHERE jobs.status = 'pending')::bigint AS pending_matches,
        COUNT(*) FILTER (WHERE jobs.status = 'retry')::bigint AS retry_matches,
        COUNT(*) FILTER (WHERE jobs.status = 'leased')::bigint AS leased_matches,
        COUNT(*) FILTER (WHERE jobs.match_id IS NULL)::bigint AS not_enqueued_matches
    FROM discovered
    LEFT JOIN tft_match_jobs jobs
      ON jobs.routing_region = discovered.routing_region
     AND jobs.match_id = discovered.match_id
    GROUP BY discovered.routing_region
), global_counts AS (
    SELECT
        routing_region,
        COUNT(*) FILTER (WHERE status = 'pending')::bigint AS global_pending_matches,
        COUNT(*) FILTER (WHERE status = 'retry')::bigint AS global_retry_matches,
        COUNT(*) FILTER (WHERE status = 'leased')::bigint AS global_leased_matches
    FROM tft_match_jobs
    WHERE status IN ('pending', 'retry', 'leased')
    GROUP BY routing_region
)
SELECT
    routes.routing_region,
    COALESCE(run_counts.discovered_matches, 0)::bigint AS discovered_matches,
    COALESCE(run_counts.completed_matches, 0)::bigint AS completed_matches,
    COALESCE(run_counts.terminal_matches, 0)::bigint AS terminal_matches,
    COALESCE(run_counts.pending_matches, 0)::bigint AS pending_matches,
    COALESCE(run_counts.retry_matches, 0)::bigint AS retry_matches,
    COALESCE(run_counts.leased_matches, 0)::bigint AS leased_matches,
    COALESCE(run_counts.not_enqueued_matches, 0)::bigint AS not_enqueued_matches,
    COALESCE(global_counts.global_pending_matches, 0)::bigint AS global_pending_matches,
    COALESCE(global_counts.global_retry_matches, 0)::bigint AS global_retry_matches,
    COALESCE(global_counts.global_leased_matches, 0)::bigint AS global_leased_matches
FROM routes
LEFT JOIN run_counts USING (routing_region)
LEFT JOIN global_counts USING (routing_region)
ORDER BY 1;

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
        SELECT COUNT(DISTINCT (discoveries.routing_region, discoveries.match_id))
        FROM tft_match_discoveries discoveries
        WHERE discoveries.run_id = run.id
    ),
    completed_matches = (
        SELECT COUNT(DISTINCT (jobs.routing_region, jobs.match_id))
        FROM tft_match_discoveries discoveries
        JOIN tft_match_jobs jobs
          ON jobs.routing_region = discoveries.routing_region AND jobs.match_id = discoveries.match_id
        WHERE discoveries.run_id = run.id AND jobs.status = 'completed'
    ),
    terminal_matches = (
        SELECT COUNT(DISTINCT (jobs.routing_region, jobs.match_id))
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
    last_synced_at = GREATEST(tft_player_match_sync.last_synced_at, EXCLUDED.last_synced_at),
    last_match_id = CASE
        WHEN (EXCLUDED.window_end, EXCLUDED.last_synced_at) >=
             (tft_player_match_sync.window_end, tft_player_match_sync.last_synced_at)
        THEN EXCLUDED.last_match_id
        ELSE tft_player_match_sync.last_match_id
    END,
    updated_at = now();

-- name: GetTFTPlayerMatchSync :one
SELECT * FROM tft_player_match_sync
WHERE platform = @platform AND puuid = @puuid AND queue_type = @queue_type;

-- name: InsertTFTMatchDiscovery :execrows
INSERT INTO tft_match_discoveries (
    run_id, platform, routing_region, seed_puuid, match_id, cohort
) VALUES (@run_id, @platform, @routing_region, @seed_puuid, @match_id, @cohort)
ON CONFLICT DO NOTHING;

-- name: EnsureTFTRunMatchSampling :exec
INSERT INTO tft_run_match_sampling (
    run_id, target_per_region, selection_revision
) VALUES (
    @run_id, @target_per_region, @selection_revision
)
ON CONFLICT (run_id) DO NOTHING;

-- name: GetTFTRunMatchSampling :one
SELECT * FROM tft_run_match_sampling WHERE run_id = @run_id;

-- name: LockTFTRunMatchSampling :one
SELECT * FROM tft_run_match_sampling WHERE run_id = @run_id FOR UPDATE;

-- name: EnsureTFTRunPlatformSampling :exec
INSERT INTO tft_run_platform_sampling (
    run_id, platform, master_limit, diamond_per_division, salt
) VALUES (
    @run_id, @platform, @master_limit, @diamond_per_division, @salt
)
ON CONFLICT (run_id, platform) DO NOTHING;

-- name: GetTFTRunPlatformSampling :one
SELECT * FROM tft_run_platform_sampling
WHERE run_id = @run_id AND platform = @platform;

-- name: InsertTFTRunMatchCandidateSourceIfOpen :execrows
WITH sampling_gate AS MATERIALIZED (
    SELECT sampling.run_id
    FROM tft_run_match_sampling sampling
    WHERE sampling.run_id = sqlc.arg(run_id) AND sampling.phase = 'open'
    FOR KEY SHARE
), candidate AS (
    INSERT INTO tft_run_match_candidates (
        run_id, routing_region, match_id, platform, selection_key
    )
    SELECT sampling_gate.run_id, @routing_region, @match_id, @platform, @selection_key
    FROM sampling_gate
    ON CONFLICT (run_id, routing_region, match_id) DO UPDATE
    SET selection_key = EXCLUDED.selection_key,
        updated_at = now()
    RETURNING run_id, routing_region, match_id
)
INSERT INTO tft_run_match_candidate_sources (
    run_id, routing_region, match_id, platform, seed_puuid, cohort
)
SELECT candidate.run_id, candidate.routing_region, candidate.match_id,
       @platform, @seed_puuid, @cohort
FROM candidate
ON CONFLICT DO NOTHING;

-- name: GetTFTRunPlayerMatchSync :one
SELECT * FROM tft_run_player_match_sync
WHERE run_id = @run_id
  AND platform = @platform
  AND puuid = @puuid
  AND queue_type = @queue_type;

-- name: UpsertTFTRunPlayerMatchSyncIfOpen :execrows
WITH sampling_gate AS MATERIALIZED (
    SELECT sampling.run_id
    FROM tft_run_match_sampling sampling
    WHERE sampling.run_id = sqlc.arg(run_id) AND sampling.phase = 'open'
    FOR KEY SHARE
)
INSERT INTO tft_run_player_match_sync (
    run_id, platform, puuid, queue_type, window_start, window_end,
    last_synced_at, last_match_id
)
SELECT sampling_gate.run_id, @platform, @puuid, @queue_type,
       @window_start, @window_end, @last_synced_at, sqlc.narg(last_match_id)
FROM sampling_gate
ON CONFLICT (run_id, platform, puuid, queue_type) DO UPDATE
SET window_start = LEAST(tft_run_player_match_sync.window_start, EXCLUDED.window_start),
    window_end = GREATEST(tft_run_player_match_sync.window_end, EXCLUDED.window_end),
    last_synced_at = EXCLUDED.last_synced_at,
    last_match_id = EXCLUDED.last_match_id,
    updated_at = now();

-- name: SelectBalancedTFTRunMatchCandidates :exec
WITH ranked AS (
    SELECT source.run_id, source.routing_region, source.match_id,
           ROW_NUMBER() OVER (
               PARTITION BY source.routing_region
               ORDER BY source.selection_key, source.match_id
           ) <= @target_per_route::bigint AS should_select
    FROM tft_run_match_candidates source
    WHERE source.run_id = @run_id
)
UPDATE tft_run_match_candidates candidates
SET selected = ranked.should_select,
    updated_at = now()
FROM ranked
WHERE candidates.run_id = ranked.run_id
  AND candidates.routing_region = ranked.routing_region
  AND candidates.match_id = ranked.match_id;

-- name: ProjectBalancedTFTMatchDiscoveries :execrows
INSERT INTO tft_match_discoveries (
    run_id, platform, routing_region, seed_puuid, match_id, cohort
)
SELECT sources.run_id, sources.platform, sources.routing_region,
       sources.seed_puuid, sources.match_id, sources.cohort
FROM tft_run_match_candidate_sources sources
JOIN tft_run_match_candidates candidates
  ON candidates.run_id = sources.run_id
 AND candidates.routing_region = sources.routing_region
 AND candidates.match_id = sources.match_id
WHERE candidates.run_id = @run_id AND candidates.selected
ON CONFLICT DO NOTHING;

-- name: EnqueueBalancedTFTMatchJobs :execrows
INSERT INTO tft_match_jobs (routing_region, match_id, platform)
SELECT routing_region, match_id, platform
FROM tft_run_match_candidates
WHERE run_id = @run_id AND selected
ON CONFLICT (routing_region, match_id) DO NOTHING;

-- name: CommitTFTRunPlayerMatchSync :execrows
INSERT INTO tft_player_match_sync (
    platform, puuid, queue_type, window_start, window_end, last_synced_at, last_match_id
)
SELECT platform, puuid, queue_type, window_start, window_end, last_synced_at, last_match_id
FROM tft_run_player_match_sync
WHERE run_id = @run_id
ON CONFLICT (platform, puuid, queue_type) DO UPDATE
SET window_start = LEAST(tft_player_match_sync.window_start, EXCLUDED.window_start),
    window_end = GREATEST(tft_player_match_sync.window_end, EXCLUDED.window_end),
    last_synced_at = GREATEST(tft_player_match_sync.last_synced_at, EXCLUDED.last_synced_at),
    last_match_id = CASE
        WHEN (EXCLUDED.window_end, EXCLUDED.last_synced_at) >=
             (tft_player_match_sync.window_end, tft_player_match_sync.last_synced_at)
        THEN EXCLUDED.last_match_id
        ELSE tft_player_match_sync.last_match_id
    END,
    updated_at = now();

-- name: MarkTFTRunMatchSamplingFinalized :execrows
UPDATE tft_run_match_sampling
SET phase = 'finalized', finalized_at = now(), updated_at = now()
WHERE run_id = @run_id AND phase = 'open';

-- name: ListTFTRunCandidateRouteProgress :many
WITH routes AS (
    SELECT 'AMERICAS'::text AS routing_region
    UNION ALL SELECT 'ASIA'::text
    UNION ALL SELECT 'EUROPE'::text
    UNION ALL SELECT 'SEA'::text
), counts AS (
    SELECT routing_region,
           COUNT(*)::bigint AS candidate_matches,
           COUNT(*) FILTER (WHERE selected)::bigint AS selected_matches
    FROM tft_run_match_candidates
    WHERE run_id = @run_id
    GROUP BY routing_region
)
SELECT routes.routing_region,
       COALESCE(counts.candidate_matches, 0)::bigint AS candidate_matches,
       COALESCE(counts.selected_matches, 0)::bigint AS selected_matches
FROM routes
LEFT JOIN counts USING (routing_region)
ORDER BY CASE routes.routing_region
    WHEN 'AMERICAS' THEN 1 WHEN 'ASIA' THEN 2
    WHEN 'EUROPE' THEN 3 WHEN 'SEA' THEN 4 END;

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
