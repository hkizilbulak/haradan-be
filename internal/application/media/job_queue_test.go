package media_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

func queuedJob(jobType domainmedia.JobType, availableAt time.Time, maxAttempts int) domainmedia.BackgroundJob {
	key := uuid.NewString()
	return domainmedia.BackgroundJob{
		ID:               uuid.New(),
		JobType:          jobType,
		Status:           domainmedia.JobQueued,
		Payload:          domainmedia.EmptyMetadata(),
		DeduplicationKey: &key,
		AttemptCount:     0,
		MaxAttempts:      maxAttempts,
		AvailableAt:      availableAt,
		Version:          1,
		CreatedAt:        availableAt,
		UpdatedAt:        availableAt,
	}
}

func TestMemoryJobQueueClaimAndLifecycle(t *testing.T) {
	store := appmedia.NewMemoryStore()
	q := store.JobQueue()
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	job := queuedJob(domainmedia.JobValidateAndNormalize, now, 3)
	if err := store.Repo().EnqueueJob(context.Background(), job); err != nil {
		t.Fatal(err)
	}

	// Future available_at skipped.
	future := queuedJob(domainmedia.JobGenerateVariant, now.Add(time.Hour), 3)
	if err := store.Repo().EnqueueJob(context.Background(), future); err != nil {
		t.Fatal(err)
	}
	// Unsupported type skipped.
	unsupported := queuedJob(domainmedia.JobDeleteObjects, now, 3)
	if err := store.Repo().EnqueueJob(context.Background(), unsupported); err != nil {
		t.Fatal(err)
	}
	// Cancel requested skipped.
	cancel := queuedJob(domainmedia.JobGenerateVariant, now, 3)
	cancelAt := now
	cancel.CancelRequestedAt = &cancelAt
	if err := store.Repo().EnqueueJob(context.Background(), cancel); err != nil {
		t.Fatal(err)
	}
	// Max attempts skipped.
	exhausted := queuedJob(domainmedia.JobGenerateVariant, now, 2)
	exhausted.AttemptCount = 2
	if err := store.Repo().EnqueueJob(context.Background(), exhausted); err != nil {
		t.Fatal(err)
	}

	claimed, ok, err := q.ClaimNextJob(context.Background(), appmedia.ClaimJobParams{
		LeaseOwner: "w1",
		Now:        now,
		LeaseUntil: now.Add(2 * time.Minute),
		SupportedTypes: []domainmedia.JobType{
			domainmedia.JobValidateAndNormalize,
			domainmedia.JobGenerateVariant,
		},
	})
	if err != nil || !ok {
		t.Fatalf("claim ok=%v err=%v", ok, err)
	}
	if claimed.ID != job.ID || claimed.Status != domainmedia.JobLeased || claimed.AttemptCount != 1 {
		t.Fatalf("claimed=%+v", claimed)
	}
	if claimed.LeaseOwner == nil || *claimed.LeaseOwner != "w1" || claimed.Version != 2 {
		t.Fatalf("lease fields=%+v", claimed)
	}

	// Empty claim when only ineligible remain.
	_, ok, err = q.ClaimNextJob(context.Background(), appmedia.ClaimJobParams{
		LeaseOwner: "w2",
		Now:        now,
		LeaseUntil: now.Add(time.Minute),
		SupportedTypes: []domainmedia.JobType{
			domainmedia.JobValidateAndNormalize,
			domainmedia.JobGenerateVariant,
		},
	})
	if err != nil || ok {
		t.Fatalf("expected no claim, ok=%v err=%v", ok, err)
	}

	guard := appmedia.JobLeaseGuard{JobID: claimed.ID, LeaseOwner: "w1", Version: claimed.Version}
	if err := q.MarkJobSucceeded(context.Background(), guard, now); err != nil {
		t.Fatal(err)
	}
	jobs := store.Jobs()
	var found domainmedia.BackgroundJob
	for _, j := range jobs {
		if j.ID == claimed.ID {
			found = j
		}
	}
	if found.Status != domainmedia.JobSucceeded || found.CompletedAt == nil || found.LeaseOwner != nil {
		t.Fatalf("succeeded=%+v", found)
	}
}

