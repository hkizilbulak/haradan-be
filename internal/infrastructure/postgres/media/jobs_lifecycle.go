package media

// Durable job claim/lease lifecycle for hrd_background_jobs.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

const (
	jobLeaseLostMessage    = "İş kirası geçersiz veya süresi dolmuş."
	jobLeaseExpiredMessage = "İş kirası süresi doldu."
	defaultRecoveryBatch   = 100
)

// ClaimNextJob leases one eligible QUEUED job using FOR UPDATE SKIP LOCKED.
// supportedTypes are exact job_type values (parameterized; never concatenated).
func (r *Repository) ClaimNextJob(
	ctx context.Context,
	leaseOwner string,
	now, leaseUntil time.Time,
	supportedTypes []string,
) (domainmedia.BackgroundJob, bool, error) {
	if len(supportedTypes) == 0 {
		return domainmedia.BackgroundJob{}, false, apperr.Internal(errors.New("claim requires supported job types"))
	}
	if leaseOwner == "" {
		return domainmedia.BackgroundJob{}, false, apperr.Internal(errors.New("claim requires lease owner"))
	}
	if !leaseUntil.After(now) {
		return domainmedia.BackgroundJob{}, false, apperr.Internal(errors.New("lease until must be after now"))
	}

	const q = `
WITH cte AS (
  SELECT id
  FROM hrd_background_jobs
  WHERE status = 'QUEUED'
    AND available_at <= $1
    AND attempt_count < max_attempts
    AND cancel_requested_at IS NULL
    AND job_type = ANY($2::varchar[])
  ORDER BY available_at ASC, id ASC
  FOR UPDATE SKIP LOCKED
  LIMIT 1
)
UPDATE hrd_background_jobs j
SET
  status = 'LEASED',
  lease_owner = $3,
  leased_until = $4,
  attempt_count = j.attempt_count + 1,
  version = j.version + 1,
  updated_at = $1,
  last_error = NULL
FROM cte
WHERE j.id = cte.id
RETURNING ` + jobColumnsQualified

	job, err := scanJob(r.db.QueryRow(ctx, q, now, supportedTypes, leaseOwner, leaseUntil))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainmedia.BackgroundJob{}, false, nil
	}
	if err != nil {
		return domainmedia.BackgroundJob{}, false, apperr.Internal(
			fmt.Errorf("claim media job: %w", pg.SanitizeErr(err)),
		)
	}
	return job, true, nil
}

// MarkJobSucceeded marks a claimed job as SUCCEEDED.
func (r *Repository) MarkJobSucceeded(
	ctx context.Context,
	jobID uuid.UUID,
	leaseOwner string,
	version int,
	now time.Time,
) error {
	const q = `
UPDATE hrd_background_jobs
SET
  status = 'SUCCEEDED',
  completed_at = $1,
  leased_until = NULL,
  lease_owner = NULL,
  last_error = NULL,
  version = version + 1,
  updated_at = $1
WHERE id = $2
  AND status = 'LEASED'
  AND lease_owner = $3
  AND version = $4`

	tag, err := r.db.Exec(ctx, q, now, jobID, leaseOwner, version)
	if err != nil {
		return apperr.Internal(fmt.Errorf("mark job succeeded: %w", pg.SanitizeErr(err)))
	}
	if tag.RowsAffected() == 0 {
		return apperr.InvalidState(jobLeaseLostMessage)
	}
	return nil
}

// MarkJobFailed marks a claimed job as FAILED.
func (r *Repository) MarkJobFailed(
	ctx context.Context,
	jobID uuid.UUID,
	leaseOwner string,
	version int,
	now time.Time,
	lastError string,
) error {
	const q = `
UPDATE hrd_background_jobs
SET
  status = 'FAILED',
  completed_at = $1,
  leased_until = NULL,
  lease_owner = NULL,
  last_error = $2,
  version = version + 1,
  updated_at = $1
WHERE id = $3
  AND status = 'LEASED'
  AND lease_owner = $4
  AND version = $5`

	tag, err := r.db.Exec(ctx, q, now, nullIfEmpty(lastError), jobID, leaseOwner, version)
	if err != nil {
		return apperr.Internal(fmt.Errorf("mark job failed: %w", pg.SanitizeErr(err)))
	}
	if tag.RowsAffected() == 0 {
		return apperr.InvalidState(jobLeaseLostMessage)
	}
	return nil
}

