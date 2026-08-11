package tjk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domain "github.com/hkizilbulak/haradan-be/internal/domain/tjk"
	"github.com/hkizilbulak/haradan-be/internal/platform/textnorm"
)

type DB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type Repository struct {
	pool *pgxpool.Pool
	db   DB
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool, db: pool} }

func (r *Repository) WithTx(tx pgx.Tx) *Repository { return &Repository{pool: r.pool, db: tx} }

const runCols = `id,mode,status,source_adapter,scope_key,checkpoint,trigger_kind,created_by_user_id,cancel_requested_at,cancelled_at,started_at,completed_at,total_count,created_count,updated_count,unchanged_count,skipped_count,failed_count,conflict_count,last_error_summary,version,created_at,updated_at`

func (r *Repository) CreateRun(ctx context.Context, x domain.Run) error {
	_, err := r.db.Exec(ctx, `INSERT INTO hrd_tjk_sync_runs (`+runCols+`) VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$8,NULL,NULL,NULL,NULL,0,0,0,0,0,0,0,NULL,$9,$10,$11)`, x.ID, x.Mode, x.Status, x.SourceAdapter, x.Scope, x.Checkpoint, x.TriggerKind, x.CreatedByUserID, x.Version, x.CreatedAt, x.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperr.Conflict("Bu kaynak ve kapsam için zaten aktif bir TJK senkronizasyonu var.")
		}
		return apperr.Internal(fmt.Errorf("create TJK run: %w", err))
	}
	return nil
}
func (r *Repository) EnqueueRun(ctx context.Context, id uuid.UUID, now time.Time) error {
	payload, _ := json.Marshal(map[string]int{"page": 0})
	_, err := r.db.Exec(ctx, `INSERT INTO hrd_background_jobs (id,job_type,status,payload,tjk_sync_run_id,deduplication_key,max_attempts,available_at,version,created_at,updated_at) VALUES ($1,'TJK_SYNC_BATCH','QUEUED',$2::jsonb,$3,$4,3,$5,1,$5,$5)`, uuid.New(), payload, id, pageDedupKey(id, 0), now)
	if err != nil {
		return apperr.Internal(fmt.Errorf("enqueue TJK run: %w", err))
	}
	return nil
}

// CreateRunAndEnqueue persists the manual run and its bootstrap page job in one
// transaction. A caller can never observe an active QUEUED run without work.
func (r *Repository) CreateRunAndEnqueue(ctx context.Context, run domain.Run, now time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(fmt.Errorf("begin TJK trigger transaction: %w", err))
	}
	defer func() { _ = tx.Rollback(ctx) }()
	repo := r.WithTx(tx)
	if err := repo.CreateRun(ctx, run); err != nil {
		return err
	}
	if err := repo.EnqueueRun(ctx, run.ID, now); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return apperr.Internal(fmt.Errorf("commit TJK trigger transaction: %w", err))
	}
	return nil
}

