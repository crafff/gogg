ALTER TABLE player_match_sync DROP CONSTRAINT IF EXISTS player_match_sync_pkey;
ALTER TABLE player_match_sync DROP COLUMN IF EXISTS region;
ALTER TABLE player_match_sync ADD PRIMARY KEY (puuid);

ALTER TABLE player_rank_snapshots DROP COLUMN IF EXISTS region;

DROP INDEX IF EXISTS idx_matches_region_status;
ALTER TABLE matches DROP COLUMN IF EXISTS region;

ALTER TABLE runs DROP COLUMN IF EXISTS region;