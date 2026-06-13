ALTER TABLE runs ADD COLUMN IF NOT EXISTS region text NOT NULL DEFAULT 'KR';

ALTER TABLE matches ADD COLUMN IF NOT EXISTS region text NOT NULL DEFAULT 'KR';
CREATE INDEX IF NOT EXISTS idx_matches_region_status ON matches(region, fetch_status);

ALTER TABLE player_rank_snapshots ADD COLUMN IF NOT EXISTS region text NOT NULL DEFAULT 'KR';

ALTER TABLE player_match_sync ADD COLUMN IF NOT EXISTS region text NOT NULL DEFAULT 'KR';
ALTER TABLE player_match_sync DROP CONSTRAINT IF EXISTS player_match_sync_pkey;
ALTER TABLE player_match_sync ADD PRIMARY KEY (puuid, region);