func pageDedupKey(runID uuid.UUID, page int) string {
	return fmt.Sprintf("TJK_SYNC_BATCH:%s:%d", runID.String(), page)
}
func (r *Repository) ListRuns(ctx context.Context, cursor, status *string, limit int) ([]domain.Run, bool, error) {
	q, args := `SELECT `+runCols+` FROM hrd_tjk_sync_runs WHERE ($1::timestamptz IS NULL OR created_at < $1) AND ($2::varchar IS NULL OR status=$2) ORDER BY created_at DESC LIMIT $3`, []any{nil, nil, limit + 1}
	if cursor != nil {
		args[0] = *cursor
	}
	if status != nil {
		args[1] = *status
	}
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, false, apperr.Internal(fmt.Errorf("list TJK runs: %w", err))
	}
	defer rows.Close()
	out := []domain.Run{}
	for rows.Next() {
		x, e := scanRun(rows)
		if e != nil {
			return nil, false, apperr.Internal(e)
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, false, apperr.Internal(err)
	}
	more := len(out) > limit
	if more {
		out = out[:limit]
	}
	return out, more, nil
}
func (r *Repository) GetRun(ctx context.Context, id uuid.UUID) (domain.Run, error) {
	x, e := scanRun(r.db.QueryRow(ctx, `SELECT `+runCols+` FROM hrd_tjk_sync_runs WHERE id=$1`, id))
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Run{}, apperr.NotFound("TJK senkronizasyonu bulunamadı.")
	}
	if e != nil {
		return domain.Run{}, apperr.Internal(e)
	}
	return x, nil
}
func (r *Repository) RequestCancel(ctx context.Context, id uuid.UUID, version int, now time.Time) (domain.Run, error) {
	// Never-leased QUEUED runs hard-terminalize immediately so the one-active
	// unique index releases without waiting for a worker.
	x, e := scanRun(r.db.QueryRow(ctx, `
WITH cancelled_run AS (
  UPDATE hrd_tjk_sync_runs SET
    status='CANCELLED',
    cancel_requested_at=COALESCE(cancel_requested_at,$3),
    cancelled_at=$3,
    completed_at=$3,
    version=version+1,
    updated_at=$3
  WHERE id=$1 AND version=$2 AND status='QUEUED'
  RETURNING `+runCols+`
), terminalized_jobs AS (
  UPDATE hrd_background_jobs SET
    status='CANCELLED',
    cancel_requested_at=COALESCE(cancel_requested_at,$3),
    completed_at=COALESCE(completed_at,$3),
    lease_owner=NULL,
    leased_until=NULL,
    version=version+1,
    updated_at=$3
  WHERE tjk_sync_run_id=(SELECT id FROM cancelled_run)
    AND status IN ('QUEUED','LEASED')
  RETURNING id
)
SELECT `+runCols+` FROM cancelled_run`, id, version, now))
	if e == nil {
		return x, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return domain.Run{}, apperr.Internal(e)
	}

	// RUNNING stays cooperative soft-cancel until the worker checkpoints.
	x, e = scanRun(r.db.QueryRow(ctx, `
WITH cancel_requested_run AS (
  UPDATE hrd_tjk_sync_runs SET
    cancel_requested_at=COALESCE(cancel_requested_at,$3),
    version=version+1,
    updated_at=$3
  WHERE id=$1 AND version=$2 AND status='RUNNING'
  RETURNING `+runCols+`
), cancel_requested_jobs AS (
  UPDATE hrd_background_jobs SET
    cancel_requested_at=COALESCE(cancel_requested_at,$3),
    updated_at=$3
  WHERE tjk_sync_run_id=(SELECT id FROM cancel_requested_run)
    AND status IN ('QUEUED','LEASED')
  RETURNING id
)
SELECT `+runCols+` FROM cancel_requested_run`, id, version, now))
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Run{}, apperr.Conflict("TJK senkronizasyonu güncellenemedi.")
	}
	if e != nil {
		return domain.Run{}, apperr.Internal(e)
	}
	return x, nil
}
func (r *Repository) ListItemErrors(ctx context.Context, runID uuid.UUID, cursor, status *string, limit int) ([]domain.ItemError, bool, error) {
	q := `SELECT id,run_id,tjk_number,horse_id,error_class,status,message,created_at,resolved_at FROM hrd_tjk_sync_item_errors WHERE run_id=$1 AND ($2::timestamptz IS NULL OR created_at < $2) AND ($3::varchar IS NULL OR status=$3) ORDER BY created_at DESC LIMIT $4`
	args := []any{runID, nil, nil, limit + 1}
	if cursor != nil {
		args[1] = *cursor
	}
	if status != nil {
		args[2] = *status
	}
	rows, e := r.db.Query(ctx, q, args...)
	if e != nil {
		return nil, false, apperr.Internal(e)
	}
	defer rows.Close()
	out := []domain.ItemError{}
	for rows.Next() {
		var x domain.ItemError
		if e = rows.Scan(&x.ID, &x.RunID, &x.TJKNumber, &x.HorseID, &x.ErrorClass, &x.Status, &x.Message, &x.CreatedAt, &x.ResolvedAt); e != nil {
			return nil, false, apperr.Internal(e)
		}
		out = append(out, x)
	}
	more := len(out) > limit
	if more {
		out = out[:limit]
	}
	return out, more, rows.Err()
}
func (r *Repository) SetItemErrorStatus(ctx context.Context, id uuid.UUID, status string, now time.Time) (domain.ItemError, error) {
	var x domain.ItemError
	e := r.db.QueryRow(ctx, `UPDATE hrd_tjk_sync_item_errors SET status=$2,resolved_at=$3 WHERE id=$1 AND status='OPEN' RETURNING id,run_id,tjk_number,horse_id,error_class,status,message,created_at,resolved_at`, id, status, now).Scan(&x.ID, &x.RunID, &x.TJKNumber, &x.HorseID, &x.ErrorClass, &x.Status, &x.Message, &x.CreatedAt, &x.ResolvedAt)
	if errors.Is(e, pgx.ErrNoRows) {
		return x, apperr.Conflict("TJK hata kaydı güncellenemedi.")
	}
	if e != nil {
		return x, apperr.Internal(e)
	}
	return x, nil
}
func scanRun(row interface{ Scan(...any) error }) (domain.Run, error) {
	var x domain.Run
	var cp []byte
	e := row.Scan(&x.ID, &x.Mode, &x.Status, &x.SourceAdapter, &x.Scope, &cp, &x.TriggerKind, &x.CreatedByUserID, &x.CancelRequestedAt, &x.CancelledAt, &x.StartedAt, &x.CompletedAt, &x.TotalCount, &x.CreatedCount, &x.UpdatedCount, &x.UnchangedCount, &x.SkippedCount, &x.FailedCount, &x.ConflictCount, &x.LastErrorSummary, &x.Version, &x.CreatedAt, &x.UpdatedAt)
	x.Checkpoint = json.RawMessage(cp)
	return x, e
}

