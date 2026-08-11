package tjk_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	apptjk "github.com/hkizilbulak/haradan-be/internal/application/tjk"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaintjk "github.com/hkizilbulak/haradan-be/internal/domain/tjk"
	pgtjk "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/tjk"
	tjkhttp "github.com/hkizilbulak/haradan-be/internal/infrastructure/tjk"
)

func requireTJKIntegration(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping TJK integration tests")
	}
	if live := strings.TrimSpace(os.Getenv("DATABASE_URL")); live != "" && live == dsn {
		t.Fatal("TEST_DATABASE_URL must not equal DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping test database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func seedActor(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)
	email := "tjk-itest-" + id.String() + "@example.test"
	_, err := pool.Exec(context.Background(), `
INSERT INTO hrd_users (
  id,email,email_normalized,password_hash,role,status,first_name,last_name,
  security_stamp,created_at,updated_at
) VALUES ($1,$2,$2,'integration-test-hash','admin','ACTIVE','TJK','Integration',$3,$4,$4)`,
		id, email, uuid.New(), now)
	if err != nil {
		t.Fatalf("seed actor: %v", err)
	}
	return id
}

func cleanupTJK(t *testing.T, pool *pgxpool.Pool, actorID uuid.UUID, horseNumbers ...string) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM hrd_tjk_sync_item_errors WHERE run_id IN (SELECT id FROM hrd_tjk_sync_runs WHERE created_by_user_id=$1)`, actorID)
		_, _ = pool.Exec(ctx, `DELETE FROM hrd_background_jobs WHERE tjk_sync_run_id IN (SELECT id FROM hrd_tjk_sync_runs WHERE created_by_user_id=$1)`, actorID)
		_, _ = pool.Exec(ctx, `DELETE FROM hrd_tjk_sync_runs WHERE created_by_user_id=$1`, actorID)
		for _, horseNumber := range horseNumbers {
			if horseNumber != "" {
				_, _ = pool.Exec(ctx, `DELETE FROM hrd_horses WHERE tjk_number=$1`, horseNumber)
			}
		}
		_, _ = pool.Exec(ctx, `DELETE FROM hrd_users WHERE id=$1`, actorID)
	})
}

func TestQueuedCancellationReleasesActiveRunIntegration(t *testing.T) {
	pool := requireTJKIntegration(t)
	actorID := seedActor(t, pool)
	cleanupTJK(t, pool, actorID, "")
	repo := pgtjk.NewRepository(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	svc, err := apptjk.NewService(apptjk.Config{Repo: repo, Enabled: true, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}

	first, err := svc.Trigger(context.Background(), actorID, "FULL", "TJK_HTTP")
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	cancelled, err := svc.Cancel(context.Background(), first.ID, first.Version)
	if err != nil {
		t.Fatalf("cancel queued: %v", err)
	}
	if cancelled.Status != domaintjk.RunCancelled || cancelled.StartedAt != nil || cancelled.CompletedAt == nil {
		t.Fatalf("unexpected cancelled run: %#v", cancelled)
	}
	var jobStatus string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM hrd_background_jobs WHERE tjk_sync_run_id=$1`, first.ID).Scan(&jobStatus); err != nil {
		t.Fatal(err)
	}
	if jobStatus != "CANCELLED" {
		t.Fatalf("associated job status=%s", jobStatus)
	}

	second, err := svc.Trigger(context.Background(), actorID, "FULL", "TJK_HTTP")
	if err != nil {
		t.Fatalf("immediate retrigger: %v", err)
	}
	if second.ID == first.ID || second.Status != domaintjk.RunQueued {
		t.Fatalf("unexpected retrigger: %#v", second)
	}
	if _, err := svc.Cancel(context.Background(), second.ID, second.Version+1); err == nil {
		t.Fatal("expected stale version conflict")
	} else if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindConflict {
		t.Fatalf("stale version error=%v", err)
	}
	if _, err := svc.Cancel(context.Background(), second.ID, second.Version); err != nil {
		t.Fatalf("cleanup second run: %v", err)
	}
}

