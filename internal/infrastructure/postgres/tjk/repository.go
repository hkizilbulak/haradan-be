package tjk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domain "github.com/hkizilbulak/haradan-be/internal/domain/tjk"
	"github.com/hkizilbulak/haradan-be/internal/platform/textnorm"
)

type DB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type Repository struct{ db DB }

func NewRepository(db DB) *Repository { return &Repository{db: db} }

const runCols = `id,mode,status,source_adapter,scope_key,checkpoint,trigger_kind,created_by_user_id,cancel_requested_at,cancelled_at,started_at,completed_at,total_count,created_count,updated_count,unchanged_count,skipped_count,failed_count,conflict_count,last_error_summary,version,created_at,updated_at`
const qualifiedRunCols = `r.id,r.mode,r.status,r.source_adapter,r.scope_key,r.checkpoint,r.trigger_kind,r.created_by_user_id,r.cancel_requested_at,r.cancelled_at,r.started_at,r.completed_at,r.total_count,r.created_count,r.updated_count,r.unchanged_count,r.skipped_count,r.failed_count,r.conflict_count,r.last_error_summary,r.version,r.created_at,r.updated_at`

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
	_, err := r.db.Exec(ctx, `INSERT INTO hrd_background_jobs (id,job_type,status,payload,tjk_sync_run_id,max_attempts,available_at,version,created_at,updated_at) VALUES ($1,'TJK_SYNC_BATCH','QUEUED','{}',$2,3,$3,1,$3,$3)`, uuid.New(), id, now)
	if err != nil {
		return apperr.Internal(fmt.Errorf("enqueue TJK run: %w", err))
	}
	return nil
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

func (r *Repository) ClaimTJKJob(ctx context.Context, owner string, now, leaseUntil time.Time) (uuid.UUID, domain.Run, bool, error) {
	// Skip jobs whose run is already terminal (e.g. hard-cancelled QUEUED). Soft-
	// cancelled RUNNING jobs remain claimable so the worker can FinishTJKRun.
	const q = `WITH next AS (
 SELECT j.id FROM hrd_background_jobs j
 JOIN hrd_tjk_sync_runs r ON r.id=j.tjk_sync_run_id
 WHERE j.job_type='TJK_SYNC_BATCH'
   AND j.status='QUEUED'
   AND j.available_at <= $1
   AND j.attempt_count < j.max_attempts
   AND r.status IN ('QUEUED','RUNNING')
 ORDER BY j.available_at,j.id FOR UPDATE SKIP LOCKED LIMIT 1
), claimed AS (
 UPDATE hrd_background_jobs j SET status='LEASED',lease_owner=$2,leased_until=$3,attempt_count=attempt_count+1,version=version+1,updated_at=$1
 FROM next WHERE j.id=next.id
 RETURNING j.id,j.tjk_sync_run_id
) SELECT c.id,` + qualifiedRunCols + ` FROM claimed c JOIN hrd_tjk_sync_runs r ON r.id=c.tjk_sync_run_id`
	var id uuid.UUID
	var run domain.Run
	row := r.db.QueryRow(ctx, q, now, owner, leaseUntil)
	var cp []byte
	err := row.Scan(&id, &run.ID, &run.Mode, &run.Status, &run.SourceAdapter, &run.Scope, &cp, &run.TriggerKind, &run.CreatedByUserID, &run.CancelRequestedAt, &run.CancelledAt, &run.StartedAt, &run.CompletedAt, &run.TotalCount, &run.CreatedCount, &run.UpdatedCount, &run.UnchangedCount, &run.SkippedCount, &run.FailedCount, &run.ConflictCount, &run.LastErrorSummary, &run.Version, &run.CreatedAt, &run.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, domain.Run{}, false, nil
	}
	if err != nil {
		return uuid.Nil, domain.Run{}, false, apperr.Internal(fmt.Errorf("claim TJK job: %w", err))
	}
	run.Checkpoint = json.RawMessage(cp)
	if run.Status == domain.RunQueued {
		_, err = r.db.Exec(ctx, `UPDATE hrd_tjk_sync_runs SET status='RUNNING',started_at=COALESCE(started_at,$2),updated_at=$2 WHERE id=$1`, run.ID, now)
		if err != nil {
			return uuid.Nil, domain.Run{}, false, apperr.Internal(err)
		}
		run.Status = domain.RunRunning
	}
	return id, run, true, nil
}

