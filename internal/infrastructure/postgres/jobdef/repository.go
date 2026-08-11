package jobdef

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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

const historyColumns = `id, job_definition_id, job_type, status, execution_type, triggered_by_user_id,
reference_date, attempt_count, max_attempts, available_at, started_at, completed_at, last_error,
created_at, updated_at`

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
	out, err := scanDefinition(r.db.QueryRow(ctx, q,
		def.ID, expectedVersion, def.CronExpression, def.IsActive, def.TimeoutSeconds, def.UpdatedAt,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		_, getErr := r.GetDefinition(ctx, def.ID)
		if getErr != nil {
			return domainjobdef.JobDefinition{}, getErr
		}
		return domainjobdef.JobDefinition{}, apperr.StaleVersion(staleVersionMessage)
	}
	if err != nil {
		return domainjobdef.JobDefinition{}, apperr.Internal(fmt.Errorf("update job definition: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// ListHistory returns execution history for a definition (newest first).
func (r *Repository) ListHistory(
	ctx context.Context,
	definitionID uuid.UUID,
	f appjobadmin.HistoryFilter,
) ([]domainjobdef.JobExecution, error) {
	q := `
SELECT ` + historyColumns + `
FROM hrd_background_jobs
WHERE job_definition_id = $1
  AND (
    $2::timestamptz IS NULL
    OR created_at < $2
    OR (created_at = $2 AND id < $3)
  )
ORDER BY created_at DESC, id DESC
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
SELECT DISTINCT ON (job_definition_id)
       job_definition_id,
       COALESCE(started_at, created_at) AS last_run_at,
       status,
       CASE
         WHEN started_at IS NOT NULL AND completed_at IS NOT NULL
         THEN GREATEST(0, (EXTRACT(EPOCH FROM (completed_at - started_at)) * 1000)::bigint)
         ELSE NULL
       END AS last_duration_ms
FROM hrd_background_jobs
WHERE job_definition_id = ANY($1)
ORDER BY job_definition_id, created_at DESC, id DESC`
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
		&exec.CreatedAt, &exec.UpdatedAt,
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

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