func (r *Repository) ClaimTJKJob(ctx context.Context, owner string, now, leaseUntil time.Time) (domain.PageJob, domain.Run, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.PageJob{}, domain.Run{}, false, apperr.Internal(fmt.Errorf("begin TJK claim: %w", err))
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var job domain.PageJob
	var runID uuid.UUID
	var payload []byte
	err = tx.QueryRow(ctx, `
SELECT j.id,j.tjk_sync_run_id,j.payload
FROM hrd_background_jobs j
JOIN hrd_tjk_sync_runs r ON r.id=j.tjk_sync_run_id
WHERE j.job_type='TJK_SYNC_BATCH' AND j.status='QUEUED' AND j.available_at <= $1
  AND j.attempt_count < j.max_attempts AND r.status IN ('QUEUED','RUNNING')
ORDER BY j.available_at,j.id
FOR UPDATE OF j,r SKIP LOCKED LIMIT 1`, now).Scan(&job.ID, &runID, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PageJob{}, domain.Run{}, false, nil
	}
	if err != nil {
		return domain.PageJob{}, domain.Run{}, false, apperr.Internal(fmt.Errorf("select TJK job: %w", err))
	}
	var body struct {
		Page int `json:"page"`
	}
	if json.Unmarshal(payload, &body) != nil || body.Page < 0 {
		return domain.PageJob{}, domain.Run{}, false, apperr.Internal(errors.New("invalid TJK page job payload"))
	}
	job.Page = body.Page
	tag, err := tx.Exec(ctx, `UPDATE hrd_background_jobs SET status='LEASED',lease_owner=$2,leased_until=$3,attempt_count=attempt_count+1,version=version+1,started_at=COALESCE(started_at,$1),updated_at=$1 WHERE id=$4 AND status='QUEUED'`, now, owner, leaseUntil, job.ID)
	if err != nil {
		return domain.PageJob{}, domain.Run{}, false, apperr.Internal(fmt.Errorf("lease TJK job: %w", err))
	}
	if tag.RowsAffected() != 1 {
		return domain.PageJob{}, domain.Run{}, false, apperr.InvalidState("TJK page job is no longer claimable")
	}
	if _, err = tx.Exec(ctx, `UPDATE hrd_tjk_sync_runs SET status='RUNNING',started_at=COALESCE(started_at,$2),updated_at=$2 WHERE id=$1`, runID, now); err != nil {
		return domain.PageJob{}, domain.Run{}, false, apperr.Internal(fmt.Errorf("start TJK run: %w", err))
	}
	run, err := scanRun(tx.QueryRow(ctx, `SELECT `+runCols+` FROM hrd_tjk_sync_runs WHERE id=$1`, runID))
	if err != nil {
		return domain.PageJob{}, domain.Run{}, false, apperr.Internal(fmt.Errorf("load claimed TJK run: %w", err))
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PageJob{}, domain.Run{}, false, apperr.Internal(fmt.Errorf("commit TJK claim: %w", err))
	}
	return job, run, true, nil
}

