package media

import (
	"context"
	"time"

	"github.com/google/uuid"

	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

// ClaimJobParams selects and leases the next eligible background job.
type ClaimJobParams struct {
	LeaseOwner     string
	Now            time.Time
	LeaseUntil     time.Time
	SupportedTypes []domainmedia.JobType
}

// JobLeaseGuard identifies a claimed job for optimistic ownership checks.
type JobLeaseGuard struct {
	JobID      uuid.UUID
	LeaseOwner string
	Version    int
}

// RetryJobParams reschedules a claimed job or marks it DEAD when attempts are exhausted.
// AttemptCount is the value after claim (already incremented).
type RetryJobParams struct {
	JobLeaseGuard
	Now             time.Time
	NextAvailableAt time.Time
	LastError       string
	AttemptCount    int
	MaxAttempts     int
}

// JobQueue is the durable job runtime port used by the worker process.
// It is intentionally separate from Repository so media CRUD ports stay narrow.
type JobQueue interface {
	// ClaimNextJob leases one eligible QUEUED job of a supported type, or returns ok=false.
	ClaimNextJob(ctx context.Context, params ClaimJobParams) (job domainmedia.BackgroundJob, ok bool, err error)

	// MarkJobSucceeded marks a claimed job SUCCEEDED.
	MarkJobSucceeded(ctx context.Context, guard JobLeaseGuard, now time.Time) error

	// MarkJobFailed marks a claimed job FAILED with a sanitized last_error.
	MarkJobFailed(ctx context.Context, guard JobLeaseGuard, now time.Time, lastError string) error

	// RetryOrDeadLetterJob returns a claimed job to QUEUED with backoff, or DEAD if attempts are exhausted.
	RetryOrDeadLetterJob(ctx context.Context, params RetryJobParams) error

	// RecoverExpiredJobLeases requeues or dead-letters LEASED rows whose lease has expired.
	RecoverExpiredJobLeases(ctx context.Context, now time.Time, limit int) (recovered int, err error)
}
