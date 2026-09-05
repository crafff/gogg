DROP INDEX IF EXISTS uniq_rank_snapshots_run_player_scope;

ALTER TABLE player_rank_snapshots
    ALTER COLUMN division DROP NOT NULL,
    ALTER COLUMN division DROP DEFAULT;
