package media_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	pgmedia "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/media"
)

func requireJobQueueIntegration(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if strings.TrimSpace(os.Getenv("RUN_JOB_QUEUE_INTEGRATION_TESTS")) != "1" {
		t.Skip("RUN_JOB_QUEUE_INTEGRATION_TESTS!=1; skipping job queue integration tests")
	}
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping job queue integration tests")
	}
	if strings.TrimSpace(os.Getenv("DATABASE_URL")) != "" && dsn == strings.TrimSpace(os.Getenv("DATABASE_URL")) {
		t.Fatalf("TEST_DATABASE_URL must not equal DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestJobQueueClaimIntegrationOptIn(t *testing.T) {
	pool := requireJobQueueIntegration(t)
	ctx := context.Background()
	repo := pgmedia.NewRepository(pool)
	queue, err := appmedia.NewPostgresJobQueue(pool)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	key := "itest-claim-" + uuid.NewString()
	job := domainmedia.BackgroundJob{
		ID:               uuid.New(),
		JobType:          domainmedia.JobValidateAndNormalize,
		Status:           domainmedia.JobQueued,
		Payload:          domainmedia.EmptyMetadata(),
		DeduplicationKey: &key,
		MaxAttempts:      3,
		AvailableAt:      now,
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := repo.EnqueueJob(ctx, job); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM hrd_background_jobs WHERE id = $1`, job.ID)
	})

	claimed, ok, err := queue.ClaimNextJob(ctx, appmedia.ClaimJobParams{
		LeaseOwner:     "itest-worker",
		Now:            now,
		LeaseUntil:     now.Add(2 * time.Minute),
		SupportedTypes: []domainmedia.JobType{domainmedia.JobValidateAndNormalize},
	})
	if err != nil || !ok {
		t.Fatalf("claim ok=%v err=%v", ok, err)
	}
	if claimed.ID != job.ID || claimed.Status != domainmedia.JobLeased || claimed.AttemptCount != 1 {
		t.Fatalf("claimed=%+v", claimed)
	}

	_, ok, err = queue.ClaimNextJob(ctx, appmedia.ClaimJobParams{
		LeaseOwner:     "other",
		Now:            now,
		LeaseUntil:     now.Add(time.Minute),
		SupportedTypes: []domainmedia.JobType{domainmedia.JobValidateAndNormalize},
	})
	if err != nil || ok {
		t.Fatalf("second claim should miss, ok=%v err=%v", ok, err)
	}

	if err := queue.MarkJobSucceeded(ctx, appmedia.JobLeaseGuard{
		JobID: claimed.ID, LeaseOwner: "wrong", Version: claimed.Version,
	}, now); err == nil {
		t.Fatal("wrong owner must fail")
	} else if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeInvalidState {
		t.Fatalf("want INVALID_STATE got %v", err)
	}

	if err := queue.MarkJobSucceeded(ctx, appmedia.JobLeaseGuard{
		JobID: claimed.ID, LeaseOwner: "itest-worker", Version: claimed.Version,
	}, now); err != nil {
		t.Fatal(err)
	}
}

func TestJobQueueConcurrentClaimIntegrationOptIn(t *testing.T) {
	pool := requireJobQueueIntegration(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := pgmedia.NewRepository(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	key := "itest-concurrent-" + uuid.NewString()
	job := domainmedia.BackgroundJob{
		ID:               uuid.New(),
		JobType:          domainmedia.JobValidateAndNormalize,
		Status:           domainmedia.JobQueued,
		Payload:          domainmedia.EmptyMetadata(),
		DeduplicationKey: &key,
		MaxAttempts:      3,
		AvailableAt:      now,
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := repo.EnqueueJob(ctx, job); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM hrd_background_jobs WHERE id = $1`, job.ID)
	})

	queueA, err := appmedia.NewPostgresJobQueue(pool)
	if err != nil {
		t.Fatal(err)
	}
	queueB, err := appmedia.NewPostgresJobQueue(pool)
	if err != nil {
		t.Fatal(err)
	}

	type result struct {
		ok  bool
		id  uuid.UUID
		err error
	}
	ch := make(chan result, 2)
	claim := func(q appmedia.JobQueue, owner string) {
		claimed, ok, err := q.ClaimNextJob(ctx, appmedia.ClaimJobParams{
			LeaseOwner:     owner,
			Now:            now,
			LeaseUntil:     now.Add(2 * time.Minute),
			SupportedTypes: []domainmedia.JobType{domainmedia.JobValidateAndNormalize},
		})
		if err != nil || !ok {
			ch <- result{ok: ok, err: err}
			return
		}
		ch <- result{ok: true, id: claimed.ID}
	}
	go claim(queueA, "worker-a")
	go claim(queueB, "worker-b")

	var winners int
	for i := 0; i < 2; i++ {
		select {
		case <-ctx.Done():
			t.Fatal("concurrent claim timed out")
		case r := <-ch:
			if r.err != nil {
				t.Fatalf("claim error: %v", r.err)
			}
			if r.ok {
				winners++
				if r.id != job.ID {
					t.Fatalf("unexpected job id %s", r.id)
				}
			}
		}
	}
	if winners != 1 {
		t.Fatalf("want exactly one concurrent winner, got %d", winners)
	}
}