type runCheckpoint struct {
	Page            int    `json:"page"`
	LastFingerprint string `json:"lastFingerprint,omitempty"`
	PagesProcessed  int    `json:"pagesProcessed,omitempty"`
	SourceProcessed int    `json:"sourceProcessed,omitempty"`
	SourceTotal     *int   `json:"sourceTotal,omitempty"`
}

type storedHorse struct {
	ID                             uuid.UUID
	Name, Normalized               string
	BirthYear                      *int
	Sire, Dam, Breed, Gender, Coat *string
	Detail                         []byte
	LastSeen                       *time.Time
}

type pageCounters struct{ total, created, updated, unchanged, skipped, failed, conflict int }

func (r *Repository) ApplyTJKPage(ctx context.Context, job domain.PageJob, claimedRun domain.Run, page domain.PageResult, now time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(fmt.Errorf("begin TJK page transaction: %w", err))
	}
	defer func() { _ = tx.Rollback(ctx) }()
	repo := r.WithTx(tx)
	run, err := scanRun(tx.QueryRow(ctx, `SELECT `+runCols+` FROM hrd_tjk_sync_runs WHERE id=$1 FOR UPDATE`, claimedRun.ID))
	if err != nil {
		return apperr.Internal(fmt.Errorf("lock TJK run: %w", err))
	}
	if run.Status != domain.RunRunning || run.CancelRequestedAt != nil {
		return apperr.InvalidState("TJK run is not writable")
	}
	cp, err := decodeRunCheckpoint(run.Checkpoint)
	if err != nil || cp.Page != job.Page {
		return apperr.InvalidState("TJK checkpoint does not match page job")
	}
	if err := lockLeasedPageJob(ctx, tx, job, run.ID); err != nil {
		return err
	}
	if page.Fingerprint == "" || page.EndOfSource {
		return apperr.InvalidState("TJK page outcome is not applicable")
	}
	if cp.LastFingerprint == page.Fingerprint {
		return apperr.InvalidState("TJK page did not advance")
	}
	if page.SourceTotal != nil && *page.SourceTotal > 0 {
		if cp.SourceTotal != nil && *cp.SourceTotal != *page.SourceTotal {
			return apperr.InvalidState("TJK source total changed during run")
		}
		if cp.SourceTotal == nil {
			total := *page.SourceTotal
			cp.SourceTotal = &total
		}
	}

	counters := pageCounters{total: len(page.Horses) + page.SkippedCount, skipped: page.SkippedCount}
	if page.SkippedCount > 0 {
		if err := insertItemError(ctx, repo.db, run.ID, job.Page, nil, "summary", "PERMANENT", fmt.Sprintf("%d malformed TJK summary row(s) were skipped", page.SkippedCount), now); err != nil {
			return err
		}
	}
	seen := make(map[string]struct{}, len(page.Horses))
	seenAt := now
	if run.StartedAt != nil {
		seenAt = *run.StartedAt
	}
	for _, horse := range page.Horses {
		number, name := strings.TrimSpace(horse.Number), strings.TrimSpace(horse.Name)
		if number == "" || name == "" {
			counters.skipped++
			if err := insertItemError(ctx, repo.db, run.ID, job.Page, nil, "summary", "PERMANENT", "Malformed TJK horse was skipped", now); err != nil {
				return err
			}
			continue
		}
		if _, duplicate := seen[number]; duplicate {
			counters.conflict++
			if err := insertItemError(ctx, repo.db, run.ID, job.Page, &number, "duplicate", "CONFLICT", "Duplicate TJK horse number in source page", now); err != nil {
				return err
			}
			continue
		}
		seen[number] = struct{}{}
		outcome, err := upsertHorse(ctx, repo.db, run, horse, number, name, seenAt, now)
		if err != nil {
			return err
		}
		switch outcome {
		case "created":
			counters.created++
		case "updated":
			counters.updated++
		case "unchanged":
			counters.unchanged++
		case "conflict":
			counters.conflict++
			if err := insertItemError(ctx, repo.db, run.ID, job.Page, &number, "duplicate", "CONFLICT", "TJK horse number repeated across source pages", now); err != nil {
				return err
			}
		}
		if len(horse.EnrichmentIssues) > 0 {
			counters.failed++
			for _, issue := range horse.EnrichmentIssues {
				if err := insertItemError(ctx, repo.db, run.ID, job.Page, &number, issue.Component, "TRANSIENT", issue.Message, now); err != nil {
					return err
				}
			}
		}
	}
	cp.Page++
	cp.LastFingerprint = page.Fingerprint
	cp.PagesProcessed++
	cp.SourceProcessed += counters.total
	if cp.SourceTotal != nil && cp.SourceProcessed > *cp.SourceTotal {
		return apperr.InvalidState("TJK processed count exceeds source total")
	}
	cpRaw, _ := json.Marshal(cp)
	var summary any
	if counters.failed > 0 || counters.skipped > 0 || counters.conflict > 0 {
		summary = "TJK page completed with observable item issues"
	}
	tag, err := tx.Exec(ctx, `UPDATE hrd_tjk_sync_runs SET checkpoint=$2::jsonb,
total_count=total_count+$3,created_count=created_count+$4,updated_count=updated_count+$5,
unchanged_count=unchanged_count+$6,skipped_count=skipped_count+$7,failed_count=failed_count+$8,
conflict_count=conflict_count+$9,last_error_summary=COALESCE($10,last_error_summary),
version=version+1,updated_at=$11 WHERE id=$1 AND status='RUNNING' AND cancel_requested_at IS NULL`,
		run.ID, cpRaw, counters.total, counters.created, counters.updated, counters.unchanged,
		counters.skipped, counters.failed, counters.conflict, summary, now)
	if err != nil {
		return apperr.Internal(fmt.Errorf("advance TJK checkpoint: %w", err))
	}
	if tag.RowsAffected() != 1 {
		return apperr.InvalidState("TJK run checkpoint is no longer writable")
	}
	if err := completePageJob(ctx, tx, job.ID, now, "SUCCEEDED"); err != nil {
		return err
	}
	nextPayload, _ := json.Marshal(map[string]int{"page": cp.Page})
	if _, err := tx.Exec(ctx, `INSERT INTO hrd_background_jobs
(id,job_type,status,payload,tjk_sync_run_id,deduplication_key,max_attempts,available_at,version,created_at,updated_at)
VALUES ($1,'TJK_SYNC_BATCH','QUEUED',$2::jsonb,$3,$4,3,$5,1,$5,$5) ON CONFLICT DO NOTHING`,
		uuid.New(), nextPayload, run.ID, pageDedupKey(run.ID, cp.Page), now); err != nil {
		return apperr.Internal(fmt.Errorf("enqueue next TJK page: %w", err))
	}
	if err := tx.Commit(ctx); err != nil {
		return apperr.Internal(fmt.Errorf("commit TJK page transaction: %w", err))
	}
	return nil
}

