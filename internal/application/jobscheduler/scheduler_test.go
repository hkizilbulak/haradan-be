package jobscheduler_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	appjobadmin "github.com/hkizilbulak/haradan-be/internal/application/jobadmin"
	"github.com/hkizilbulak/haradan-be/internal/application/jobscheduler"
	domainjobdef "github.com/hkizilbulak/haradan-be/internal/domain/jobdef"
)

type fixedClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fixedClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fixedClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = t
}

type fakeDefs struct {
	mu   sync.Mutex
	defs []domainjobdef.JobDefinition
}

func (f *fakeDefs) ListDefinitions(context.Context) ([]domainjobdef.JobDefinition, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := append([]domainjobdef.JobDefinition(nil), f.defs...)
	return out, nil
}

func (f *fakeDefs) Set(defs []domainjobdef.JobDefinition) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.defs = append([]domainjobdef.JobDefinition(nil), defs...)
}

type fakeEnqueuer struct {
	mu    sync.Mutex
	calls []appjobadmin.EnqueueRequest
	dedup map[string]struct{}
}

func newFakeEnqueuer() *fakeEnqueuer {
	return &fakeEnqueuer{dedup: map[string]struct{}{}}
}

func (f *fakeEnqueuer) Enqueue(_ context.Context, req appjobadmin.EnqueueRequest) (appjobadmin.EnqueueResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.dedup[req.DeduplicationKey]; ok {
		return appjobadmin.EnqueueResult{AlreadyExists: true}, nil
	}
	f.dedup[req.DeduplicationKey] = struct{}{}
	f.calls = append(f.calls, req)
	return appjobadmin.EnqueueResult{BackgroundJobID: uuid.New()}, nil
}

func (f *fakeEnqueuer) Calls() []appjobadmin.EnqueueRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]appjobadmin.EnqueueRequest(nil), f.calls...)
}

func sampleDef(jobType domainjobdef.JobType, active bool) domainjobdef.JobDefinition {
	key := string(jobType)
	return domainjobdef.JobDefinition{
		ID:             uuid.New(),
		JobKey:         key,
		Name:           key,
		JobType:        jobType,
		CronExpression: "0 0 9 * * *",
		IsActive:       active,
		TimeoutSeconds: 1800,
		DefaultPayload: []byte(`{}`),
		Version:        1,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
}

func TestSchedulerOccurrenceDedup(t *testing.T) {
	t.Parallel()
	defs := &fakeDefs{}
	enq := newFakeEnqueuer()
	clock := &fixedClock{t: time.Date(2026, 8, 5, 9, 0, 0, 0, domainjobdef.Istanbul())}
	def := sampleDef(domainjobdef.JobTypePackageExpiryScan, true)
	defs.Set([]domainjobdef.JobDefinition{def})

	sched, err := jobscheduler.New(jobscheduler.Config{
		Definitions: defs, Enqueuer: enq,
		Capabilities:    appjobadmin.ProviderCapabilities{B2Enabled: true, TinifyEnabled: true, TJKEnabled: true},
		RefreshInterval: time.Minute,
		Clock:           clock,
	})
	if err != nil {
		t.Fatal(err)
	}
	occ := clock.Now()
	if err := sched.FireForTest(context.Background(), def, occ); err != nil {
		t.Fatal(err)
	}
	if err := sched.FireForTest(context.Background(), def, occ); err != nil {
		t.Fatal(err)
	}
	calls := enq.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 enqueue after dedup, got %d", len(calls))
	}
	want := domainjobdef.ScheduledOccurrenceDedupKey(def.JobKey, occ)
	if calls[0].DeduplicationKey != want {
		t.Fatalf("dedup=%q want %q", calls[0].DeduplicationKey, want)
	}
	if calls[0].ExecutionType != domainjobdef.ExecutionTypeScheduled {
		t.Fatalf("execution type=%s", calls[0].ExecutionType)
	}
}

func TestSchedulerSkipsDisabledProvider(t *testing.T) {
	t.Parallel()
	defs := &fakeDefs{}
	enq := newFakeEnqueuer()
	tjk := sampleDef(domainjobdef.JobTypeTJKSync, true)
	media := sampleDef(domainjobdef.JobTypeMediaReconcile, true)
	expiry := sampleDef(domainjobdef.JobTypePackageExpiryScan, true)
	defs.Set([]domainjobdef.JobDefinition{tjk, media, expiry})

	sched, err := jobscheduler.New(jobscheduler.Config{
		Definitions: defs, Enqueuer: enq,
		Capabilities:    appjobadmin.ProviderCapabilities{TJKEnabled: false, B2Enabled: false},
		RefreshInterval: time.Minute,
		Clock:           &fixedClock{t: time.Now().UTC()},
	})
	if err != nil {
		t.Fatal(err)
	}
	sched.RefreshForTest(context.Background())
	if sched.ScheduledCountForTest() != 1 {
		t.Fatalf("expected only expiry scheduled, got %d", sched.ScheduledCountForTest())
	}

	if err := sched.FireForTest(context.Background(), tjk, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := sched.FireForTest(context.Background(), media, time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(enq.Calls()) != 0 {
		t.Fatalf("provider-disabled jobs must not enqueue: %d", len(enq.Calls()))
	}
}

func TestSchedulerRefreshRemovesInactive(t *testing.T) {
	t.Parallel()
	defs := &fakeDefs{}
	enq := newFakeEnqueuer()
	def := sampleDef(domainjobdef.JobTypePackageExpiryScan, true)
	defs.Set([]domainjobdef.JobDefinition{def})

	sched, err := jobscheduler.New(jobscheduler.Config{
		Definitions: defs, Enqueuer: enq,
		Capabilities:    appjobadmin.ProviderCapabilities{B2Enabled: true},
		RefreshInterval: time.Minute,
		Clock:           &fixedClock{t: time.Now().UTC()},
	})
	if err != nil {
		t.Fatal(err)
	}
	sched.RefreshForTest(context.Background())
	if sched.ScheduledCountForTest() != 1 {
		t.Fatal("expected one schedule")
	}
	def.IsActive = false
	def.Version = 2
	defs.Set([]domainjobdef.JobDefinition{def})
	sched.RefreshForTest(context.Background())
	if sched.ScheduledCountForTest() != 0 {
		t.Fatalf("inactive must be removed, got %d", sched.ScheduledCountForTest())
	}
}

func TestSchedulerGracefulShutdownNoTickerLeak(t *testing.T) {
	t.Parallel()
	defs := &fakeDefs{}
	enq := newFakeEnqueuer()
	sched, err := jobscheduler.New(jobscheduler.Config{
		Definitions: defs, Enqueuer: enq,
		Capabilities:    appjobadmin.ProviderCapabilities{},
		RefreshInterval: 50 * time.Millisecond,
		Clock:           &fixedClock{t: time.Now().UTC()},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		sched.Run(ctx)
		close(done)
	}()
	time.Sleep(120 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop")
	}
	sched.Wait()
}