func TestManualTriggerRollsBackRunWhenInitialJobFailsIntegration(t *testing.T) {
	pool := requireTJKIntegration(t)
	actorID := seedActor(t, pool)
	cleanupTJK(t, pool, actorID)
	ctx := context.Background()
	sql := fmt.Sprintf(`
CREATE OR REPLACE FUNCTION hrd_test_reject_tjk_bootstrap() RETURNS trigger AS $$
BEGIN
  IF NEW.job_type='TJK_SYNC_BATCH' AND EXISTS (
    SELECT 1 FROM hrd_tjk_sync_runs WHERE id=NEW.tjk_sync_run_id AND created_by_user_id='%s'::uuid
  ) THEN
    RAISE EXCEPTION 'intentional TJK bootstrap rollback';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS hrd_test_reject_tjk_bootstrap ON hrd_background_jobs;
CREATE TRIGGER hrd_test_reject_tjk_bootstrap BEFORE INSERT ON hrd_background_jobs
FOR EACH ROW EXECUTE FUNCTION hrd_test_reject_tjk_bootstrap()`, actorID)
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("install bootstrap trigger: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS hrd_test_reject_tjk_bootstrap ON hrd_background_jobs; DROP FUNCTION IF EXISTS hrd_test_reject_tjk_bootstrap()`)
	})
	repo := pgtjk.NewRepository(pool)
	svc, _ := apptjk.NewService(apptjk.Config{Repo: repo, Enabled: true})
	if _, err := svc.Trigger(ctx, actorID, "FULL", "TJK_HTTP"); err == nil {
		t.Fatal("expected bootstrap enqueue failure")
	}
	var runCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM hrd_tjk_sync_runs WHERE created_by_user_id=$1`, actorID).Scan(&runCount); err != nil {
		t.Fatal(err)
	}
	if runCount != 0 {
		t.Fatalf("manual trigger leaked %d active run(s)", runCount)
	}
}

func TestQueuedCancellationAssociatedJobCompatibilityIntegration(t *testing.T) {
	pool := requireTJKIntegration(t)
	actorID := seedActor(t, pool)
	cleanupTJK(t, pool, actorID, "")
	repo := pgtjk.NewRepository(pool)

	for _, tc := range []struct {
		name      string
		jobStatus string
		version   int
	}{
		{name: "missing"},
		{name: "stale-version-queued", jobStatus: "QUEUED", version: 7},
		{name: "terminal-succeeded", jobStatus: "SUCCEEDED", version: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Microsecond)
			run := domaintjk.Run{
				ID: uuid.New(), Mode: "FULL", Status: domaintjk.RunQueued,
				SourceAdapter: "TJK_HTTP", Scope: "HORSES", Checkpoint: []byte(`{"page":0}`),
				TriggerKind: "MANUAL", CreatedByUserID: &actorID, Version: 1,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := repo.CreateRun(context.Background(), run); err != nil {
				t.Fatal(err)
			}
			var jobID uuid.UUID
			if tc.jobStatus != "" {
				jobID = uuid.New()
				var completed any
				if tc.jobStatus == "SUCCEEDED" {
					completed = now
				}
				_, err := pool.Exec(context.Background(), `
