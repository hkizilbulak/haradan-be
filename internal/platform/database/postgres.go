package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds PostgreSQL pool settings. DatabaseURL must never be logged.
type Config struct {
	DatabaseURL     string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
	HealthTimeout   time.Duration
}

// Postgres wraps a pgx connection pool.
type Postgres struct {
	pool *pgxpool.Pool
}

// Open creates a pool, applies limits, and verifies connectivity with Ping.
func Open(ctx context.Context, cfg Config) (*Postgres, error) {
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return nil, fmt.Errorf("database url must not be empty")
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", sanitizeErr(err))
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("open database pool: %w", sanitizeErr(err))
	}

	pingCtx := ctx
	var cancel context.CancelFunc
	if cfg.HealthTimeout > 0 {
		pingCtx, cancel = context.WithTimeout(ctx, cfg.HealthTimeout)
		defer cancel()
	}
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", sanitizeErr(err))
	}

	return &Postgres{pool: pool}, nil
}

// Ping checks database connectivity.
func (p *Postgres) Ping(ctx context.Context) error {
	if p == nil || p.pool == nil {
		return fmt.Errorf("database pool is not initialized")
	}
	if err := p.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", sanitizeErr(err))
	}
	return nil
}

// Pool returns the underlying pool for future repository wiring.
func (p *Postgres) Pool() *pgxpool.Pool {
	if p == nil {
		return nil
	}
	return p.pool
}

// Close closes the pool.
func (p *Postgres) Close() {
	if p == nil || p.pool == nil {
		return
	}
	p.pool.Close()
}

func sanitizeErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "password") ||
		strings.Contains(lower, "postgres://") ||
		strings.Contains(lower, "postgresql://") ||
		strings.Contains(lower, "user=") ||
		strings.Contains(lower, "pwd=") {
		return fmt.Errorf("database error")
	}
	return err
}
