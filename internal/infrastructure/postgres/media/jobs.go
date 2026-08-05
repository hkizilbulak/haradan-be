package media

// Durable media jobs live in hrd_background_jobs. They are written through the
// same Repository, and therefore the same transaction, as the asset rows they
// belong to: an asset is never left UPLOADED without its validate job.

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

const jobColumns = `id, job_type, status, payload, deduplication_key, attempt_count, max_attempts,
available_at, leased_until, lease_owner, last_error, cancel_requested_at, version, created_at,
updated_at, completed_at`

const jobColumnsQualified = `j.id, j.job_type, j.status, j.payload, j.deduplication_key, j.attempt_count, j.max_attempts,
j.available_at, j.leased_until, j.lease_owner, j.last_error, j.cancel_requested_at, j.version, j.created_at,
j.updated_at, j.completed_at`

const jobNotFoundMessage = "İş kaydı bulunamadı."

// EnqueueJob inserts a durable job. tjk_sync_run_id is always NULL because the
// table CHECK reserves it for TJK_SYNC_BATCH. A duplicate deduplication key is
// reported as a conflict so the caller can treat it as "already enqueued".
func (r *Repository) EnqueueJob(ctx context.Context, job domainmedia.BackgroundJob) error {
	const q = `
INSERT INTO hrd_background_jobs (
  id, job_type, status, payload, tjk_sync_run_id, deduplication_key, attempt_count, max_attempts,
  available_at, leased_until, lease_owner, last_error, cancel_requested_at, version,
  created_at, updated_at, completed_at
) VALUES (
  $1,$2,$3,$4::jsonb,NULL,$5,$6,$7,$8,NULL,NULL,NULL,NULL,$9,$10,$11,NULL
)`
	_, err := r.db.Exec(ctx, q,
		job.ID, string(job.JobType), string(job.Status), metadataOrEmpty(job.Payload),
		job.DeduplicationKey, job.AttemptCount, job.MaxAttempts, job.AvailableAt,
		versionOrOne(job.Version), job.CreatedAt, job.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict("İş kaydı zaten kuyruğa alındı.")
		}
		return apperr.Internal(fmt.Errorf("enqueue media job: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// FindJobByDedupKey returns the job carrying a deduplication key.
func (r *Repository) FindJobByDedupKey(ctx context.Context, key string) (domainmedia.BackgroundJob, error) {
	const q = `SELECT ` + jobColumns + ` FROM hrd_background_jobs WHERE deduplication_key = $1`

	job, err := scanJob(r.db.QueryRow(ctx, q, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainmedia.BackgroundJob{}, apperr.NotFound(jobNotFoundMessage)
	}
	if err != nil {
		return domainmedia.BackgroundJob{}, apperr.Internal(
			fmt.Errorf("find media job by dedup key: %w", pg.SanitizeErr(err)),
		)
	}
	return job, nil
}

func scanJob(row rowScanner) (domainmedia.BackgroundJob, error) {
	var (
		job     domainmedia.BackgroundJob
		jobType string
		status  string
		payload []byte
	)
	if err := row.Scan(
		&job.ID, &jobType, &status, &payload, &job.DeduplicationKey, &job.AttemptCount,
		&job.MaxAttempts, &job.AvailableAt, &job.LeasedUntil, &job.LeaseOwner, &job.LastError,
		&job.CancelRequestedAt, &job.Version, &job.CreatedAt, &job.UpdatedAt, &job.CompletedAt,
	); err != nil {
		return domainmedia.BackgroundJob{}, err
	}
	job.JobType = domainmedia.JobType(jobType)
	job.Status = domainmedia.JobStatus(status)
	job.Payload = metadataOrEmpty(payload)
	return job, nil
}

func versionOrOne(version int) int {
	if version < 1 {
		return 1
	}
	return version
}