func (r *Repository) FinishTJKRun(ctx context.Context, job domain.PageJob, claimedRun domain.Run, now time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(fmt.Errorf("begin TJK finish transaction: %w", err))
	}
	defer func() { _ = tx.Rollback(ctx) }()
	run, err := scanRun(tx.QueryRow(ctx, `SELECT `+runCols+` FROM hrd_tjk_sync_runs WHERE id=$1 FOR UPDATE`, claimedRun.ID))
	if err != nil {
		return apperr.Internal(fmt.Errorf("lock finishing TJK run: %w", err))
	}
	if err := lockLeasedPageJob(ctx, tx, job, run.ID); err != nil {
		return err
	}
	cp, err := decodeRunCheckpoint(run.Checkpoint)
	if err != nil || cp.Page != job.Page {
		return apperr.InvalidState("TJK EOF job does not match checkpoint")
	}
	status := domain.RunSucceeded
	jobStatus := "SUCCEEDED"
	if run.CancelRequestedAt != nil {
		status, jobStatus = domain.RunCancelled, "CANCELLED"
	} else {
		if cp.SourceTotal != nil && cp.SourceProcessed != *cp.SourceTotal {
			return apperr.InvalidState("TJK EOF reached before source total was processed")
		}
		if run.FailedCount > 0 || run.SkippedCount > 0 || run.ConflictCount > 0 {
			status = domain.RunPartialSuccess
		}
	}
	_, err = tx.Exec(ctx, `UPDATE hrd_tjk_sync_runs SET status=$2::varchar,completed_at=$3::timestamptz,
cancelled_at=CASE WHEN $2::varchar='CANCELLED' THEN $3::timestamptz ELSE NULL END,updated_at=$3::timestamptz,version=version+1 WHERE id=$1`, run.ID, status, now)
	if err != nil {
		return apperr.Internal(fmt.Errorf("finish TJK run: %w", err))
	}
	if err := completePageJob(ctx, tx, job.ID, now, jobStatus); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return apperr.Internal(fmt.Errorf("commit TJK finish transaction: %w", err))
	}
	return nil
}

