package worker_test

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	appworker "github.com/hkizilbulak/haradan-be/internal/application/worker"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

type stubHandler struct {
	validate  func(context.Context, uuid.UUID) error
	variant   func(context.Context, uuid.UUID, string) error
	reconcile func(context.Context, []byte) error
}

func (s stubHandler) ProcessValidateAndNormalize(ctx context.Context, id uuid.UUID) error {
	if s.validate != nil {
		return s.validate(ctx, id)
	}
	return nil
}

func (s stubHandler) ProcessGenerateVariant(ctx context.Context, id uuid.UUID, profile string) error {
	if s.variant != nil {
		return s.variant(ctx, id, profile)
	}
	return nil
}

func (s stubHandler) ProcessDeleteObjects(context.Context, []byte) error { return nil }
func (s stubHandler) ProcessReconcile(ctx context.Context, payload []byte) error {
	if s.reconcile != nil {
		return s.reconcile(ctx, payload)
	}
	return nil
}

func enqueueValidate(t *testing.T, store *appmedia.MemoryStore, assetID uuid.UUID, now time.Time) {
	t.Helper()
	key := domainmedia.ValidateJobDedupKey(assetID)
	payload, _ := json.Marshal(map[string]string{"assetId": assetID.String()})
	job := domainmedia.BackgroundJob{
		ID:               uuid.New(),
		JobType:          domainmedia.JobValidateAndNormalize,
		Status:           domainmedia.JobQueued,
		Payload:          payload,
		DeduplicationKey: &key,
		MaxAttempts:      3,
		AvailableAt:      now,
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := store.Repo().EnqueueJob(context.Background(), job); err != nil {
		t.Fatal(err)
	}
}

func TestRunnerClaimSuccessAndPermanentFailure(t *testing.T) {
	store := appmedia.NewMemoryStore()
	now := time.Now().UTC()
	assetOK := uuid.New()
	assetBad := uuid.New()
	enqueueValidate(t, store, assetOK, now)
	enqueueValidate(t, store, assetBad, now)

	var calls atomic.Int32
	handler := stubHandler{
		validate: func(_ context.Context, id uuid.UUID) error {
			calls.Add(1)
			if id == assetBad {
				return apperr.Validation("bad image")
			}
			return nil
		},
	}

	clockNow := now
	runner, err := appworker.NewRunner(appworker.Config{
		WorkerID:              "test-worker",
		Concurrency:           1,
		PollInterval:          20 * time.Millisecond,
		LeaseDuration:         2 * time.Minute,
		JobTimeout:            5 * time.Second,
		ShutdownTimeout:       time.Second,
		RetryBaseDelay:        time.Second,
		RetryMaxDelay:         time.Minute,
		LeaseRecoveryInterval: time.Hour,
		Queue:                 store.JobQueue(),
		Handler:               handler,
		Clock:                 func() time.Time { return clockNow },
		Backoff:               appworker.Backoff{Base: time.Second, Max: time.Minute, Float63: func() float64 { return 1 }},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()

	deadline := time.After(2 * time.Second)
	for {
		succeeded, failed := 0, 0
		for _, j := range store.Jobs() {
			switch j.Status {
			case domainmedia.JobSucceeded:
				succeeded++
			case domainmedia.JobFailed:
				failed++
			}
		}
		if succeeded == 1 && failed == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout statuses succeeded=%d failed=%d calls=%d jobs=%v", succeeded, failed, calls.Load(), store.Jobs())
		case <-time.After(20 * time.Millisecond):
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRunnerTransientRetryAndUnsupportedNotClaimed(t *testing.T) {
	store := appmedia.NewMemoryStore()
	now := time.Now().UTC()
	assetID := uuid.New()
	enqueueValidate(t, store, assetID, now)

	key := uuid.NewString()
	deleteJob := domainmedia.BackgroundJob{
		ID:               uuid.New(),
		JobType:          domainmedia.JobDeleteObjects,
		Status:           domainmedia.JobQueued,
		Payload:          domainmedia.EmptyMetadata(),
		DeduplicationKey: &key,
		MaxAttempts:      3,
		AvailableAt:      now,
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := store.Repo().EnqueueJob(context.Background(), deleteJob); err != nil {
		t.Fatal(err)
	}

	var attempts atomic.Int32
	handler := stubHandler{
		validate: func(context.Context, uuid.UUID) error {
			attempts.Add(1)
			return apperr.DependencyUnavailable("tinify down")
		},
	}

	runner, err := appworker.NewRunner(appworker.Config{
		WorkerID:              "test-worker",
		Concurrency:           1,
		PollInterval:          10 * time.Millisecond,
		LeaseDuration:         time.Minute,
		JobTimeout:            2 * time.Second,
		ShutdownTimeout:       time.Second,
		RetryBaseDelay:        time.Millisecond,
		RetryMaxDelay:         10 * time.Millisecond,
		LeaseRecoveryInterval: time.Hour,
		Queue:                 store.JobQueue(),
		Handler:               handler,
		Clock:                 func() time.Time { return time.Now().UTC() },
		Backoff:               appworker.Backoff{Base: time.Millisecond, Max: 5 * time.Millisecond, Float63: func() float64 { return 1 }},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()

	deadline := time.After(2 * time.Second)
	for {
		var dead bool
		for _, j := range store.Jobs() {
			if j.ID == deleteJob.ID && j.Status != domainmedia.JobQueued {
				t.Fatalf("unsupported job was claimed: %+v", j)
			}
			if j.JobType == domainmedia.JobValidateAndNormalize && j.Status == domainmedia.JobDead {
				dead = true
			}
		}
		if dead {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout attempts=%d jobs=%v", attempts.Load(), store.Jobs())
		case <-time.After(20 * time.Millisecond):
		}
	}
	cancel()
	<-done
}

func TestRunnerPanicRecovery(t *testing.T) {
	store := appmedia.NewMemoryStore()
	now := time.Now().UTC()
	assetID := uuid.New()
	enqueueValidate(t, store, assetID, now)

	handler := stubHandler{
		validate: func(context.Context, uuid.UUID) error {
			panic("boom")
		},
	}
	runner, err := appworker.NewRunner(appworker.Config{
		WorkerID:              "panic-worker",
		Concurrency:           1,
		PollInterval:          10 * time.Millisecond,
		LeaseDuration:         time.Minute,
		JobTimeout:            time.Second,
		ShutdownTimeout:       time.Second,
		RetryBaseDelay:        time.Millisecond,
		RetryMaxDelay:         time.Millisecond,
		LeaseRecoveryInterval: time.Hour,
		Queue:                 store.JobQueue(),
		Handler:               handler,
		Clock:                 func() time.Time { return time.Now().UTC() },
		Backoff:               appworker.Backoff{Base: time.Millisecond, Max: time.Millisecond, Float63: func() float64 { return 1 }},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()

	deadline := time.After(2 * time.Second)
	for {
		for _, j := range store.Jobs() {
			if j.Status == domainmedia.JobQueued || j.Status == domainmedia.JobDead {
				cancel()
				<-done
				return
			}
		}
		select {
		case <-deadline:
			t.Fatal("panic did not recover into retry/dead")
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestRunnerDefinitionTimeoutCancelsSlowHandler(t *testing.T) {
	store := appmedia.NewMemoryStore()
	now := time.Now().UTC()
	key := "timeout-def-" + uuid.NewString()
	payload, _ := json.Marshal(map[string]any{"timeoutSeconds": 1})
	job := domainmedia.BackgroundJob{
		ID:               uuid.New(),
		JobType:          domainmedia.JobReconcile,
		Status:           domainmedia.JobQueued,
		Payload:          payload,
		DeduplicationKey: &key,
		MaxAttempts:      1,
		AvailableAt:      now,
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := store.Repo().EnqueueJob(context.Background(), job); err != nil {
		t.Fatal(err)
	}

	var sawCancel atomic.Bool
	handler := stubHandler{
		reconcile: func(ctx context.Context, _ []byte) error {
			select {
			case <-ctx.Done():
				sawCancel.Store(true)
				return ctx.Err()
			case <-time.After(5 * time.Second):
				return nil
			}
		},
	}

	runner, err := appworker.NewRunner(appworker.Config{
		WorkerID:              "timeout-worker",
		Concurrency:           1,
		PollInterval:          10 * time.Millisecond,
		LeaseDuration:         3 * time.Hour,
		JobTimeout:            60 * time.Second, // global default must not win over payload=1s
		MaxJobTimeout:         2 * time.Hour,
		ShutdownTimeout:       time.Second,
		RetryBaseDelay:        time.Millisecond,
		RetryMaxDelay:         time.Millisecond,
		LeaseRecoveryInterval: time.Hour,
		SupportedJobTypes:     []domainmedia.JobType{domainmedia.JobReconcile},
		Queue:                 store.JobQueue(),
		Handler:               handler,
		Clock:                 func() time.Time { return time.Now().UTC() },
		Backoff:               appworker.Backoff{Base: time.Millisecond, Max: time.Millisecond, Float63: func() float64 { return 1 }},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()

	deadline := time.After(4 * time.Second)
	for {
		for _, j := range store.Jobs() {
			if j.AttemptCount > 0 && (j.Status == domainmedia.JobDead || j.Status == domainmedia.JobFailed || j.Status == domainmedia.JobQueued) {
				cancel()
				<-done
				if !sawCancel.Load() {
					t.Fatal("handler context was not cancelled by definition timeout")
				}
				return
			}
		}
		select {
		case <-deadline:
			cancel()
			<-done
			t.Fatalf("definition timeout did not finalize job: %+v", store.Jobs())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestRunnerEventDrivenUsesGlobalDefaultTimeout(t *testing.T) {
	store := appmedia.NewMemoryStore()
	now := time.Now().UTC()
	key := "timeout-event-" + uuid.NewString()
	payload := domainmedia.EmptyMetadata()
	job := domainmedia.BackgroundJob{
		ID:               uuid.New(),
		JobType:          domainmedia.JobReconcile,
		Status:           domainmedia.JobQueued,
		Payload:          payload,
		DeduplicationKey: &key,
		MaxAttempts:      1,
		AvailableAt:      now,
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := store.Repo().EnqueueJob(context.Background(), job); err != nil {
		t.Fatal(err)
	}

	var sawCancel atomic.Bool
	handler := stubHandler{
		reconcile: func(ctx context.Context, _ []byte) error {
			select {
			case <-ctx.Done():
				sawCancel.Store(true)
				return ctx.Err()
			case <-time.After(5 * time.Second):
				return nil
			}
		},
	}

	runner, err := appworker.NewRunner(appworker.Config{
		WorkerID:              "event-timeout-worker",
		Concurrency:           1,
		PollInterval:          10 * time.Millisecond,
		LeaseDuration:         5 * time.Second,
		JobTimeout:            100 * time.Millisecond,
		MaxJobTimeout:         2 * time.Second,
		ShutdownTimeout:       time.Second,
		RetryBaseDelay:        time.Millisecond,
		RetryMaxDelay:         time.Millisecond,
		LeaseRecoveryInterval: time.Hour,
		SupportedJobTypes:     []domainmedia.JobType{domainmedia.JobReconcile},
		Queue:                 store.JobQueue(),
		Handler:               handler,
		Clock:                 func() time.Time { return time.Now().UTC() },
		Backoff:               appworker.Backoff{Base: time.Millisecond, Max: time.Millisecond, Float63: func() float64 { return 1 }},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()

	deadline := time.After(2 * time.Second)
	for {
		for _, j := range store.Jobs() {
			if j.AttemptCount > 0 && (j.Status == domainmedia.JobDead || j.Status == domainmedia.JobFailed || j.Status == domainmedia.JobQueued) {
				cancel()
				<-done
				if !sawCancel.Load() {
					t.Fatal("event-driven job must cancel on global JobTimeout")
				}
				return
			}
		}
		select {
		case <-deadline:
			cancel()
			<-done
			t.Fatalf("event-driven timeout did not finalize: %+v", store.Jobs())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestRunnerLongerDefinitionTimeoutNotClampedToGlobalDefault(t *testing.T) {
	store := appmedia.NewMemoryStore()
	now := time.Now().UTC()
	key := "timeout-long-" + uuid.NewString()
	// Representative of seeded 1800s: payload timeout longer than global default.
	payload, _ := json.Marshal(map[string]any{"timeoutSeconds": 2})
	job := domainmedia.BackgroundJob{
		ID:               uuid.New(),
		JobType:          domainmedia.JobReconcile,
		Status:           domainmedia.JobQueued,
		Payload:          payload,
		DeduplicationKey: &key,
		MaxAttempts:      1,
		AvailableAt:      now,
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := store.Repo().EnqueueJob(context.Background(), job); err != nil {
		t.Fatal(err)
	}

	handler := stubHandler{
		reconcile: func(ctx context.Context, _ []byte) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
				return nil
			}
		},
	}

	runner, err := appworker.NewRunner(appworker.Config{
		WorkerID:              "long-timeout-worker",
		Concurrency:           1,
		PollInterval:          10 * time.Millisecond,
		LeaseDuration:         10 * time.Second,
		JobTimeout:            50 * time.Millisecond, // would cut short if wrongly used as ceiling
		MaxJobTimeout:         5 * time.Second,
		ShutdownTimeout:       time.Second,
		RetryBaseDelay:        time.Millisecond,
		RetryMaxDelay:         time.Millisecond,
		LeaseRecoveryInterval: time.Hour,
		SupportedJobTypes:     []domainmedia.JobType{domainmedia.JobReconcile},
		Queue:                 store.JobQueue(),
		Handler:               handler,
		Clock:                 func() time.Time { return time.Now().UTC() },
		Backoff:               appworker.Backoff{Base: time.Millisecond, Max: time.Millisecond, Float63: func() float64 { return 1 }},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()

	deadline := time.After(3 * time.Second)
	for {
		for _, j := range store.Jobs() {
			if j.Status == domainmedia.JobSucceeded {
				cancel()
				<-done
				return
			}
			if j.Status == domainmedia.JobDead || j.Status == domainmedia.JobFailed {
				cancel()
				<-done
				t.Fatalf("definition timeout must not clamp to global default; job=%+v", j)
			}
		}
		select {
		case <-deadline:
			cancel()
			<-done
			t.Fatalf("job did not succeed: %+v", store.Jobs())
		case <-time.After(20 * time.Millisecond):
		}
	}
}
