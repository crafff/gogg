package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config 定义了数据库连接的配置参数。
type Config struct {
	DSN             string
	MaxOpenConns    int32
	MaxIdleConns    int32
	ConnMaxLifetime time.Duration
}

// Store 是数据库访问的封装，提供了对外的接口方法。
type Store struct {
	pool *pgxpool.Pool
}

// NewStore 根据配置创建一个新的 Store 实例。
func NewStore(ctx context.Context, cfg Config) (*Store, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("database dsn is required")
	}

	parsed, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse database dsn: %w", err)
	}

	if cfg.MaxOpenConns > 0 {
		parsed.MaxConns = cfg.MaxOpenConns
	}
	if cfg.MaxIdleConns > 0 {
		parsed.MinConns = cfg.MaxIdleConns
	}
	if cfg.ConnMaxLifetime > 0 {
		parsed.MaxConnLifetime = cfg.ConnMaxLifetime
	}

	pool, err := pgxpool.NewWithConfig(ctx, parsed)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Close 关闭数据库连接池。
func (s *Store) Close() {
	if s == nil || s.pool == nil {
		return
	}
	s.pool.Close()
}
