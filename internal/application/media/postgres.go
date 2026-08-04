package media

import (
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	pgmedia "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/media"
)

type pgMediaRepo struct{ *pgmedia.Repository }

func (r pgMediaRepo) WithTx(tx pgx.Tx) Repository {
	return pgMediaRepo{r.Repository.WithTx(tx)}
}

// NewPostgresService constructs a Service backed by PostgreSQL repositories.
// Storage and Processor are left to the caller: while no provider adapter is
// wired they fall back to the unconfigured implementations, which report
// DEPENDENCY_UNAVAILABLE instead of pretending an upload succeeded. The test
// doubles in memory.go must never be passed here.
func NewPostgresService(pool *pgxpool.Pool, cfg Config) (*Service, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	cfg.Repo = pgMediaRepo{pgmedia.NewRepository(pool)}
	return NewService(cfg)
}

// NewPostgresWorker constructs a Worker backed by PostgreSQL repositories.
func NewPostgresWorker(pool *pgxpool.Pool, cfg WorkerConfig) (*Worker, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	cfg.Repo = pgMediaRepo{pgmedia.NewRepository(pool)}
	return NewWorker(cfg)
}

var _ Repository = pgMediaRepo{}
