package tjk

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domain "github.com/hkizilbulak/haradan-be/internal/domain/tjk"
)

type cancelFakeRepo struct {
	runs     map[uuid.UUID]domain.Run
	active   map[string]uuid.UUID // source|scope -> active run id
	jobs     map[uuid.UUID]string // runID -> job status
	triggers int
}

func newCancelFake() *cancelFakeRepo {
	return &cancelFakeRepo{
		runs:   map[uuid.UUID]domain.Run{},
		active: map[string]uuid.UUID{},
		jobs:   map[uuid.UUID]string{},
	}
}

func (f *cancelFakeRepo) key(source, scope string) string { return source + "|" + scope }

func (f *cancelFakeRepo) CreateRun(_ context.Context, r domain.Run) error {
	k := f.key(r.SourceAdapter, r.Scope)
	if id, ok := f.active[k]; ok {
		if run, exists := f.runs[id]; exists && (run.Status == domain.RunQueued || run.Status == domain.RunRunning) {
			return apperr.Conflict("Bu kaynak ve kapsam için zaten aktif bir TJK senkronizasyonu var.")
		}
	}
	f.runs[r.ID] = r
	f.active[k] = r.ID
	return nil
}
func (f *cancelFakeRepo) EnqueueRun(_ context.Context, id uuid.UUID, _ time.Time) error {
	f.jobs[id] = "QUEUED"
	f.triggers++
	return nil
}
func (f *cancelFakeRepo) CreateRunAndEnqueue(ctx context.Context, run domain.Run, now time.Time) error {
	if err := f.CreateRun(ctx, run); err != nil {
		return err
	}
	return f.EnqueueRun(ctx, run.ID, now)
}
func (*cancelFakeRepo) ListRuns(context.Context, *string, *string, int) ([]domain.Run, bool, error) {
	return nil, false, nil
}
func (f *cancelFakeRepo) GetRun(_ context.Context, id uuid.UUID) (domain.Run, error) {
	r, ok := f.runs[id]
	if !ok {
		return domain.Run{}, apperr.NotFound("missing")
	}
	return r, nil
}
func (f *cancelFakeRepo) RequestCancel(_ context.Context, id uuid.UUID, version int, now time.Time) (domain.Run, error) {
	r, ok := f.runs[id]
	if !ok || r.Version != version {
		return domain.Run{}, apperr.Conflict("TJK senkronizasyonu güncellenemedi.")
	}
	switch r.Status {
	case domain.RunQueued:
		r.Status = domain.RunCancelled
		r.CancelRequestedAt = &now
		r.CancelledAt = &now
		r.CompletedAt = &now
		r.Version++
		r.UpdatedAt = now
		f.runs[id] = r
		f.jobs[id] = "CANCELLED"
		delete(f.active, f.key(r.SourceAdapter, r.Scope))
		return r, nil
	case domain.RunRunning:
		r.CancelRequestedAt = &now
		r.Version++
		r.UpdatedAt = now
		f.runs[id] = r
		return r, nil
	default:
		return domain.Run{}, apperr.Conflict("TJK senkronizasyonu güncellenemedi.")
	}
}
func (*cancelFakeRepo) ListItemErrors(context.Context, uuid.UUID, *string, *string, int) ([]domain.ItemError, bool, error) {
	return nil, false, nil
}
func (*cancelFakeRepo) SetItemErrorStatus(context.Context, uuid.UUID, string, time.Time) (domain.ItemError, error) {
	return domain.ItemError{}, nil
}

func TestQueuedCancelAllowsImmediateRetrigger(t *testing.T) {
	repo := newCancelFake()
	svc, err := NewService(Config{Repo: repo, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	actor := uuid.New()
	first, err := svc.Trigger(context.Background(), actor, "FULL", "TJK_HTTP")
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := svc.Cancel(context.Background(), first.ID, first.Version)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != domain.RunCancelled {
		t.Fatalf("queued cancel must terminalize, got %s", cancelled.Status)
	}
	if repo.jobs[first.ID] != "CANCELLED" {
		t.Fatalf("job status=%s", repo.jobs[first.ID])
	}
	second, err := svc.Trigger(context.Background(), actor, "FULL", "TJK_HTTP")
	if err != nil {
		t.Fatalf("expected immediate re-trigger after queued cancel, got %v", err)
	}
	if second.Status != domain.RunQueued || second.ID == first.ID {
		t.Fatalf("unexpected second run %#v", second)
	}
}

func TestTerminalCancelConflicts(t *testing.T) {
	repo := newCancelFake()
	svc, _ := NewService(Config{Repo: repo, Enabled: true})
	run, err := svc.Trigger(context.Background(), uuid.New(), "FULL", "TJK_HTTP")
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := svc.Cancel(context.Background(), run.ID, run.Version)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Cancel(context.Background(), cancelled.ID, cancelled.Version)
	if err == nil {
		t.Fatal("expected conflict on terminal cancel")
	}
	if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindConflict {
		t.Fatalf("got %#v", err)
	}
}

func TestRunningCancelStaysSoft(t *testing.T) {
	repo := newCancelFake()
	id := uuid.New()
	now := time.Now().UTC()
	repo.runs[id] = domain.Run{
		ID: id, Status: domain.RunRunning, SourceAdapter: "TJK_HTTP", Scope: "HORSES",
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	repo.active[repo.key("TJK_HTTP", "HORSES")] = id
	svc, _ := NewService(Config{Repo: repo, Enabled: true})
	out, err := svc.Cancel(context.Background(), id, 1)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != domain.RunRunning || out.CancelRequestedAt == nil {
		t.Fatalf("running cancel must stay soft: %#v", out)
	}
	// Still active — new trigger conflicts.
	_, err = svc.Trigger(context.Background(), uuid.New(), "FULL", "TJK_HTTP")
	if err == nil {
		t.Fatal("expected active conflict while soft-cancelled RUNNING remains")
	}
}
