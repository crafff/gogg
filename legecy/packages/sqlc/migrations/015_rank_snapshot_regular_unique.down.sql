DROP INDEX IF EXISTS uniq_rank_snapshots_run_player_scope;

CREATE UNIQUE INDEX IF NOT EXISTS uniq_rank_snapshots_run_player_scope
    ON player_rank_snapshots (run_id, puuid, region, source, queue, tier, division)
    WHERE run_id IS NOT NULL;