// FailTJKJob terminates or requeues a LEASED TJK batch job. When retryable is
// true and attempt_count < max_attempts, the job returns to QUEUED with a short
// backoff; otherwise it becomes FAILED (permanent) or DEAD (attempts exhausted).
func (r *Repository) FailTJKJob(ctx context.Context, job domain.PageJob, runID uuid.UUID, message string, now time.Time, retryable bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(fmt.Errorf("begin TJK failure transaction: %w", err))
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	var attempts, maxAttempts int
	err = tx.QueryRow(ctx, `SELECT status,attempt_count,max_attempts FROM hrd_background_jobs WHERE id=$1 AND tjk_sync_run_id=$2 FOR UPDATE`, job.ID, runID).Scan(&status, &attempts, &maxAttempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("TJK job not found")
	}
	if err != nil {
		return apperr.Internal(fmt.Errorf("lock failed TJK job: %w", err))
	}
	if status != "LEASED" {
		return nil
	}
	if retryable && attempts < maxAttempts {
		_, err = tx.Exec(ctx, `UPDATE hrd_background_jobs SET status='QUEUED',available_at=$2::timestamptz+INTERVAL '5 seconds',
lease_owner=NULL,leased_until=NULL,last_error=$3,updated_at=$2::timestamptz WHERE id=$1`, job.ID, now, message)
		if err != nil {
			return apperr.Internal(fmt.Errorf("requeue TJK job: %w", err))
		}
		return tx.Commit(ctx)
	}
	terminalJob := "FAILED"
	if retryable {
		terminalJob = "DEAD"
	}
	_, err = tx.Exec(ctx, `UPDATE hrd_background_jobs SET status=$2::varchar,last_error=$3,lease_owner=NULL,leased_until=NULL,
completed_at=$4::timestamptz,updated_at=$4::timestamptz WHERE id=$1`, job.ID, terminalJob, message, now)
	if err != nil {
		return apperr.Internal(fmt.Errorf("terminalize TJK job: %w", err))
	}
	_, err = tx.Exec(ctx, `UPDATE hrd_background_jobs SET status='FAILED',last_error=$2,completed_at=$3::timestamptz,updated_at=$3::timestamptz
WHERE tjk_sync_run_id=$1 AND id<>$4 AND status='QUEUED'`, runID, message, now, job.ID)
	if err != nil {
		return apperr.Internal(fmt.Errorf("terminalize successor TJK jobs: %w", err))
	}
	_, err = tx.Exec(ctx, `UPDATE hrd_tjk_sync_runs SET status='FAILED',completed_at=$2::timestamptz,last_error_summary=$3,
version=version+1,updated_at=$2::timestamptz WHERE id=$1 AND status IN ('QUEUED','RUNNING')`, runID, now, message)
	if err != nil {
		return apperr.Internal(fmt.Errorf("terminalize TJK run: %w", err))
	}
	if err := tx.Commit(ctx); err != nil {
		return apperr.Internal(fmt.Errorf("commit TJK failure transaction: %w", err))
	}
	return nil
}

