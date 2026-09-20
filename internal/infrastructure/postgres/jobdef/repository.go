package jobdef

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	appjobadmin "github.com/hkizilbulak/haradan-be/internal/application/jobadmin"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainjobdef "github.com/hkizilbulak/haradan-be/internal/domain/jobdef"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

const (
	jobNotFoundMessage  = "Görev tanımı bulunamadı."
	staleVersionMessage = "Görev tanımı başka bir işlem tarafından güncellendi."
)

const definitionColumns = `id, job_key, name, description, job_type, cron_expression, is_active,
timeout_seconds, default_payload, supports_reference_date, version, created_at, updated_at`

const historyColumns = `j.id, j.job_definition_id, j.job_type, COALESCE(r.status, j.status) AS status, j.execution_type, j.triggered_by_user_id,
j.reference_date, j.attempt_count, j.max_attempts, j.available_at, COALESCE(r.started_at, j.started_at) AS started_at, COALESCE(r.completed_at, j.completed_at) AS completed_at, COALESCE(r.last_error_summary, j.last_error) AS last_error,
j.created_at, j.updated_at, COALESCE(r.total_count, j.processed_count, 0) AS processed_count, j.tjk_sync_run_id`

// Querier is implemented by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository persists job definitions and linked background job history.
type Repository struct {
	pool *pgxpool.Pool
	db   Querier
}

// NewRepository constructs a job definition repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, db: pool}
}

// WithTx returns a repository scoped to a transaction.
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{pool: r.pool, db: tx}
}

