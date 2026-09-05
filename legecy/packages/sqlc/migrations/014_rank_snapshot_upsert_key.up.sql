WITH ranked AS (
    SELECT id,
           row_number() OVER (
               PARTITION BY run_id, puuid, region, source, queue, tier, COALESCE(division, 'I')
               ORDER BY created_at DESC, id DESC
           ) AS rn
    FROM player_rank_snapshots
    WHERE run_id IS NOT NULL
)
DELETE FROM player_rank_snapshots prs
USING ranked r
WHERE prs.id = r.id
  AND r.rn > 1;

UPDATE player_rank_snapshots
SET division = 'I'
WHERE division IS NULL;

ALTER TABLE player_rank_snapshots
    ALTER COLUMN division SET DEFAULT 'I',
    ALTER COLUMN division SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uniq_rank_snapshots_run_player_scope
    ON player_rank_snapshots (run_id, puuid, region, source, queue, tier, division)
    WHERE run_id IS NOT NULL;
