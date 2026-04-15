package server

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ChampionRankingItem is a single row in champion ranking response.
type ChampionRankingItem struct {
	ChampionID   int     `json:"championId"`
	ChampionName string  `json:"championName"`
	TeamPosition []string `json:"teamPosition"`		// "TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY"，如果是整体排名查询则可能是多个位置的组合，例如 "TOP/JUNGLE"
	Games        int     `json:"games"`
	Wins         int     `json:"wins"`
	Losses       int     `json:"losses"`
	WinRate      float64 `json:"winRate"`
	PickRate     float64 `json:"pickRate"`
	BanRate      float64 `json:"banRate"`
	KDA          float64 `json:"kda"`
}

// ChampionRankingQuery controls ranking filters.
type ChampionRankingQuery struct {
	QueueID  int
	GameVersion string		// 例如 "16.7", 实际比赛版本号可能是 "16.7.760.9485", 我们只关心前两段
	Position string			// "TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY"，空字符串表示不限位置
	MinGames int	        // >= 0, 如果Position为空时表示整体排名的最低游戏场次，如果Position不为空时表示该位置的最低游戏场次
	Limit    int			// -1 表示不限制
	PositionThreshold float64 // 仅在 Position 为空时使用，表示出场率的最低阈值（百分数），例如 5.0 表示至少 5% 的出场率
}

// RankingStore reads ranking data from PostgreSQL.
type RankingStore struct {
	pool *pgxpool.Pool
}

func NewRankingStore(pool *pgxpool.Pool) *RankingStore {
	return &RankingStore{pool: pool}
}


