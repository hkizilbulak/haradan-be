package tjk_test

import (
	"context"
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

func cleanupTJK(t *testing.T, pool *pgxpool.Pool, actorID uuid.UUID, horseNumber string) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM hrd_background_jobs WHERE tjk_sync_run_id IN (SELECT id FROM hrd_tjk_sync_runs WHERE created_by_user_id=$1)`, actorID)
		_, _ = pool.Exec(ctx, `DELETE FROM hrd_tjk_sync_runs WHERE created_by_user_id=$1`, actorID)
		if horseNumber != "" {
			_, _ = pool.Exec(ctx, `DELETE FROM hrd_horses WHERE tjk_number=$1`, horseNumber)
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
	if err := pool.QueryRow(context.Background(), `SELECT status,cancel_requested_at FROM hrd_background_jobs WHERE id=$1`, jobID).Scan(&jobStatus, &cancelRequested); err != nil {
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

func TestFakeHTTPFullSyncIntegration(t *testing.T) {
	pool := requireTJKIntegration(t)
	actorID := seedActor(t, pool)
	horseNumber := "ITEST-" + uuid.NewString()
	cleanupTJK(t, pool, actorID, horseNumber)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/TR/YarisSever/Query/DataRows/Atlar":
			if r.URL.Query().Get("PageNumber") == "0" {
				_, _ = fmt.Fprintf(w, `<html><body><a href="?QueryParameter_AtId=%s">LOCAL TEST HORSE</a> ARAP <a href="?QueryParameter_BabaAdi=x">LOCAL SIRE</a><a href="?QueryParameter_AnneAdi=x">LOCAL DAM</a></body></html>`, horseNumber)
				return
			}
			_, _ = w.Write([]byte(`<html><body></body></html>`))
		case "/TR/YarisSever/Query/ConnectedPage/AtKosuBilgileri":
			_, _ = w.Write([]byte(`<div class="grid_8"><span>Doğ. Trh</span><span>01.01.2020</span></div>`))
		case "/TR/YarisSever/Query/Pedigri/Pedigri", "/TR/YarisSever/Query/Kardes/Kardes":
			_, _ = w.Write([]byte(`<html><body></body></html>`))
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
	for i := 0; i < 2; i++ {
		claimed, err := worker.ProcessOnce(context.Background(), time.Minute)
		if err != nil || !claimed {
			t.Fatalf("worker pass %d claimed=%v err=%v cause=%v", i+1, claimed, err, errors.Unwrap(err))
		}
	}
	finished, err := repo.GetRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if finished.Status != domaintjk.RunSucceeded || finished.TotalCount != 1 || finished.CompletedAt == nil {
		t.Fatalf("unexpected full-sync result: %#v", finished)
	}
	var name string
	var birthYear *int
	if err := pool.QueryRow(context.Background(), `SELECT original_name,birth_year FROM hrd_horses WHERE tjk_number=$1`, horseNumber).Scan(&name, &birthYear); err != nil {
		t.Fatal(err)
	}
	if name != "LOCAL TEST HORSE" || birthYear == nil || *birthYear != 2020 {
		t.Fatalf("horse upsert name=%q birthYear=%v", name, birthYear)
	}
	var succeededJobs int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM hrd_background_jobs WHERE tjk_sync_run_id=$1 AND status='SUCCEEDED'`, run.ID).Scan(&succeededJobs); err != nil {
		t.Fatal(err)
	}
	if succeededJobs != 2 {
		t.Fatalf("succeeded TJK jobs=%d", succeededJobs)
	}
}
