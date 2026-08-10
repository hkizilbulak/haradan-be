package media

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
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

// NewPostgresJobQueue returns the durable job runtime port for the worker process.
func NewPostgresJobQueue(pool *pgxpool.Pool) (JobQueue, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	return pgMediaRepo{pgmedia.NewRepository(pool)}, nil
}

func (r pgMediaRepo) ClaimNextJob(ctx context.Context, params ClaimJobParams) (domainmedia.BackgroundJob, bool, error) {
	types := make([]string, 0, len(params.SupportedTypes))
	for _, t := range params.SupportedTypes {
		types = append(types, string(t))
	}
	return r.Repository.ClaimNextJob(ctx, params.LeaseOwner, params.Now, params.LeaseUntil, types)
}

func (r pgMediaRepo) MarkJobSucceeded(ctx context.Context, guard JobLeaseGuard, now time.Time) error {
	return r.Repository.MarkJobSucceeded(ctx, guard.JobID, guard.LeaseOwner, guard.Version, now)
}

func (r pgMediaRepo) MarkJobFailed(ctx context.Context, guard JobLeaseGuard, now time.Time, lastError string) error {
	return r.Repository.MarkJobFailed(ctx, guard.JobID, guard.LeaseOwner, guard.Version, now, lastError)
}

func (r pgMediaRepo) RetryOrDeadLetterJob(ctx context.Context, params RetryJobParams) error {
	return r.Repository.RetryOrDeadLetterJob(
		ctx,
		params.JobID, params.LeaseOwner, params.Version,
		params.Now, params.NextAvailableAt, params.LastError,
		params.AttemptCount, params.MaxAttempts,
	)
}

func (r pgMediaRepo) RecoverExpiredJobLeases(ctx context.Context, now time.Time, limit int) (int, error) {
	return r.Repository.RecoverExpiredJobLeases(ctx, now, limit)
}

var (
	_ Repository = pgMediaRepo{}
	_ JobQueue   = pgMediaRepo{}
)
