package server

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ChampionRankingItem is a single row in the champion ranking response.
type ChampionRankingItem struct {
	ChampionID   int      `json:"championId"`
	ChampionName string   `json:"championName"`
	TeamPosition []string `json:"teamPosition"`
	Games        int      `json:"games"`
	Wins         int      `json:"wins"`
	Losses       int      `json:"losses"`
	WinRate      float64  `json:"winRate"`
	PickRate     float64  `json:"pickRate"`
	BanRate      float64  `json:"banRate"`
	KDA          float64  `json:"kda"`
}

// ChampionRankingQuery controls ranking filters.
type ChampionRankingQuery struct {
	QueueID           int
	Version           string  // exact match on matches.version, e.g. "16.7"; "" = all
	Region            string  // e.g. "KR"; "" = all regions
	Position          string  // "TOP" / "JUNGLE" / "MIDDLE" / "BOTTOM" / "UTILITY"; "" = all
	TierGroup         string  // "master" | "master_plus" | "grandmaster" | "grandmaster_plus" | "challenger" | ""
	MinGames          int
	Limit             int     // -1 = unlimited
	PositionThreshold float64 // overall query only: minimum position share %, e.g. 5.0
}

// tierGroupToAvgTiers maps a tier group name to the avg_tier values it covers.
//
//   - challenger      → CHALLENGER
//   - grandmaster_plus → GRANDMASTER, CHALLENGER
//   - grandmaster     → GRANDMASTER
//   - master_plus     → MASTER, GRANDMASTER, CHALLENGER
//   - master          → MASTER
//   - ""              → nil (no filter)
func tierGroupToAvgTiers(tg string) []string {
	switch tg {
	case "challenger":
		return []string{"CHALLENGER"}
	case "grandmaster_plus":
		return []string{"GRANDMASTER", "CHALLENGER"}
	case "grandmaster":
		return []string{"GRANDMASTER"}
	case "master_plus":
		return []string{"MASTER", "GRANDMASTER", "CHALLENGER"}
	case "master":
		return []string{"MASTER"}
	default:
		return []string{}
	}
}

// RankingStore reads ranking data from PostgreSQL.
type RankingStore struct {
	pool *pgxpool.Pool
}

func NewRankingStore(pool *pgxpool.Pool) *RankingStore {
	return &RankingStore{pool: pool}
}

// GetRegionsWithData returns distinct regions that have completed matches.
func (s *RankingStore) GetRegionsWithData(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT region
		FROM matches
		WHERE fetch_status = 'done' AND region IS NOT NULL AND region != ''
		ORDER BY region
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var regions []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		regions = append(regions, r)
	}
	return regions, rows.Err()
}

// GetOverallRankings returns champion rankings across all positions.
// Champions with multiple viable positions are returned with all positions listed;
// positions below PositionThreshold are filtered out.
func (s *RankingStore) GetOverallRankings(ctx context.Context, q ChampionRankingQuery) ([]ChampionRankingItem, int, error) {
	const query = `
WITH filtered_matches AS (
    SELECT m.match_id
    FROM matches m
    WHERE m.queue_id = $1
      AND ($2 = '' OR m.version = $2)
      AND ($3 = '' OR m.region  = $3)
      AND m.fetch_status = 'done'
      AND (cardinality($4::text[]) = 0 OR m.avg_tier = ANY($4::text[]))
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
    SELECT
        COUNT(*)::float8                 AS total_participants,
        COUNT(DISTINCT match_id)::float8 AS total_matches
    FROM base
),
champ_agg AS (
    SELECT
        b.champion_id,
        MAX(b.champion_name)                                                      AS champion_name,
        COUNT(*)::int                                                             AS games,
        SUM(CASE WHEN b.win THEN 1 ELSE 0 END)::int                              AS wins,
        AVG((b.kills + b.assists)::float8 / GREATEST(b.deaths, 1))::float8       AS kda
    FROM base b
    GROUP BY b.champion_id
),
pos_agg AS (
    SELECT b.champion_id, b.team_position, COUNT(*)::int AS pos_games
    FROM base b
    GROUP BY b.champion_id, b.team_position
),
valid_positions AS (
    SELECT
        pa.champion_id,
        ARRAY_AGG(pa.team_position ORDER BY pa.pos_games DESC) AS positions
    FROM pos_agg pa
    INNER JOIN champ_agg ca ON pa.champion_id = ca.champion_id
    WHERE ((pa.pos_games::float8 / NULLIF(ca.games, 0)) * 100.0) >= $5
    GROUP BY pa.champion_id
),
ban_agg AS (
    SELECT b.champion_id, COUNT(DISTINCT b.match_id)::float8 AS ban_matches
    FROM match_bans b
    INNER JOIN filtered_matches fm ON fm.match_id = b.match_id
    GROUP BY b.champion_id
)
SELECT
    ca.champion_id,
    ca.champion_name,
    COALESCE(vp.positions, ARRAY[]::text[])                                                           AS team_position,
    ca.games,
    ca.wins,
    (ca.games - ca.wins)                                                                              AS losses,
    ROUND(((ca.wins::float8  / NULLIF(ca.games, 0))           * 100.0)::numeric, 2)::float8          AS win_rate,
    ROUND(((ca.games::float8 / NULLIF(t.total_matches,0))       * 100.0)::numeric, 2)::float8         AS pick_rate,
    ROUND(((COALESCE(ba.ban_matches,0) / NULLIF(t.total_matches,0)) * 100.0)::numeric, 2)::float8    AS ban_rate,
    ROUND(ca.kda::numeric, 2)::float8                                                                 AS kda,
    t.total_matches::int                                                                              AS total_matches
FROM champ_agg ca
CROSS JOIN totals t
LEFT JOIN valid_positions vp ON vp.champion_id = ca.champion_id
LEFT JOIN ban_agg ba         ON ba.champion_id = ca.champion_id
WHERE ca.games >= $6
ORDER BY win_rate DESC, pick_rate DESC, games DESC
LIMIT NULLIF($7, -1)
`
	tiers := tierGroupToAvgTiers(q.TierGroup)
	rows, err := s.pool.Query(ctx, query,
		q.QueueID,           // $1
		q.Version,           // $2
		q.Region,            // $3
		tiers,               // $4
		q.PositionThreshold, // $5
		q.MinGames,          // $6
		q.Limit,             // $7
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query overall champion rankings: %w", err)
	}
	defer rows.Close()

	items := make([]ChampionRankingItem, 0, clampCap(q.Limit))
	totalMatches := 0
	for rows.Next() {
		var item ChampionRankingItem
		if err := rows.Scan(
			&item.ChampionID, &item.ChampionName, &item.TeamPosition,
			&item.Games, &item.Wins, &item.Losses,
			&item.WinRate, &item.PickRate, &item.BanRate, &item.KDA,
			&totalMatches,
		); err != nil {
			return nil, 0, fmt.Errorf("scan overall champion ranking row: %w", err)
		}
		items = append(items, item)
	}
	return items, totalMatches, rows.Err()
}

