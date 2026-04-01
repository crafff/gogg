package storage

import (
	"context"
	"fmt"
)

func (s *Store) InitSchema(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("store is not initialized")
	}

	if _, err := s.pool.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("initialize schema: %w", err)
	}

	return nil
}

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
  end_of_game_result TEXT,
  game_creation TIMESTAMPTZ,
  game_duration BIGINT,
  game_end_time TIMESTAMPTZ,
  game_id BIGINT,
  game_mode TEXT,
  game_name TEXT,
  game_start_time TIMESTAMPTZ,
  game_type TEXT,
  game_version TEXT,
  map_id INT,
  platform_id TEXT NOT NULL DEFAULT '',
  queue_id INT,
  tournament_code TEXT,
  processed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_players_puuid ON players(puuid);
CREATE INDEX IF NOT EXISTS idx_matches_processed_at ON matches(processed_at);

CREATE TABLE IF NOT EXISTS match_teams (
  match_id TEXT NOT NULL REFERENCES matches(match_id) ON DELETE CASCADE,
  team_id INT NOT NULL,
  win BOOLEAN NOT NULL,
  objectives JSONB,  -- 存储推塔、拿龙的数据
  feats JSONB,       -- 存储 First Blood 等成就数据
  PRIMARY KEY (match_id, team_id)
);

CREATE TABLE IF NOT EXISTS bans (
  match_id TEXT NOT NULL REFERENCES matches(match_id) ON DELETE CASCADE,
  team_id INT NOT NULL,
  champion_id INT NOT NULL,
  pick_turn INT NOT NULL,
  PRIMARY KEY (match_id, team_id, pick_turn)
);

ALTER TABLE bans ADD COLUMN IF NOT EXISTS pick_turn INT NOT NULL;
CREATE INDEX IF NOT EXISTS idx_bans_match_id ON bans(match_id);

CREATE TABLE IF NOT EXISTS match_participants (
  match_id TEXT NOT NULL REFERENCES matches(match_id) ON DELETE CASCADE,
  participant_id INT NOT NULL,       -- 局内编号 (1-10)
  puuid TEXT NOT NULL,               -- 玩家唯一标识
  team_id INT NOT NULL,              -- 100 或 200
  
  -- 英雄与位置
  champion_id INT NOT NULL,
  champion_name TEXT NOT NULL,
  team_position TEXT,                -- TOP, JUNGLE, MID, BOTTOM, UTILITY
  individual_position TEXT,
  lane TEXT,
  role TEXT,
  
  -- 胜负
  win BOOLEAN NOT NULL,
  
  -- 核心战绩 (KDA)
  kills INT NOT NULL DEFAULT 0,
  deaths INT NOT NULL DEFAULT 0,
  assists INT NOT NULL DEFAULT 0,
  
  -- 经济与发育
  gold_earned INT NOT NULL DEFAULT 0,
  gold_spent INT NOT NULL DEFAULT 0,
  champ_level INT NOT NULL DEFAULT 0,
  total_minions_killed INT NOT NULL DEFAULT 0,
  neutral_minions_killed INT NOT NULL DEFAULT 0,
  
  -- 战斗数据 (伤害/承伤)
  total_damage_dealt_to_champions INT NOT NULL DEFAULT 0,
  physical_damage_dealt_to_champions INT NOT NULL DEFAULT 0,
  magic_damage_dealt_to_champions INT NOT NULL DEFAULT 0,
  true_damage_dealt_to_champions INT NOT NULL DEFAULT 0,
  total_damage_taken INT NOT NULL DEFAULT 0,
  damage_self_mitigated INT NOT NULL DEFAULT 0,
  total_heal INT NOT NULL DEFAULT 0,
  
  -- 视野
  vision_score INT NOT NULL DEFAULT 0,
  wards_placed INT NOT NULL DEFAULT 0,
  wards_killed INT NOT NULL DEFAULT 0,
  vision_wards_bought_in_game INT NOT NULL DEFAULT 0,
  
  -- 装备与技能 (出装列表)
  item0 INT, item1 INT, item2 INT, item3 INT, item4 INT, item5 INT, item6 INT,
  summoner1_id INT,
  summoner2_id INT,
  
  -- 复杂嵌套数据 (符文和所有剩余的长尾统计)
  raw_stats JSONB,   -- 存储其它未列出的 DTO 字段 (如各类 Pings, 多杀, 斗魂竞技场Augments等)
  
  PRIMARY KEY (match_id, participant_id)
);

-- 创建索引以支持快速查询某人的战绩历史和英雄偏好
CREATE INDEX IF NOT EXISTS idx_participants_puuid ON match_participants(puuid);
CREATE INDEX IF NOT EXISTS idx_participants_champion ON match_participants(champion_id);

CREATE TABLE IF NOT EXISTS match_participant_perks_ext (
  match_id TEXT NOT NULL,
  participant_id INT NOT NULL,
  
  -- 冗余过来的核心查询条件 (空间换时间)
  champion_id INT NOT NULL,
  win BOOLEAN NOT NULL,
  
  -- 符文系别
  primary_style TEXT,
  sub_style TEXT,
  
  -- 拍平的核心符文 ID
  perk_0 INT, perk_1 INT, perk_2 INT, perk_3 INT, perk_4 INT, perk_5 INT,
  stat_defense INT, stat_flex INT, stat_offense INT,
  
  -- 那些又臭又长的局内伤害/回血数值，统统丢进 JSONB (按需存储)
  perk_vars JSONB, 
  
  PRIMARY KEY (match_id, participant_id),
  FOREIGN KEY (match_id, participant_id) REFERENCES match_participants(match_id, participant_id) ON DELETE CASCADE
);

-- 为你的终极查询需求建立复合索引
CREATE INDEX IF NOT EXISTS idx_perks_champ_win ON match_participant_perks_ext(champion_id, win);
CREATE INDEX IF NOT EXISTS idx_perks_champ_keystone ON match_participant_perks_ext(champion_id, perk_0);

`
