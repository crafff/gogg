package storage

import (
	"context"
	"fmt"
)

// schemaSQL 定义了数据库的表结构和索引。
const schemaSQL = `
CREATE TABLE IF NOT EXISTS players (
  id BIGSERIAL PRIMARY KEY,
  puuid TEXT NOT NULL UNIQUE,
  game_name TEXT,
  tag_line TEXT,
  platform TEXT NOT NULL DEFAULT '',
  region TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE players ALTER COLUMN game_name DROP NOT NULL;
ALTER TABLE players ALTER COLUMN game_name DROP DEFAULT;
ALTER TABLE players ALTER COLUMN tag_line DROP NOT NULL;
ALTER TABLE players ALTER COLUMN tag_line DROP DEFAULT;

CREATE TABLE IF NOT EXISTS player_rank_current (
  player_id BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
  queue TEXT NOT NULL,
  tier TEXT NOT NULL,
  rank TEXT NOT NULL,
  league_points INT NOT NULL,
  wins INT NOT NULL,
  losses INT NOT NULL,
  veteran BOOLEAN NOT NULL,
  inactive BOOLEAN NOT NULL,
  fresh_blood BOOLEAN NOT NULL,
  hot_streak BOOLEAN NOT NULL,
  source_task_id TEXT,
  fetched_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (player_id, queue)
);

CREATE TABLE IF NOT EXISTS game_versions (
  id BIGSERIAL PRIMARY KEY,
  version TEXT NOT NULL UNIQUE,
  start_at TIMESTAMPTZ,
  end_at TIMESTAMPTZ,
  is_active BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS player_match_sync_state (
  player_id BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
  version_id BIGINT NOT NULL REFERENCES game_versions(id) ON DELETE CASCADE,
  last_checked_at TIMESTAMPTZ,
  last_match_time TIMESTAMPTZ,
  cursor TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (player_id, version_id)
);

CREATE TABLE IF NOT EXISTS matches (
  match_id TEXT PRIMARY KEY,
  platform TEXT NOT NULL DEFAULT '',
  queue_id INT,
  game_version TEXT,
  game_start_time TIMESTAMPTZ,
  game_end_time TIMESTAMPTZ,
  processed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_players_puuid ON players(puuid);
CREATE INDEX IF NOT EXISTS idx_matches_processed_at ON matches(processed_at);
`

func (s *Store) InitSchema(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("store is not initialized")
	}

	if _, err := s.pool.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("initialize schema: %w", err)
	}

	return nil
}