// GetRankingsByPosition returns champion rankings for a specific position.
func (s *RankingStore) GetRankingsByPosition(ctx context.Context, q ChampionRankingQuery) ([]ChampionRankingItem, int, error) {
	const query = `
WITH filtered_matches AS (
    SELECT m.match_id
    FROM matches m
    WHERE m.queue_id = $1
      AND ($2 = '' OR m.version = $2)
      AND ($3 = '' OR m.region  = $3)
      AND m.fetch_status = 'done'
      AND (cardinality($4::text[]) = 0 OR m.avg_tier = ANY($4::text[]))
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
      AND mp.team_position = $5
),
totals AS (
    SELECT
        COUNT(*)::float8                 AS total_participants,
        COUNT(DISTINCT match_id)::float8 AS total_matches
    FROM base
),
champ_agg AS (
    SELECT
        b.champion_id,
        MAX(b.champion_name)                                                      AS champion_name,
        COUNT(*)::int                                                             AS games,
        SUM(CASE WHEN b.win THEN 1 ELSE 0 END)::int                              AS wins,
        AVG((b.kills + b.assists)::float8 / GREATEST(b.deaths, 1))::float8       AS kda
    FROM base b
    GROUP BY b.champion_id
),
ban_agg AS (
    SELECT b.champion_id, COUNT(DISTINCT b.match_id)::float8 AS ban_matches
    FROM match_bans b
    INNER JOIN filtered_matches fm ON fm.match_id = b.match_id
    GROUP BY b.champion_id
)
SELECT
    ca.champion_id,
    ca.champion_name,
    ca.games,
    ca.wins,
    (ca.games - ca.wins)                                                                              AS losses,
    ROUND(((ca.wins::float8  / NULLIF(ca.games, 0))           * 100.0)::numeric, 2)::float8          AS win_rate,
    ROUND(((ca.games::float8 / NULLIF(t.total_matches,0))       * 100.0)::numeric, 2)::float8         AS pick_rate,
    ROUND(((COALESCE(ba.ban_matches,0) / NULLIF(t.total_matches,0)) * 100.0)::numeric, 2)::float8    AS ban_rate,
    ROUND(ca.kda::numeric, 2)::float8                                                                 AS kda,
    t.total_matches::int                                                                              AS total_matches
FROM champ_agg ca
CROSS JOIN totals t
LEFT JOIN ban_agg ba ON ba.champion_id = ca.champion_id
WHERE ca.games >= $6
ORDER BY win_rate DESC, pick_rate DESC, games DESC
LIMIT NULLIF($7, -1)
`
	tiers := tierGroupToAvgTiers(q.TierGroup)
	rows, err := s.pool.Query(ctx, query,
		q.QueueID,  // $1
		q.Version,  // $2
		q.Region,   // $3
		tiers,      // $4
		q.Position, // $5
		q.MinGames, // $6
		q.Limit,    // $7
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query position champion rankings: %w", err)
	}
	defer rows.Close()

	items := make([]ChampionRankingItem, 0, clampCap(q.Limit))
	totalMatches := 0
	for rows.Next() {
		var item ChampionRankingItem
		if err := rows.Scan(
			&item.ChampionID, &item.ChampionName,
			&item.Games, &item.Wins, &item.Losses,
			&item.WinRate, &item.PickRate, &item.BanRate, &item.KDA,
			&totalMatches,
		); err != nil {
			return nil, 0, fmt.Errorf("scan position champion ranking row: %w", err)
		}
		item.TeamPosition = []string{q.Position}
		items = append(items, item)
	}
	return items, totalMatches, rows.Err()
}

func clampCap(limit int) int {
	if limit > 0 {
		return limit
	}
	return 300
}
