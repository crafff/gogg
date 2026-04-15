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

func (s *VersionStore) GetLatestVersion() (string, error) {
	const query = `
SELECT version
FROM game_versions
WHERE is_active = TRUE
ORDER BY start_at DESC
LIMIT 1;
`
	var version string
	err := s.pool.QueryRow(context.Background(), query).Scan(&version)
	if err != nil {
		return "", err
	}
	return version, nil
}