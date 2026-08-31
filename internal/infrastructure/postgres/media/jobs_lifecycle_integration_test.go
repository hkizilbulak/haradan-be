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
		ID: uuid.New(),
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
		ID: uuid.New(),
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

func TestJobQueueFailedDeadReenqueueIntegrationOptIn(t *testing.T) {
	pool := requireJobQueueIntegration(t)
	ctx := context.Background()
	repo := pgmedia.NewRepository(pool)
	queue, err := appmedia.NewPostgresJobQueue(pool)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	key := "itest-reenqueue-" + uuid.NewString()
	job := domainmedia.BackgroundJob{
		ID: uuid.New(),
		JobType:          domainmedia.JobValidateAndNormalize,
		Status:           domainmedia.JobQueued,
		Payload:          domainmedia.EmptyMetadata(),
		DeduplicationKey: &key,
		MaxAttempts:      1,
		AvailableAt:      now,
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := repo.EnqueueJob(ctx, job); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM hrd_background_jobs WHERE deduplication_key = $1`, key)
	})

	claimed, ok, err := queue.ClaimNextJob(ctx, appmedia.ClaimJobParams{
		LeaseOwner:     "itest-fail",
		Now:            now,
		LeaseUntil:     now.Add(2 * time.Minute),
		SupportedTypes: []domainmedia.JobType{domainmedia.JobValidateAndNormalize},
	})
	if err != nil || !ok {
		t.Fatalf("claim ok=%v err=%v", ok, err)
	}
	if err := queue.MarkJobFailed(ctx, appmedia.JobLeaseGuard{
		JobID: claimed.ID, LeaseOwner: "itest-fail", Version: claimed.Version,
	}, now, "boom"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	// Active duplicate of a FAILED row must be allowed (new row, same occurrence key).
	retry := job
	retry.ID = uuid.New()
	retry.CreatedAt = now.Add(time.Second)
	retry.UpdatedAt = retry.CreatedAt
	retry.AvailableAt = retry.CreatedAt
	if err := repo.EnqueueJob(ctx, retry); err != nil {
		t.Fatalf("FAILED terminal must allow re-enqueue: %v", err)
	}

	// Duplicate active (QUEUED) still blocked.
	dupActive := retry
	dupActive.ID = uuid.New()
	if err := repo.EnqueueJob(ctx, dupActive); err == nil {
		t.Fatal("duplicate active dedup key must fail")
	} else if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeConflict {
		t.Fatalf("want CONFLICT, got %v", err)
	}

	claimed2, ok, err := queue.ClaimNextJob(ctx, appmedia.ClaimJobParams{
		LeaseOwner:     "itest-dead",
		Now:            now.Add(2 * time.Second),
		LeaseUntil:     now.Add(4 * time.Minute),
		SupportedTypes: []domainmedia.JobType{domainmedia.JobValidateAndNormalize},
	})
	if err != nil || !ok {
		t.Fatalf("claim retry ok=%v err=%v", ok, err)
	}
	if claimed2.ID != retry.ID {
		t.Fatalf("claimed retry id=%s want %s", claimed2.ID, retry.ID)
	}
	if err := queue.RetryOrDeadLetterJob(ctx, appmedia.RetryJobParams{
		JobLeaseGuard: appmedia.JobLeaseGuard{
			JobID: claimed2.ID, LeaseOwner: "itest-dead", Version: claimed2.Version,
		},
		Now:             now.Add(2 * time.Second),
		NextAvailableAt: now.Add(2 * time.Second),
		LastError:       "dead",
		AttemptCount:    claimed2.AttemptCount,
		MaxAttempts:     claimed2.MaxAttempts,
	}); err != nil {
		t.Fatalf("dead-letter: %v", err)
	}

	afterDead := job
	afterDead.ID = uuid.New()
	afterDead.CreatedAt = now.Add(3 * time.Second)
	afterDead.UpdatedAt = afterDead.CreatedAt
	afterDead.AvailableAt = afterDead.CreatedAt
	if err := repo.EnqueueJob(ctx, afterDead); err != nil {
		t.Fatalf("DEAD terminal must allow re-enqueue: %v", err)
	}
}

func TestJobQueueSucceededBlocksSameDedupIntegrationOptIn(t *testing.T) {
	pool := requireJobQueueIntegration(t)
	ctx := context.Background()
	repo := pgmedia.NewRepository(pool)
	queue, err := appmedia.NewPostgresJobQueue(pool)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	key := "itest-succeeded-" + uuid.NewString()
	job := domainmedia.BackgroundJob{
		ID: uuid.New(),
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
		_, _ = pool.Exec(context.Background(), `DELETE FROM hrd_background_jobs WHERE deduplication_key = $1`, key)
	})

	claimed, ok, err := queue.ClaimNextJob(ctx, appmedia.ClaimJobParams{
		LeaseOwner:     "itest-ok",
		Now:            now,
		LeaseUntil:     now.Add(2 * time.Minute),
		SupportedTypes: []domainmedia.JobType{domainmedia.JobValidateAndNormalize},
	})
	if err != nil || !ok {
		t.Fatalf("claim ok=%v err=%v", ok, err)
	}
	if err := queue.MarkJobSucceeded(ctx, appmedia.JobLeaseGuard{
		JobID: claimed.ID, LeaseOwner: "itest-ok", Version: claimed.Version,
	}, now); err != nil {
		t.Fatalf("mark succeeded: %v", err)
	}

	dup := job
	dup.ID = uuid.New()
	if err := repo.EnqueueJob(ctx, dup); err == nil {
		t.Fatal("SUCCEEDED occurrence must still block same dedup key")
	} else if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeConflict {
		t.Fatalf("want CONFLICT, got %v", err)
	}
}