// ListDefinitions returns all job definitions ordered by job_key.
func (r *Repository) ListDefinitions(ctx context.Context) ([]domainjobdef.JobDefinition, error) {
	q := `SELECT ` + definitionColumns + ` FROM hrd_job_definitions ORDER BY job_key ASC`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list job definitions: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()
	out := make([]domainjobdef.JobDefinition, 0)
	for rows.Next() {
		def, err := scanDefinition(rows)
		if err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan job definition: %w", pg.SanitizeErr(err)))
		}
		out = append(out, def)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate job definitions: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// GetDefinition loads one definition by id.
func (r *Repository) GetDefinition(ctx context.Context, id uuid.UUID) (domainjobdef.JobDefinition, error) {
	q := `SELECT ` + definitionColumns + ` FROM hrd_job_definitions WHERE id = $1`
	def, err := scanDefinition(r.db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainjobdef.JobDefinition{}, apperr.NotFound(jobNotFoundMessage)
	}
	if err != nil {
		return domainjobdef.JobDefinition{}, apperr.Internal(fmt.Errorf("get job definition: %w", pg.SanitizeErr(err)))
	}
	return def, nil
}

// UpdateDefinitionOptimistic updates mutable fields with version check.
func (r *Repository) UpdateDefinitionOptimistic(
	ctx context.Context,
	def domainjobdef.JobDefinition,
	expectedVersion int,
) (domainjobdef.JobDefinition, error) {
	const q = `
UPDATE hrd_job_definitions
SET cron_expression = $3,
    is_active = $4,
    timeout_seconds = $5,
    version = version + 1,
    updated_at = $6
WHERE id = $1 AND version = $2
RETURNING ` + definitionColumns
	row := r.db.QueryRow(ctx, q, def.ID, expectedVersion, def.CronExpression, def.IsActive, def.TimeoutSeconds, def.UpdatedAt)
	updated, err := scanDefinition(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainjobdef.JobDefinition{}, apperr.Conflict(staleVersionMessage)
	}
	if err != nil {
		return domainjobdef.JobDefinition{}, apperr.Internal(fmt.Errorf("update job definition: %w", pg.SanitizeErr(err)))
	}
	return updated, nil
}

// ListHistory returns durable execution history for one definition with cursor pagination.
func (r *Repository) ListHistory(
	ctx context.Context,
	definitionID uuid.UUID,
	f appjobadmin.HistoryFilter,
) ([]domainjobdef.JobExecution, error) {
	q := `
SELECT ` + historyColumns + `
FROM hrd_background_jobs j
LEFT JOIN hrd_tjk_sync_runs r ON j.tjk_sync_run_id = r.id
WHERE j.job_definition_id = $1
  AND (
    $2::timestamptz IS NULL
    OR j.created_at < $2
    OR (j.created_at = $2 AND j.id < $3)
  )
ORDER BY j.created_at DESC, j.id DESC
LIMIT $4`
	var afterAt any
	var afterID any
	if f.AfterCreatedAt != nil && f.AfterID != nil {
		afterAt = *f.AfterCreatedAt
		afterID = *f.AfterID
	}
	limit := f.Limit
	if limit < 1 {
		limit = 21
	}
	rows, err := r.db.Query(ctx, q, definitionID, afterAt, afterID, limit)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list job history: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()
	out := make([]domainjobdef.JobExecution, 0)
	for rows.Next() {
		row, err := scanHistory(rows)
		if err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan job history: %w", pg.SanitizeErr(err)))
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate job history: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// ListLastRuns returns the newest background job per definition in one query.
func (r *Repository) ListLastRuns(
	ctx context.Context,
	definitionIDs []uuid.UUID,
) (map[uuid.UUID]domainjobdef.LastRunSummary, error) {
	out := make(map[uuid.UUID]domainjobdef.LastRunSummary)
	if len(definitionIDs) == 0 {
		return out, nil
	}
	const q = `
SELECT DISTINCT ON (j.job_definition_id)
       j.job_definition_id,
       COALESCE(r.started_at, j.started_at, r.created_at, j.created_at) AS last_run_at,
       COALESCE(r.status, j.status) AS status,
       CASE
         WHEN COALESCE(r.status, j.status) IN ('QUEUED', 'RUNNING', 'LEASED') THEN NULL
         WHEN COALESCE(r.completed_at, j.completed_at) IS NOT NULL AND COALESCE(r.started_at, j.started_at) IS NOT NULL
         THEN GREATEST(0, (EXTRACT(EPOCH FROM (COALESCE(r.completed_at, j.completed_at) - COALESCE(r.started_at, j.started_at))) * 1000)::bigint)
         ELSE NULL
       END AS last_duration_ms
FROM hrd_background_jobs j
LEFT JOIN hrd_tjk_sync_runs r ON j.tjk_sync_run_id = r.id
WHERE j.job_definition_id = ANY($1)
ORDER BY j.job_definition_id, (CASE WHEN r.status IN ('QUEUED', 'RUNNING') OR j.status IN ('QUEUED', 'LEASED') THEN 0 ELSE 1 END), j.created_at DESC, j.id DESC`
	rows, err := r.db.Query(ctx, q, definitionIDs)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list job last runs: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()
	for rows.Next() {
		var (
			summary    domainjobdef.LastRunSummary
			durationMs *int64
		)
		if err := rows.Scan(&summary.DefinitionID, &summary.LastRunAt, &summary.LastStatus, &durationMs); err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan job last run: %w", pg.SanitizeErr(err)))
		}
		if durationMs != nil {
			ms := int(*durationMs)
			summary.LastDurationMs = &ms
		}
		out[summary.DefinitionID] = summary
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate job last runs: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// Enqueue inserts a durable background job (and TJK sync run when needed).
func (r *Repository) Enqueue(ctx context.Context, req appjobadmin.EnqueueRequest) (appjobadmin.EnqueueResult, error) {
	if r.pool == nil {
		return appjobadmin.EnqueueResult{}, apperr.Internal(errors.New("jobdef repository has no pool"))
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return appjobadmin.EnqueueResult{}, apperr.Internal(fmt.Errorf("begin job enqueue tx: %w", pg.SanitizeErr(err)))
	}
	defer func() { _ = tx.Rollback(ctx) }()

	repo := r.WithTx(tx)
	out, err := repo.enqueueTx(ctx, req)
	if err != nil {
		return appjobadmin.EnqueueResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return appjobadmin.EnqueueResult{}, apperr.Internal(fmt.Errorf("commit job enqueue: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

func (r *Repository) enqueueTx(ctx context.Context, req appjobadmin.EnqueueRequest) (appjobadmin.EnqueueResult, error) {
	queueType, ok := domainjobdef.QueueJobType(req.Definition.JobType)
	if !ok {
		return appjobadmin.EnqueueResult{}, apperr.Validation("Geçersiz görev tipi.")
	}
	payload := req.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	jobID := uuid.New()
	defID := req.Definition.ID
	var tjkRunID *uuid.UUID
	var tjkArg any

	if req.Definition.JobType == domainjobdef.JobTypeTJKSync {
		runID := uuid.New()
		triggerKind := "SCHEDULED"
		if req.ExecutionType == domainjobdef.ExecutionTypeManual {
			triggerKind = "MANUAL"
		}
		checkpoint := []byte(`{"page":0}`)
		payload = json.RawMessage(`{"page":0}`)
		_, err := r.db.Exec(ctx, `
INSERT INTO hrd_tjk_sync_runs (
  id, mode, status, source_adapter, scope_key, checkpoint, trigger_kind, created_by_user_id,
  cancel_requested_at, cancelled_at, started_at, completed_at,
  total_count, created_count, updated_count, unchanged_count, skipped_count, failed_count, conflict_count,
  last_error_summary, version, created_at, updated_at
) VALUES (
  $1,'FULL','QUEUED','TJK_HTTP','HORSES',$2::jsonb,$3,$4,
  NULL,NULL,NULL,NULL,
  0,0,0,0,0,0,0,
  NULL,1,$5,$5
)`, runID, checkpoint, triggerKind, req.TriggeredByUserID, req.Now)
		if err != nil {
			return appjobadmin.EnqueueResult{}, apperr.Internal(fmt.Errorf("create TJK run for job definition: %w", pg.SanitizeErr(err)))
		}
		tjkRunID = &runID
		tjkArg = runID
	}

	dedup := req.DeduplicationKey
	maxAttempts := 3
	if req.Definition.JobType == domainjobdef.JobTypeMediaReconcile {
		maxAttempts = 10
	}
	_, err := r.db.Exec(ctx, `
INSERT INTO hrd_background_jobs (
  id, job_type, status, payload, tjk_sync_run_id, deduplication_key,
  attempt_count, max_attempts, available_at, leased_until, lease_owner, last_error,
  cancel_requested_at, version, created_at, updated_at, completed_at,
  execution_type, triggered_by_user_id, reference_date, job_definition_id, started_at
) VALUES (
  $1,$2,'QUEUED',$3::jsonb,$4,$5,
  0,$6,$7,NULL,NULL,NULL,
  NULL,1,$8,$8,NULL,
  $9,$10,$11,$12,NULL
)`,
		jobID, queueType, payload, tjkArg, dedup,
		maxAttempts, req.AvailableAt, req.Now,
		string(req.ExecutionType), req.TriggeredByUserID, req.ReferenceDate, defID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			existing, findErr := r.findJobIDByDedup(ctx, dedup)
			if findErr == nil {
				return appjobadmin.EnqueueResult{BackgroundJobID: existing, AlreadyExists: true}, nil
			}
			return appjobadmin.EnqueueResult{AlreadyExists: true}, nil
		}
		return appjobadmin.EnqueueResult{}, apperr.Internal(fmt.Errorf("enqueue job definition run: %w", pg.SanitizeErr(err)))
	}
	return appjobadmin.EnqueueResult{BackgroundJobID: jobID, TJKSyncRunID: tjkRunID}, nil
}

func (r *Repository) findJobIDByDedup(ctx context.Context, key string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `SELECT id FROM hrd_background_jobs WHERE deduplication_key = $1`, key).Scan(&id)
	return id, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDefinition(row rowScanner) (domainjobdef.JobDefinition, error) {
	var (
		def     domainjobdef.JobDefinition
		jobType string
		payload []byte
	)
	err := row.Scan(
		&def.ID, &def.JobKey, &def.Name, &def.Description, &jobType, &def.CronExpression,
		&def.IsActive, &def.TimeoutSeconds, &payload, &def.SupportsReferenceDate,
		&def.Version, &def.CreatedAt, &def.UpdatedAt,
	)
	if err != nil {
		return domainjobdef.JobDefinition{}, err
	}
	def.JobType = domainjobdef.JobType(jobType)
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}
	def.DefaultPayload = payload
	return def, nil
}

func scanHistory(row rowScanner) (domainjobdef.JobExecution, error) {
	var (
		exec          domainjobdef.JobExecution
		executionType *string
	)
	err := row.Scan(
		&exec.ID, &exec.JobDefinitionID, &exec.BackgroundJobType, &exec.Status, &executionType,
		&exec.TriggeredByUserID, &exec.ReferenceDate, &exec.AttemptCount, &exec.MaxAttempts,
		&exec.AvailableAt, &exec.StartedAt, &exec.CompletedAt, &exec.LastError,
		&exec.CreatedAt, &exec.UpdatedAt, &exec.ProcessedCount, &exec.TJKSyncRunID,
	)
	if err != nil {
		return domainjobdef.JobExecution{}, err
	}
	if executionType != nil {
		t := domainjobdef.ExecutionType(*executionType)
		exec.ExecutionType = &t
	}
	return exec, nil
}

// CancelActiveJob cancels any QUEUED or LEASED background jobs (and linked TJK sync runs) for definition.
func (r *Repository) CancelActiveJob(ctx context.Context, jobID uuid.UUID, now time.Time) error {
	// 1. Cancel linked TJK sync runs if any
	_, err := r.db.Exec(ctx, `
UPDATE hrd_tjk_sync_runs SET
  status = 'CANCELLED',
  cancel_requested_at = COALESCE(cancel_requested_at, $2),
  cancelled_at = $2,
  completed_at = COALESCE(completed_at, $2),
  version = version + 1,
  updated_at = $2
WHERE (
  id IN (
    SELECT tjk_sync_run_id FROM hrd_background_jobs
    WHERE job_definition_id = $1 AND tjk_sync_run_id IS NOT NULL
  )
  OR id IN (
    SELECT id FROM hrd_tjk_sync_runs WHERE status IN ('QUEUED', 'RUNNING')
  )
) AND status IN ('QUEUED', 'RUNNING')`, jobID, now)
	if err != nil {
		return apperr.Internal(fmt.Errorf("cancel linked tjk runs: %w", pg.SanitizeErr(err)))
	}

	// 2. Cancel active background jobs (including batches)
	_, err = r.db.Exec(ctx, `
UPDATE hrd_background_jobs SET
  status = 'CANCELLED',
  cancel_requested_at = COALESCE(cancel_requested_at, $2),
  completed_at = COALESCE(completed_at, $2),
  lease_owner = NULL,
  leased_until = NULL,
  version = version + 1,
  updated_at = $2
WHERE (job_definition_id = $1 OR tjk_sync_run_id IN (
  SELECT tjk_sync_run_id FROM hrd_background_jobs WHERE job_definition_id = $1 AND tjk_sync_run_id IS NOT NULL
)) AND status IN ('QUEUED', 'LEASED')`, jobID, now)
	if err != nil {
		return apperr.Internal(fmt.Errorf("cancel background jobs: %w", pg.SanitizeErr(err)))
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