func decodeRunCheckpoint(raw json.RawMessage) (runCheckpoint, error) {
	var cp runCheckpoint
	if err := json.Unmarshal(raw, &cp); err == nil && cp.Page >= 0 {
		return cp, nil
	}
	var legacy struct {
		Page string `json:"page"`
	}
	if err := json.Unmarshal(raw, &legacy); err == nil {
		var page int
		if _, err := fmt.Sscanf(legacy.Page, "%d", &page); err == nil && page >= 0 {
			cp.Page = page
			return cp, nil
		}
	}
	return runCheckpoint{}, errors.New("invalid TJK checkpoint")
}

func lockLeasedPageJob(ctx context.Context, db DB, job domain.PageJob, runID uuid.UUID) error {
	var payload []byte
	var status string
	if err := db.QueryRow(ctx, `SELECT payload,status FROM hrd_background_jobs WHERE id=$1 AND tjk_sync_run_id=$2 FOR UPDATE`, job.ID, runID).Scan(&payload, &status); err != nil {
		return apperr.Internal(fmt.Errorf("lock TJK page job: %w", err))
	}
	var body struct {
		Page int `json:"page"`
	}
	if status != "LEASED" || json.Unmarshal(payload, &body) != nil || body.Page != job.Page {
		return apperr.InvalidState("TJK page job lease or identity is invalid")
	}
	return nil
}

func completePageJob(ctx context.Context, db DB, jobID uuid.UUID, now time.Time, status string) error {
	tag, err := db.Exec(ctx, `UPDATE hrd_background_jobs SET status=$2::varchar,
cancel_requested_at=CASE WHEN $2::varchar='CANCELLED' THEN COALESCE(cancel_requested_at,$3::timestamptz) ELSE cancel_requested_at END,
lease_owner=NULL,leased_until=NULL,completed_at=$3::timestamptz,updated_at=$3::timestamptz,version=version+1
WHERE id=$1 AND status='LEASED'`, jobID, status, now)
	if err != nil {
		return apperr.Internal(fmt.Errorf("complete TJK page job: %w", err))
	}
	if tag.RowsAffected() != 1 {
		return apperr.InvalidState("TJK page job is no longer leased")
	}
	return nil
}