func (r *Repository) ApplyTJKPage(ctx context.Context, jobID uuid.UUID, run domain.Run, horses []domain.HorseInput, nextCursor string, now time.Time) error {
	applied := 0
	for _, h := range horses {
		if h.Number == "" || h.Name == "" {
			// Malformed summary rows are skipped; they must not abort the page.
			continue
		}
		detail := h.Detail
		if len(detail) == 0 {
			detail = json.RawMessage(`{}`)
		}
		breed := strings.TrimSpace(h.Race)
		var gender any
		if h.Gender != nil {
			g := strings.TrimSpace(*h.Gender)
			if g != "" {
				gender = g
			}
		}
		_, err := r.db.Exec(ctx, `
INSERT INTO hrd_horses (
  id, tjk_number, original_name, name_normalized,
  birth_year, sire_name, dam_name, breed, gender,
  detail, last_synced_at, last_seen_at, created_at, updated_at
) VALUES (
  $1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),$9,
  $10::jsonb,$11,$11,$11,$11
)
ON CONFLICT (tjk_number) DO UPDATE SET
  original_name = EXCLUDED.original_name,
  name_normalized = EXCLUDED.name_normalized,
  birth_year = COALESCE(EXCLUDED.birth_year, hrd_horses.birth_year),
  sire_name = COALESCE(EXCLUDED.sire_name, hrd_horses.sire_name),
  dam_name = COALESCE(EXCLUDED.dam_name, hrd_horses.dam_name),
  breed = COALESCE(EXCLUDED.breed, hrd_horses.breed),
  gender = COALESCE(EXCLUDED.gender, hrd_horses.gender),
  detail = CASE
    WHEN EXCLUDED.detail = '{}'::jsonb THEN hrd_horses.detail
    ELSE COALESCE(hrd_horses.detail, '{}'::jsonb) || EXCLUDED.detail
  END,
  last_synced_at = $11,
  last_seen_at = $11,
  updated_at = $11`,
			uuid.New(), h.Number, h.Name, textnorm.TurkishFold(h.Name),
			h.BirthYear, h.Sire, h.Dam, breed, gender,
			[]byte(detail), now)
		if err != nil {
			return apperr.Internal(fmt.Errorf("upsert TJK horse: %w", err))
		}
		applied++
	}
	cp, _ := json.Marshal(map[string]any{"page": nextCursor})
	_, err := r.db.Exec(ctx, `UPDATE hrd_tjk_sync_runs SET checkpoint=$2::jsonb,total_count=total_count+$3,updated_count=updated_count+$3,version=version+1,updated_at=$4 WHERE id=$1 AND cancel_requested_at IS NULL`, run.ID, cp, applied, now)
	if err != nil {
		return apperr.Internal(err)
	}
	_, err = r.db.Exec(ctx, `UPDATE hrd_background_jobs SET status='SUCCEEDED',lease_owner=NULL,leased_until=NULL,completed_at=$2,updated_at=$2 WHERE id=$1`, jobID, now)
	if err != nil {
		return apperr.Internal(err)
	}
	_, err = r.db.Exec(ctx, `INSERT INTO hrd_background_jobs (id,job_type,status,payload,tjk_sync_run_id,max_attempts,available_at,version,created_at,updated_at) VALUES ($1,'TJK_SYNC_BATCH','QUEUED','{}',$2,3,$3,1,$3,$3)`, uuid.New(), run.ID, now)
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}
func (r *Repository) FinishTJKRun(ctx context.Context, jobID uuid.UUID, run domain.Run, now time.Time) error {
	status := domain.RunSucceeded
	if run.CancelRequestedAt != nil {
		status = domain.RunCancelled
	}
	_, err := r.db.Exec(ctx, `UPDATE hrd_tjk_sync_runs SET status=$2::varchar,completed_at=$3::timestamptz,cancelled_at=CASE WHEN $2::varchar='CANCELLED' THEN $3::timestamptz ELSE NULL END,updated_at=$3::timestamptz,version=version+1 WHERE id=$1`, run.ID, status, now)
	if err != nil {
		return apperr.Internal(fmt.Errorf("finish TJK run: %w", err))
	}
	_, err = r.db.Exec(ctx, `UPDATE hrd_background_jobs SET status=CASE WHEN $2::varchar='CANCELLED' THEN 'CANCELLED' ELSE 'SUCCEEDED' END,cancel_requested_at=CASE WHEN $2::varchar='CANCELLED' THEN COALESCE(cancel_requested_at,$3::timestamptz) ELSE cancel_requested_at END,lease_owner=NULL,leased_until=NULL,completed_at=$3::timestamptz,updated_at=$3::timestamptz WHERE id=$1`, jobID, status, now)
	if err != nil {
		return apperr.Internal(fmt.Errorf("finish TJK job: %w", err))
	}
	return nil
}

// FailTJKJob terminates or requeues a LEASED TJK batch job. When retryable is
// true and attempt_count < max_attempts, the job returns to QUEUED with a short
// backoff; otherwise it becomes FAILED (permanent) or DEAD (attempts exhausted).
func (r *Repository) FailTJKJob(ctx context.Context, jobID uuid.UUID, message string, now time.Time, retryable bool) error {
	if retryable {
		_, err := r.db.Exec(ctx, `
UPDATE hrd_background_jobs SET
  status = CASE WHEN attempt_count >= max_attempts THEN 'DEAD' ELSE 'QUEUED' END,
  completed_at = CASE WHEN attempt_count >= max_attempts THEN $2 ELSE NULL END,
  available_at = CASE
    WHEN attempt_count >= max_attempts THEN available_at
    ELSE $2 + INTERVAL '5 seconds'
  END,
  lease_owner = NULL,
  leased_until = NULL,
  last_error = $3,
  updated_at = $2
WHERE id = $1`, jobID, now, message)
		if err != nil {
			return apperr.Internal(err)
		}
		return nil
	}
	_, err := r.db.Exec(ctx, `
UPDATE hrd_background_jobs SET
  status='FAILED',
  last_error=$2,
  lease_owner=NULL,
  leased_until=NULL,
  completed_at=$3,
  updated_at=$3
WHERE id=$1`, jobID, message, now)
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}