INSERT INTO hrd_background_jobs (
  id,job_type,status,payload,tjk_sync_run_id,max_attempts,available_at,version,
  created_at,updated_at,completed_at
) VALUES ($1,'TJK_SYNC_BATCH',$2,'{}',$3,3,$4,$5,$4,$4,$6)`,
					jobID, tc.jobStatus, run.ID, now, tc.version, completed)
				if err != nil {
					t.Fatal(err)
				}
			}
			cancelled, err := repo.RequestCancel(context.Background(), run.ID, 1, now.Add(time.Millisecond))
			if err != nil || cancelled.Status != domaintjk.RunCancelled {
				t.Fatalf("cancelled=%#v err=%v", cancelled, err)
			}
			if jobID != uuid.Nil {
				var status string
				var version int
				if err := pool.QueryRow(context.Background(), `SELECT status,version FROM hrd_background_jobs WHERE id=$1`, jobID).Scan(&status, &version); err != nil {
					t.Fatal(err)
				}
				if tc.jobStatus == "QUEUED" && (status != "CANCELLED" || version != tc.version+1) {
					t.Fatalf("stale queued job status=%s version=%d", status, version)
				}
				if tc.jobStatus == "SUCCEEDED" && (status != "SUCCEEDED" || version != tc.version) {
					t.Fatalf("terminal job changed status=%s version=%d", status, version)
				}
			}
		})
	}
}

func TestRunningCancellationIsSoftIntegration(t *testing.T) {
	pool := requireTJKIntegration(t)
	actorID := seedActor(t, pool)
	cleanupTJK(t, pool, actorID, "")
	repo := pgtjk.NewRepository(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	svc, err := apptjk.NewService(apptjk.Config{Repo: repo, Enabled: true, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	run, err := svc.Trigger(context.Background(), actorID, "FULL", "TJK_HTTP")
	if err != nil {
		t.Fatal(err)
	}
	jobID, running, ok, err := repo.ClaimTJKJob(context.Background(), "integration-worker", now, now.Add(time.Minute))
	if err != nil || !ok || running.Status != domaintjk.RunRunning {
		t.Fatalf("claim ok=%v run=%#v err=%v", ok, running, err)
	}
	running, err = svc.Cancel(context.Background(), run.ID, run.Version)
	if err != nil {
		t.Fatal(err)
	}
	if running.Status != domaintjk.RunRunning || running.CancelRequestedAt == nil || running.CompletedAt != nil {
		t.Fatalf("running cancellation was not soft: %#v", running)
	}
	var jobStatus string
	var cancelRequested *time.Time
	if err := pool.QueryRow(context.Background(), `SELECT status,cancel_requested_at FROM hrd_background_jobs WHERE id=$1`, jobID.ID).Scan(&jobStatus, &cancelRequested); err != nil {
		t.Fatal(err)
	}
	if jobStatus != "LEASED" || cancelRequested == nil {
		t.Fatalf("leased job status=%s cancelRequested=%v", jobStatus, cancelRequested)
	}
	if _, err := svc.Trigger(context.Background(), actorID, "FULL", "TJK_HTTP"); err == nil {
		t.Fatal("soft-cancelled RUNNING run must retain active constraint")
	}
	if err := repo.FinishTJKRun(context.Background(), jobID, running, now.Add(time.Millisecond)); err != nil {
		t.Fatalf("finish soft cancellation: %v", errors.Unwrap(err))
	}
}

func TestPermanentFailureTerminalizesRunIntegration(t *testing.T) {
	pool := requireTJKIntegration(t)
	actorID := seedActor(t, pool)
	cleanupTJK(t, pool, actorID)
	repo := pgtjk.NewRepository(pool)
	svc, _ := apptjk.NewService(apptjk.Config{Repo: repo, Enabled: true})
	run, err := svc.Trigger(context.Background(), actorID, "FULL", "TJK_HTTP")
	if err != nil {
		t.Fatal(err)
	}
	job, _, ok, err := repo.ClaimTJKJob(context.Background(), "integration-worker", time.Now().UTC(), time.Now().UTC().Add(time.Minute))
	if err != nil || !ok {
		t.Fatalf("claim ok=%v err=%v", ok, err)
	}
	if err := repo.FailTJKJob(context.Background(), job, run.ID, "safe permanent failure", time.Now().UTC(), false); err != nil {
		t.Fatal(err)
	}
	failed, err := repo.GetRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != domaintjk.RunFailed || failed.CompletedAt == nil || failed.LastErrorSummary == nil {
		t.Fatalf("run was not terminalized: %#v", failed)
	}
	if next, err := svc.Trigger(context.Background(), actorID, "FULL", "TJK_HTTP"); err != nil {
		t.Fatalf("terminal failure must release active-run constraint: %v", err)
	} else if _, err := svc.Cancel(context.Background(), next.ID, next.Version); err != nil {
		t.Fatal(err)
	}
}

func TestRetryExhaustionTerminalizesRunIntegration(t *testing.T) {
	pool := requireTJKIntegration(t)
	actorID := seedActor(t, pool)
	cleanupTJK(t, pool, actorID)
	repo := pgtjk.NewRepository(pool)
	svc, _ := apptjk.NewService(apptjk.Config{Repo: repo, Enabled: true})
	run, err := svc.Trigger(context.Background(), actorID, "FULL", "TJK_HTTP")
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 3; attempt++ {
		now := time.Now().UTC().Add(time.Duration(attempt) * 10 * time.Second)
		job, _, ok, err := repo.ClaimTJKJob(context.Background(), "integration-worker", now, now.Add(time.Minute))
		if err != nil || !ok {
			t.Fatalf("attempt %d claim ok=%v err=%v", attempt, ok, err)
		}
		if err := repo.FailTJKJob(context.Background(), job, run.ID, "safe transient failure", now, true); err != nil {
			t.Fatal(err)
		}
	}
	failed, err := repo.GetRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	var jobStatus string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM hrd_background_jobs WHERE tjk_sync_run_id=$1`, run.ID).Scan(&jobStatus); err != nil {
		t.Fatal(err)
	}
	if failed.Status != domaintjk.RunFailed || failed.CompletedAt == nil || jobStatus != "DEAD" {
		t.Fatalf("retry exhaustion run=%#v job=%s", failed, jobStatus)
	}
}

