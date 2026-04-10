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
	Position string
	MinGames int
	Limit    int
}

// RankingStore reads ranking data from PostgreSQL.
type RankingStore struct {
	pool *pgxpool.Pool
}

func NewRankingStore(pool *pgxpool.Pool) *RankingStore {
	return &RankingStore{pool: pool}
}

func (s *RankingStore) GetChampionRankings(ctx context.Context, q ChampionRankingQuery) ([]ChampionRankingItem, error) {
	const query = `
WITH filtered_matches AS (
    SELECT m.match_id
    FROM matches m
    WHERE m.queue_id = $1
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
    WHERE ($2 = '' OR mp.team_position = $2)
      AND mp.champion_id > 0
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
WHERE ca.games >= $3
ORDER BY win_rate DESC, pick_rate DESC, games DESC
LIMIT $4;
`

	rows, err := s.pool.Query(ctx, query, q.QueueID, q.Position, q.MinGames, q.Limit)
	if err != nil {
		return nil, fmt.Errorf("query champion rankings: %w", err)
	}
	defer rows.Close()

	items := make([]ChampionRankingItem, 0, q.Limit)
	for rows.Next() {
		var item ChampionRankingItem
		if err := rows.Scan(
			&item.ChampionID,
			&item.ChampionName,
			&item.Games,
			&item.Wins,
			&item.Losses,
			&item.WinRate,
			&item.PickRate,
			&item.BanRate,
			&item.KDA,
		); err != nil {
			return nil, fmt.Errorf("scan champion ranking row: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate champion ranking rows: %w", err)
	}

	return items, nil
}
