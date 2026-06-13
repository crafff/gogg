package server

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type VersionItem struct {
	Version   string    `json:"version"`
	StartTime time.Time `json:"startTime"`
}

type VersionStore struct {
	pool *pgxpool.Pool
}

func NewVersionStore(pool *pgxpool.Pool) *VersionStore {
	return &VersionStore{pool: pool}
}

// GetVersionsWithData returns distinct versions that have completed matches, newest first.
func (s *VersionStore) GetVersionsWithData(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT version
		FROM matches
		WHERE fetch_status = 'done' AND version IS NOT NULL AND version != ''
		ORDER BY version DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var versions []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

func (s *VersionStore) GetLatestVersion() (string, error) {
	const query = `
SELECT version
FROM game_versions
WHERE is_latest = TRUE
ORDER BY patch_start_at DESC
LIMIT 1;
`
	var version string
	err := s.pool.QueryRow(context.Background(), query).Scan(&version)
	if err != nil {
		return "", err
	}
	return version, nil
}