func TestExpiredFinalLeaseTerminalizesTJKRunIntegration(t *testing.T) {
	pool := requireTJKIntegration(t)
	actorID := seedActor(t, pool)
	cleanupTJK(t, pool, actorID)
	repo := pgtjk.NewRepository(pool)
	svc, _ := apptjk.NewService(apptjk.Config{Repo: repo, Enabled: true})
	run, err := svc.Trigger(context.Background(), actorID, "FULL", "TJK_HTTP")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC()
	for attempt := 1; attempt <= 2; attempt++ {
		now := base.Add(time.Duration(attempt) * 10 * time.Second)
		job, _, ok, err := repo.ClaimTJKJob(context.Background(), "integration-worker", now, now.Add(time.Minute))
		if err != nil || !ok {
			t.Fatalf("attempt %d claim ok=%v err=%v", attempt, ok, err)
		}
		if err := repo.FailTJKJob(context.Background(), job, run.ID, "safe transient failure", now, true); err != nil {
			t.Fatal(err)
		}
	}
	claimAt := base.Add(30 * time.Second)
	if _, _, ok, err := repo.ClaimTJKJob(context.Background(), "crashed-worker", claimAt, claimAt.Add(time.Second)); err != nil || !ok {
		t.Fatalf("final claim ok=%v err=%v", ok, err)
	}
	queue, err := appmedia.NewPostgresJobQueue(pool)
	if err != nil {
		t.Fatal(err)
	}
	if recovered, err := queue.RecoverExpiredJobLeases(context.Background(), claimAt.Add(2*time.Second), 10); err != nil || recovered != 1 {
		t.Fatalf("recovered=%d err=%v", recovered, err)
	}
	failed, err := repo.GetRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != domaintjk.RunFailed || failed.CompletedAt == nil {
		t.Fatalf("expired final lease left active run: %#v", failed)
	}
}

type sequenceFetcher struct {
	pages map[string]domaintjk.PageResult
}