// 用于不指定位置时的整体排名查询，其逻辑与指定位置查询稍有不同
// 需要识别这个英雄有几个位置的玩法，并按照出场率的高低返回一个TeamPosition []string
// 同时会过滤出场率低于阈值的位置（< PositionThreshold - 百分数）以避免过于分散的排名结果
func (s *RankingStore) GetOverallRankings(ctx context.Context, q ChampionRankingQuery) ([]ChampionRankingItem, error) {
// SQL 语句
	const query = `
WITH filtered_matches AS (
    SELECT m.match_id
    FROM matches m
    WHERE m.queue_id = $1
      -- 处理版本号：例如传入 "16.7"，使用 LIKE 匹配 "16.7%"
      AND ($2 = '' OR m.game_version LIKE $2 || '%')
      AND m.processed_at IS NOT NULL
),
base AS (
    SELECT
        mp.match_id,
        mp.champion_id,
        mp.champion_name,
        mp.team_position,
        mp.win,
        mp.kills,
        mp.deaths,
        mp.assists
    FROM match_participants mp
    INNER JOIN filtered_matches fm ON fm.match_id = mp.match_id
    WHERE mp.champion_id > 0
),
totals AS (
    -- 计算所有玩家人次和总对局数，用于最后计算整体 PickRate 和 BanRate
    SELECT
        COUNT(*)::float8 AS total_participants,
        COUNT(DISTINCT match_id)::float8 AS total_matches
    FROM base
),
champ_agg AS (
    -- 第一层聚合：计算英雄的总场次、胜场、KDA 等基础数据
    SELECT
        b.champion_id,
        MAX(b.champion_name) AS champion_name,
        COUNT(*)::int AS games,
        SUM(CASE WHEN b.win THEN 1 ELSE 0 END)::int AS wins,
        AVG((b.kills + b.assists)::float8 / GREATEST(b.deaths, 1))::float8 AS kda
    FROM base b
    GROUP BY b.champion_id
),
pos_agg AS (
    -- 第二层聚合：计算每个英雄在不同位置的具体场次
    SELECT 
        b.champion_id, 
        b.team_position, 
        COUNT(*)::int AS pos_games
    FROM base b
    GROUP BY b.champion_id, b.team_position
),
valid_positions AS (
    -- 核心逻辑：过滤出场率大于阈值的位置，并聚合成数组
    SELECT 
        pa.champion_id,
        -- ARRAY_AGG 把符合条件的位置拼成一个 PostgreSQL 数组，按位置出场数降序排列
        ARRAY_AGG(pa.team_position ORDER BY pa.pos_games DESC) AS positions
    FROM pos_agg pa
    INNER JOIN champ_agg ca ON pa.champion_id = ca.champion_id
    -- 根据阈值过滤：例如 $3 传入 5.0，这里就是过滤位置出场率 >= 5% 的数据
    WHERE ((pa.pos_games::float8 / NULLIF(ca.games, 0)) * 100.0) >= $3
    GROUP BY pa.champion_id
),
ban_agg AS (
    SELECT
        b.champion_id,
        COUNT(DISTINCT b.match_id)::float8 AS ban_matches
    FROM bans b
    INNER JOIN filtered_matches fm ON fm.match_id = b.match_id
    GROUP BY b.champion_id
)
SELECT
    ca.champion_id,
    ca.champion_name,
    -- 防止某些极端情况下连一个符合条件的位置都没有，返回空数组而不是 NULL
    COALESCE(vp.positions, ARRAY[]::text[]) AS team_position,
    ca.games,
    ca.wins,
    (ca.games - ca.wins) AS losses,
    ROUND(((ca.wins::float8 / NULLIF(ca.games, 0)) * 100.0)::numeric, 2)::float8 AS win_rate,
    ROUND(((ca.games::float8 / NULLIF(t.total_participants, 0)) * 100.0)::numeric, 2)::float8 AS pick_rate,
    ROUND(((COALESCE(ba.ban_matches, 0.0) / NULLIF(t.total_matches, 0)) * 100.0)::numeric, 2)::float8 AS ban_rate,
    ROUND((ca.kda)::numeric, 2)::float8 AS kda
FROM champ_agg ca
CROSS JOIN totals t
LEFT JOIN valid_positions vp ON vp.champion_id = ca.champion_id
LEFT JOIN ban_agg ba ON ba.champion_id = ca.champion_id
-- $4 是 MinGames 的限制
WHERE ca.games >= $4
ORDER BY win_rate DESC, pick_rate DESC, games DESC
-- $5 是 Limit，使用 NULLIF 将 -1 转换成 NULL，在 PG 中 LIMIT NULL 表示不限制
LIMIT NULLIF($5, -1);
`

	// 执行查询
	rows, err := s.pool.Query(ctx, query, 
        q.QueueID,           // $1
        q.GameVersion,       // $2
        q.PositionThreshold, // $3
        q.MinGames,          // $4
        q.Limit,             // $5
    )
	if err != nil {
		return nil, fmt.Errorf("query overall champion rankings: %w", err)
	}
	defer rows.Close()

	// 组装数据
	// 如果 Limit 为 -1，给 slice 一个合理的初始容量
	capSize := q.Limit
	if capSize <= 0 {
		capSize = 300 
	}
	items := make([]ChampionRankingItem, 0, capSize)
    
	for rows.Next() {
		var item ChampionRankingItem
		if err := rows.Scan(
			&item.ChampionID,
			&item.ChampionName,
			&item.TeamPosition, // pgx 原生支持将 ARRAY[]::text[] 解析到 []string
			&item.Games,
			&item.Wins,
			&item.Losses,
			&item.WinRate,
			&item.PickRate,
			&item.BanRate,
			&item.KDA,
		); err != nil {
			return nil, fmt.Errorf("scan overall champion ranking row: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate overall champion ranking rows: %w", err)
	}

	return items, nil
}

// 用于指定位置的排名查询，直接按照指定位置的统计数据返回排名结果
func (s *RankingStore) GetRankingsByPosition(ctx context.Context, q ChampionRankingQuery) ([]ChampionRankingItem, error) {
// SQL 语句：比 Overall 版本简单很多，直接过滤即可
	const query = `
WITH filtered_matches AS (
    SELECT m.match_id
    FROM matches m
    WHERE m.queue_id = $1
      AND ($2 = '' OR m.game_version LIKE $2 || '%')
      AND m.processed_at IS NOT NULL
),
base AS (
    SELECT
        mp.match_id,
        mp.champion_id,
        mp.champion_name,
        mp.win,
        mp.kills,
        mp.deaths,
        mp.assists
    FROM match_participants mp
    INNER JOIN filtered_matches fm ON fm.match_id = mp.match_id
    WHERE mp.champion_id > 0
      -- 核心区别：在这里直接把位置定死
      AND mp.team_position = $3
),
totals AS (
    SELECT
        COUNT(*)::float8 AS total_participants,
        COUNT(DISTINCT match_id)::float8 AS total_matches
    FROM base
),
champ_agg AS (
    SELECT
        b.champion_id,
        MAX(b.champion_name) AS champion_name,
        COUNT(*)::int AS games,
        SUM(CASE WHEN b.win THEN 1 ELSE 0 END)::int AS wins,
        AVG((b.kills + b.assists)::float8 / GREATEST(b.deaths, 1))::float8 AS kda
    FROM base b
    GROUP BY b.champion_id
),
ban_agg AS (
    SELECT
        b.champion_id,
        COUNT(DISTINCT b.match_id)::float8 AS ban_matches
    FROM bans b
    INNER JOIN filtered_matches fm ON fm.match_id = b.match_id
    GROUP BY b.champion_id
)
SELECT
    ca.champion_id,
    ca.champion_name,
    ca.games,
    ca.wins,
    (ca.games - ca.wins) AS losses,
    ROUND(((ca.wins::float8 / NULLIF(ca.games, 0)) * 100.0)::numeric, 2)::float8 AS win_rate,
    ROUND(((ca.games::float8 / NULLIF(t.total_participants, 0)) * 100.0)::numeric, 2)::float8 AS pick_rate,
    ROUND(((COALESCE(ba.ban_matches, 0.0) / NULLIF(t.total_matches, 0)) * 100.0)::numeric, 2)::float8 AS ban_rate,
    ROUND((ca.kda)::numeric, 2)::float8 AS kda
FROM champ_agg ca
CROSS JOIN totals t
LEFT JOIN ban_agg ba ON ba.champion_id = ca.champion_id
-- $4 是 MinGames 的限制
WHERE ca.games >= $4
ORDER BY win_rate DESC, pick_rate DESC, games DESC
-- $5 是 Limit
LIMIT NULLIF($5, -1);
`

	// 执行查询，传入的参数位置必须和 SQL 里的 $1~$5 严格对应
	rows, err := s.pool.Query(ctx, query,
		q.QueueID,     // $1
		q.GameVersion, // $2
		q.Position,    // $3，例如 "JUNGLE"
		q.MinGames,    // $4
		q.Limit,       // $5
	)
	if err != nil {
		return nil, fmt.Errorf("query position champion rankings: %w", err)
	}
	defer rows.Close()

	capSize := q.Limit
	if capSize <= 0 {
		capSize = 300 // 默认预分配300个位置，避免频繁扩容
	}
	items := make([]ChampionRankingItem, 0, capSize)

	for rows.Next() {
		var item ChampionRankingItem
		if err := rows.Scan(
			&item.ChampionID,
			&item.ChampionName,
			// 注意：这里没有 Scan Position 字段，因为 SQL 里没有 SELECT 这个字段
			&item.Games,
			&item.Wins,
			&item.Losses,
			&item.WinRate,
			&item.PickRate,
			&item.BanRate,
			&item.KDA,
		); err != nil {
			return nil, fmt.Errorf("scan position champion ranking row: %w", err)
		}

		// 核心区别：在 Go 代码层面手动为其赋予指定的位置数组
		item.TeamPosition = []string{q.Position}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate position champion ranking rows: %w", err)
	}

	return items, nil
}
