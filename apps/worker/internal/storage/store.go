package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
	dsn  string
}

func New(ctx context.Context, dsn string, maxOpen, maxIdle int32, maxLifetimeSec int) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	if maxOpen > 0 {
		cfg.MaxConns = maxOpen
	}
	if maxIdle > 0 {
		cfg.MinConns = maxIdle
	}
	if maxLifetimeSec > 0 {
		cfg.MaxConnLifetime = time.Duration(maxLifetimeSec) * time.Second
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return &Store{Pool: pool, dsn: dsn}, nil
}

func (s *Store) Close() {
	s.Pool.Close()
}