func (f sequenceFetcher) FetchPage(_ context.Context, cursor string) (domaintjk.PageResult, error) {
	page, ok := f.pages[cursor]
	if !ok {
		return domaintjk.PageResult{}, fmt.Errorf("unexpected page %s", cursor)
	}
	return page, nil
}

func TestCountersDuplicatesAndEnrichmentPartialIntegration(t *testing.T) {
	pool := requireTJKIntegration(t)
	actorID := seedActor(t, pool)
	number := "ITEST-PARTIAL-" + uuid.NewString()
	cleanupTJK(t, pool, actorID, number)
	repo := pgtjk.NewRepository(pool)
	svc, _ := apptjk.NewService(apptjk.Config{Repo: repo, Enabled: true})
	run, err := svc.Trigger(context.Background(), actorID, "FULL", "TJK_HTTP")
	if err != nil {
		t.Fatal(err)
	}
	total := 3
	worker, err := apptjk.NewWorker(repo, sequenceFetcher{pages: map[string]domaintjk.PageResult{
		"0": {
			Fingerprint: "partial-page", SourceTotal: &total, SkippedCount: 1,
			Horses: []domaintjk.HorseInput{
				{Number: number, Name: "PARTIAL HORSE", EnrichmentIssues: []domaintjk.EnrichmentIssue{{Component: "detail", Message: "detail unavailable"}}},
				{Number: number, Name: "PARTIAL HORSE DUPLICATE"},
			},
		},
		"1": {EndOfSource: true},
	}}, "integration-worker")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if claimed, err := worker.ProcessOnce(context.Background(), time.Minute); err != nil || !claimed {
			t.Fatalf("worker pass %d claimed=%v err=%v", i+1, claimed, err)
		}
	}
	finished, err := repo.GetRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if finished.Status != domaintjk.RunPartialSuccess || finished.TotalCount != 3 || finished.CreatedCount != 1 ||
		finished.UpdatedCount != 0 || finished.UnchangedCount != 0 || finished.SkippedCount != 1 ||
		finished.FailedCount != 1 || finished.ConflictCount != 1 {
		t.Fatalf("truthful counters not persisted: %#v", finished)
	}
	var horseCount, errorCount int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM hrd_horses WHERE tjk_number=$1`, number).Scan(&horseCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM hrd_tjk_sync_item_errors WHERE run_id=$1`, run.ID).Scan(&errorCount); err != nil {
		t.Fatal(err)
	}
	if horseCount != 1 || errorCount != 3 {
		t.Fatalf("horseCount=%d errorCount=%d", horseCount, errorCount)
	}
}