// RetryOrDeadLetterJob returns a claimed job to QUEUED or marks it DEAD.
// attemptCount is the value after claim (already incremented).
func (r *Repository) RetryOrDeadLetterJob(
	ctx context.Context,
	jobID uuid.UUID,
	leaseOwner string,
	version int,
	now, nextAvailableAt time.Time,
	lastError string,
	attemptCount, maxAttempts int,
) error {
	if attemptCount >= maxAttempts {
		const q = `
UPDATE hrd_background_jobs
SET
  status = 'DEAD',
  completed_at = $1,
  leased_until = NULL,
  lease_owner = NULL,
  last_error = $2,
  version = version + 1,
  updated_at = $1
WHERE id = $3
  AND status = 'LEASED'
  AND lease_owner = $4
  AND version = $5`
		tag, err := r.db.Exec(ctx, q, now, nullIfEmpty(lastError), jobID, leaseOwner, version)
		if err != nil {
			return apperr.Internal(fmt.Errorf("dead-letter job: %w", pg.SanitizeErr(err)))
		}
		if tag.RowsAffected() == 0 {
			return apperr.InvalidState(jobLeaseLostMessage)
		}
		return nil
	}

	const q = `
UPDATE hrd_background_jobs
SET
  status = 'QUEUED',
  available_at = $1,
  completed_at = NULL,
  leased_until = NULL,
  lease_owner = NULL,
  last_error = $2,
  version = version + 1,
  updated_at = $3
WHERE id = $4
  AND status = 'LEASED'
  AND lease_owner = $5
  AND version = $6`
	tag, err := r.db.Exec(ctx, q, nextAvailableAt, nullIfEmpty(lastError), now, jobID, leaseOwner, version)
	if err != nil {
		return apperr.Internal(fmt.Errorf("retry job: %w", pg.SanitizeErr(err)))
	}
	if tag.RowsAffected() == 0 {
		return apperr.InvalidState(jobLeaseLostMessage)
	}
	return nil
}

// RecoverExpiredJobLeases requeues or dead-letters expired LEASED jobs.
func (r *Repository) RecoverExpiredJobLeases(ctx context.Context, now time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = defaultRecoveryBatch
	}
	const q = `
WITH expired AS (
  SELECT id
  FROM hrd_background_jobs
  WHERE status = 'LEASED'
    AND leased_until <= $1
  ORDER BY leased_until ASC, id ASC
  FOR UPDATE SKIP LOCKED
  LIMIT $2
)
UPDATE hrd_background_jobs j
SET
  status = CASE
    WHEN j.attempt_count >= j.max_attempts THEN 'DEAD'
    ELSE 'QUEUED'
  END,
  completed_at = CASE
    WHEN j.attempt_count >= j.max_attempts THEN $1
    ELSE NULL
  END,
  available_at = CASE
    WHEN j.attempt_count >= j.max_attempts THEN j.available_at
    ELSE $1
  END,
  lease_owner = NULL,
  leased_until = NULL,
  last_error = CASE
    WHEN j.attempt_count >= j.max_attempts THEN COALESCE(j.last_error, $3)
    ELSE j.last_error
  END,
  version = j.version + 1,
  updated_at = $1
FROM expired
WHERE j.id = expired.id`

	tag, err := r.db.Exec(ctx, q, now, limit, jobLeaseExpiredMessage)
	if err != nil {
		return 0, apperr.Internal(fmt.Errorf("recover expired job leases: %w", pg.SanitizeErr(err)))
	}
	return int(tag.RowsAffected()), nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