func TestMemoryJobQueueRetryDeadAndGuards(t *testing.T) {
	store := appmedia.NewMemoryStore()
	q := store.JobQueue()
	now := time.Now().UTC()
	job := queuedJob(domainmedia.JobGenerateVariant, now, 2)
	if err := store.Repo().EnqueueJob(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	claimed, ok, err := q.ClaimNextJob(context.Background(), appmedia.ClaimJobParams{
		LeaseOwner:     "w1",
		Now:            now,
		LeaseUntil:     now.Add(time.Minute),
		SupportedTypes: []domainmedia.JobType{domainmedia.JobGenerateVariant},
	})
	if err != nil || !ok {
		t.Fatal(err)
	}
	guard := appmedia.JobLeaseGuard{JobID: claimed.ID, LeaseOwner: "wrong", Version: claimed.Version}
	if err := q.MarkJobFailed(context.Background(), guard, now, "x"); err == nil {
		t.Fatal("wrong owner must fail")
	} else if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeInvalidState {
		t.Fatalf("want INVALID_STATE, got %v", err)
	}

	guard.LeaseOwner = "w1"
	next := now.Add(5 * time.Second)
	if err := q.RetryOrDeadLetterJob(context.Background(), appmedia.RetryJobParams{
		JobLeaseGuard:   guard,
		Now:             now,
		NextAvailableAt: next,
		LastError:       "transient",
		AttemptCount:    claimed.AttemptCount,
		MaxAttempts:     claimed.MaxAttempts,
	}); err != nil {
		t.Fatal(err)
	}

	claimed2, ok, err := q.ClaimNextJob(context.Background(), appmedia.ClaimJobParams{
		LeaseOwner:     "w1",
		Now:            next,
		LeaseUntil:     next.Add(time.Minute),
		SupportedTypes: []domainmedia.JobType{domainmedia.JobGenerateVariant},
	})
	if err != nil || !ok {
		t.Fatal("retry should be claimable")
	}
	if err := q.RetryOrDeadLetterJob(context.Background(), appmedia.RetryJobParams{
		JobLeaseGuard: appmedia.JobLeaseGuard{JobID: claimed2.ID, LeaseOwner: "w1", Version: claimed2.Version},
		Now:           next,
		LastError:     "still failing",
		AttemptCount:  claimed2.AttemptCount,
		MaxAttempts:   claimed2.MaxAttempts,
	}); err != nil {
		t.Fatal(err)
	}
	for _, j := range store.Jobs() {
		if j.ID == claimed2.ID && j.Status != domainmedia.JobDead {
			t.Fatalf("want DEAD, got %s", j.Status)
		}
	}
}

func TestMemoryJobQueueExpiredLeaseRecovery(t *testing.T) {
	store := appmedia.NewMemoryStore()
	q := store.JobQueue()
	now := time.Now().UTC()
	job := queuedJob(domainmedia.JobValidateAndNormalize, now, 2)
	if err := store.Repo().EnqueueJob(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	claimed, ok, _ := q.ClaimNextJob(context.Background(), appmedia.ClaimJobParams{
		LeaseOwner:     "w1",
		Now:            now,
		LeaseUntil:     now.Add(time.Second),
		SupportedTypes: []domainmedia.JobType{domainmedia.JobValidateAndNormalize},
	})
	if !ok {
		t.Fatal("expected claim")
	}
	later := now.Add(2 * time.Second)
	n, err := q.RecoverExpiredJobLeases(context.Background(), later, 10)
	if err != nil || n != 1 {
		t.Fatalf("recovered=%d err=%v", n, err)
	}
	for _, j := range store.Jobs() {
		if j.ID == claimed.ID {
			if j.Status != domainmedia.JobQueued || j.LeaseOwner != nil {
				t.Fatalf("recovered job=%+v", j)
			}
		}
	}

	// Exhaust then expire → DEAD
	claimed2, ok, _ := q.ClaimNextJob(context.Background(), appmedia.ClaimJobParams{
		LeaseOwner:     "w1",
		Now:            later,
		LeaseUntil:     later.Add(time.Second),
		SupportedTypes: []domainmedia.JobType{domainmedia.JobValidateAndNormalize},
	})
	if !ok {
		t.Fatal("expected second claim")
	}
	// attempt now 2 == max → recovery DEAD
	n, err = q.RecoverExpiredJobLeases(context.Background(), later.Add(2*time.Second), 10)
	if err != nil || n != 1 {
		t.Fatalf("recovered=%d err=%v", n, err)
	}
	for _, j := range store.Jobs() {
		if j.ID == claimed2.ID && j.Status != domainmedia.JobDead {
			t.Fatalf("want DEAD got %s", j.Status)
		}
	}
}

func TestMemoryJobQueueConcurrentClaims(t *testing.T) {
	store := appmedia.NewMemoryStore()
	q := store.JobQueue()
	now := time.Now().UTC()
	job := queuedJob(domainmedia.JobValidateAndNormalize, now, 5)
	if err := store.Repo().EnqueueJob(context.Background(), job); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var winners []uuid.UUID
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(owner string) {
			defer wg.Done()
			claimed, ok, err := q.ClaimNextJob(context.Background(), appmedia.ClaimJobParams{
				LeaseOwner:     owner,
				Now:            now,
				LeaseUntil:     now.Add(time.Minute),
				SupportedTypes: []domainmedia.JobType{domainmedia.JobValidateAndNormalize},
			})
			if err != nil {
				t.Errorf("claim: %v", err)
				return
			}
			if ok {
				mu.Lock()
				winners = append(winners, claimed.ID)
				mu.Unlock()
			}
		}(uuid.NewString())
	}
	wg.Wait()
	if len(winners) != 1 {
		t.Fatalf("want exactly one winner, got %d", len(winners))
	}
}