func TestPageHandoffRollbackRemainsRetryableIntegration(t *testing.T) {
	pool := requireTJKIntegration(t)
	actorID := seedActor(t, pool)
	number := "ITEST-ROLLBACK-" + uuid.NewString()
	cleanupTJK(t, pool, actorID, number)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
CREATE OR REPLACE FUNCTION hrd_test_reject_tjk_page_one() RETURNS trigger AS $$
BEGIN
  IF NEW.job_type='TJK_SYNC_BATCH' AND NEW.deduplication_key LIKE 'TJK_SYNC_BATCH:%:1' THEN
    RAISE EXCEPTION 'intentional TJK handoff rollback';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS hrd_test_reject_tjk_page_one ON hrd_background_jobs;
CREATE TRIGGER hrd_test_reject_tjk_page_one BEFORE INSERT ON hrd_background_jobs
FOR EACH ROW EXECUTE FUNCTION hrd_test_reject_tjk_page_one()`)
	if err != nil {
		t.Fatalf("install rollback trigger: %v", err)
	}
	dropTrigger := func() {
		_, _ = pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS hrd_test_reject_tjk_page_one ON hrd_background_jobs; DROP FUNCTION IF EXISTS hrd_test_reject_tjk_page_one()`)
	}
	t.Cleanup(dropTrigger)
	repo := pgtjk.NewRepository(pool)
	svc, _ := apptjk.NewService(apptjk.Config{Repo: repo, Enabled: true})
	run, err := svc.Trigger(ctx, actorID, "FULL", "TJK_HTTP")
	if err != nil {
		t.Fatal(err)
	}
	total := 1
	worker, err := apptjk.NewWorker(repo, sequenceFetcher{pages: map[string]domaintjk.PageResult{
		"0": {Fingerprint: "rollback-page", SourceTotal: &total, Horses: []domaintjk.HorseInput{{Number: number, Name: "ROLLBACK HORSE"}}},
		"1": {EndOfSource: true},
	}}, "integration-worker")
	if err != nil {
		t.Fatal(err)
	}
	if claimed, err := worker.ProcessOnce(ctx, time.Minute); err == nil || !claimed {
		t.Fatalf("expected handoff failure, claimed=%v err=%v", claimed, err)
	}
	var horseCount, jobCount int
	var jobStatus string
	var checkpoint []byte
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM hrd_horses WHERE tjk_number=$1`, number).Scan(&horseCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*),min(status) FROM hrd_background_jobs WHERE tjk_sync_run_id=$1`, run.ID).Scan(&jobCount, &jobStatus); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT checkpoint FROM hrd_tjk_sync_runs WHERE id=$1`, run.ID).Scan(&checkpoint); err != nil {
		t.Fatal(err)
	}
	var cp struct {
		Page int `json:"page"`
	}
	if err := json.Unmarshal(checkpoint, &cp); err != nil {
		t.Fatal(err)
	}
	if horseCount != 0 || jobCount != 1 || jobStatus != "QUEUED" || cp.Page != 0 {
		t.Fatalf("rollback horse=%d jobs=%d status=%s checkpoint=%s", horseCount, jobCount, jobStatus, checkpoint)
	}
	dropTrigger()
	if _, err := pool.Exec(ctx, `UPDATE hrd_background_jobs SET available_at=now() WHERE tjk_sync_run_id=$1 AND status='QUEUED'`, run.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if claimed, err := worker.ProcessOnce(ctx, time.Minute); err != nil || !claimed {
			t.Fatalf("retry pass %d claimed=%v err=%v", i+1, claimed, err)
		}
	}
	finished, err := repo.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if finished.Status != domaintjk.RunSucceeded || finished.CreatedCount != 1 {
		t.Fatalf("retry did not complete run: %#v", finished)
	}
}

func TestFakeHTTPFullSyncIntegration(t *testing.T) {
	pool := requireTJKIntegration(t)
	actorID := seedActor(t, pool)
	horseNumber := "ITEST-" + uuid.NewString()
	secondHorseNumber := horseNumber + "-2"
	cleanupTJK(t, pool, actorID, horseNumber, secondHorseNumber)
	firstHorseName := "LOCAL TEST HORSE"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/TR/YarisSever/Query/DataRows/Atlar":
			switch r.URL.Query().Get("PageNumber") {
			case "0":
				_, _ = fmt.Fprintf(w, `<html><body><div>Toplam 2</div><a href="?QueryParameter_AtId=%s">%s</a> ARAP <a href="?QueryParameter_BabaAdi=x">LOCAL SIRE</a><a href="?QueryParameter_AnneAdi=x">LOCAL DAM</a></body></html>`, horseNumber, firstHorseName)
			case "1":
				_, _ = fmt.Fprintf(w, `<html><body><a href="?QueryParameter_AtId=%s">LOCAL TEST HORSE TWO</a> İNGİLİZ <a href="?QueryParameter_BabaAdi=x">LOCAL SIRE TWO</a><a href="?QueryParameter_AnneAdi=x">LOCAL DAM TWO</a></body></html>`, secondHorseNumber)
			default:
				_, _ = w.Write([]byte(`<html><body><div>Toplam 0</div></body></html>`))
			}
		case "/TR/YarisSever/Query/ConnectedPage/AtKosuBilgileri":
			_, _ = w.Write([]byte(`<div class="grid_8"><span>Doğ. Trh</span><span>01.01.2020</span></div>`))
		case "/TR/YarisSever/Query/Pedigri/Pedigri", "/TR/YarisSever/Query/Kardes/Kardes":
			_, _ = w.Write([]byte(`<div class="grid_24"><table><tbody></tbody></table></div>`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	repo := pgtjk.NewRepository(pool)
	svc, err := apptjk.NewService(apptjk.Config{Repo: repo, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	run, err := svc.Trigger(context.Background(), actorID, "FULL", "TJK_HTTP")
	if err != nil {
		t.Fatal(err)
	}
	client, err := tjkhttp.NewClient(tjkhttp.Config{
		BaseURL: server.URL, HTTPTimeout: 2 * time.Second, MaxBodyBytes: 1 << 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	worker, err := apptjk.NewWorker(repo, tjkhttp.WorkerAdapter{Client: client}, "integration-worker")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		claimed, err := worker.ProcessOnce(context.Background(), time.Minute)
		if err != nil || !claimed {
			t.Fatalf("worker pass %d claimed=%v err=%v cause=%v", i+1, claimed, err, errors.Unwrap(err))
		}
	}
	finished, err := repo.GetRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if finished.Status != domaintjk.RunSucceeded || finished.TotalCount != 2 || finished.CreatedCount != 2 || finished.UpdatedCount != 0 || finished.UnchangedCount != 0 || finished.CompletedAt == nil {
		t.Fatalf("unexpected full-sync result: %#v", finished)
	}
	var horseCount int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM hrd_horses WHERE tjk_number IN ($1,$2)`, horseNumber, secondHorseNumber).Scan(&horseCount); err != nil {
		t.Fatal(err)
	}
	if horseCount != 2 {
		t.Fatalf("unique persisted horses=%d", horseCount)
	}
	var succeededJobs int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM hrd_background_jobs WHERE tjk_sync_run_id=$1 AND status='SUCCEEDED'`, run.ID).Scan(&succeededJobs); err != nil {
		t.Fatal(err)
	}
	if succeededJobs != 3 {
		t.Fatalf("succeeded TJK jobs=%d", succeededJobs)
	}

	secondRun, err := svc.Trigger(context.Background(), actorID, "FULL", "TJK_HTTP")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		claimed, err := worker.ProcessOnce(context.Background(), time.Minute)
		if err != nil || !claimed {
			t.Fatalf("rerun worker pass %d claimed=%v err=%v", i+1, claimed, err)
		}
	}
	rerun, err := repo.GetRun(context.Background(), secondRun.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rerun.Status != domaintjk.RunSucceeded || rerun.TotalCount != 2 || rerun.UnchangedCount != 2 || rerun.CreatedCount != 0 || rerun.UpdatedCount != 0 {
		t.Fatalf("unexpected idempotent rerun: %#v", rerun)
	}

	firstHorseName = "LOCAL TEST HORSE UPDATED"
	thirdRun, err := svc.Trigger(context.Background(), actorID, "FULL", "TJK_HTTP")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		claimed, err := worker.ProcessOnce(context.Background(), time.Minute)
		if err != nil || !claimed {
			t.Fatalf("updated rerun worker pass %d claimed=%v err=%v", i+1, claimed, err)
		}
	}
	updatedRun, err := repo.GetRun(context.Background(), thirdRun.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedRun.Status != domaintjk.RunSucceeded || updatedRun.TotalCount != 2 ||
		updatedRun.CreatedCount != 0 || updatedRun.UpdatedCount != 1 || updatedRun.UnchangedCount != 1 {
		t.Fatalf("unexpected updated rerun: %#v", updatedRun)
	}
}