func upsertHorse(ctx context.Context, db DB, run domain.Run, horse domain.HorseInput, number, name string, seenAt, now time.Time) (string, error) {
	var current storedHorse
	err := db.QueryRow(ctx, `SELECT id,original_name,name_normalized,birth_year,sire_name,dam_name,breed,gender,coat,detail,last_seen_at
FROM hrd_horses WHERE tjk_number=$1 FOR UPDATE`, number).Scan(
		&current.ID, &current.Name, &current.Normalized, &current.BirthYear, &current.Sire,
		&current.Dam, &current.Breed, &current.Gender, &current.Coat, &current.Detail, &current.LastSeen,
	)
	normalized := textnorm.TurkishFold(name)
	detail := normalizeDetail(horse.Detail)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = db.Exec(ctx, `INSERT INTO hrd_horses
(id,tjk_number,original_name,name_normalized,birth_year,sire_name,dam_name,breed,gender,coat,detail,last_synced_at,last_seen_at,created_at,updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,$12,$13,$12,$12)`, uuid.New(), number, name, normalized,
			horse.BirthYear, nullableString(horse.Sire), nullableString(horse.Dam), nullableString(horse.Race),
			nullableStringPtr(horse.Gender), nullableStringPtr(horse.Coat), detail, now, seenAt)
		if err != nil {
			return "", apperr.Internal(fmt.Errorf("insert TJK horse: %w", err))
		}
		return "created", nil
	}
	if err != nil {
		return "", apperr.Internal(fmt.Errorf("load TJK horse: %w", err))
	}
	if run.StartedAt != nil && current.LastSeen != nil && current.LastSeen.Equal(*run.StartedAt) {
		return "conflict", nil
	}
	birth := chooseInt(horse.BirthYear, current.BirthYear)
	sire := chooseString(horse.Sire, current.Sire)
	dam := chooseString(horse.Dam, current.Dam)
	breed := chooseString(horse.Race, current.Breed)
	gender := chooseStringPtr(horse.Gender, current.Gender)
	coat := chooseStringPtr(horse.Coat, current.Coat)
	mergedDetail := mergeDetail(current.Detail, horse.Detail)
	changed := current.Name != name || current.Normalized != normalized ||
		!reflect.DeepEqual(current.BirthYear, birth) || !reflect.DeepEqual(current.Sire, sire) ||
		!reflect.DeepEqual(current.Dam, dam) || !reflect.DeepEqual(current.Breed, breed) ||
		!reflect.DeepEqual(current.Gender, gender) || !reflect.DeepEqual(current.Coat, coat) ||
		!jsonObjectsEqual(current.Detail, mergedDetail)
	_, err = db.Exec(ctx, `UPDATE hrd_horses SET original_name=$2,name_normalized=$3,birth_year=$4,sire_name=$5,
dam_name=$6,breed=$7,gender=$8,coat=$9,detail=$10::jsonb,last_synced_at=$11,last_seen_at=$12,updated_at=$11 WHERE id=$1`,
		current.ID, name, normalized, birth, sire, dam, breed, gender, coat, mergedDetail, now, seenAt)
	if err != nil {
		return "", apperr.Internal(fmt.Errorf("update TJK horse: %w", err))
	}
	if changed {
		return "updated", nil
	}
	return "unchanged", nil
}

func nullableString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func nullableStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	return nullableString(*value)
}

func chooseString(value string, current *string) *string {
	if next := nullableString(value); next != nil {
		return next
	}
	return current
}

func chooseStringPtr(value, current *string) *string {
	if next := nullableStringPtr(value); next != nil {
		return next
	}
	return current
}

func chooseInt(value, current *int) *int {
	if value != nil {
		return value
	}
	return current
}

func normalizeDetail(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte(`{}`)
	}
	var object map[string]any
	if json.Unmarshal(raw, &object) != nil {
		return []byte(`{}`)
	}
	out, _ := json.Marshal(object)
	return out
}

func mergeDetail(current []byte, incoming json.RawMessage) []byte {
	if len(incoming) == 0 {
		return normalizeDetail(current)
	}
	var base, delta map[string]any
	if json.Unmarshal(current, &base) != nil || base == nil {
		base = map[string]any{}
	}
	if json.Unmarshal(incoming, &delta) != nil {
		return normalizeDetail(current)
	}
	for key, value := range delta {
		base[key] = value
	}
	out, _ := json.Marshal(base)
	return out
}

func jsonObjectsEqual(a, b []byte) bool {
	var left, right any
	return json.Unmarshal(a, &left) == nil && json.Unmarshal(b, &right) == nil && reflect.DeepEqual(left, right)
}

func insertItemError(ctx context.Context, db DB, runID uuid.UUID, page int, tjkNumber *string, component, class, message string, now time.Time) error {
	key := fmt.Sprintf("%s:%d:%s:%s", runID, page, component, valueOrEmpty(tjkNumber))
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(key))
	batch, _ := json.Marshal(map[string]int{"page": page})
	detail, _ := json.Marshal(map[string]string{"component": component})
	_, err := db.Exec(ctx, `INSERT INTO hrd_tjk_sync_item_errors
(id,run_id,tjk_number,batch_context,error_class,status,message,detail,created_at)
VALUES ($1,$2,$3,$4::jsonb,$5,'OPEN',$6,$7::jsonb,$8) ON CONFLICT (id) DO NOTHING`,
		id, runID, tjkNumber, batch, class, message, detail, now)
	if err != nil {
		return apperr.Internal(fmt.Errorf("record TJK item error: %w", err))
	}
	return nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
