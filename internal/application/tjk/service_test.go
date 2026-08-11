package tjk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domain "github.com/hkizilbulak/haradan-be/internal/domain/tjk"
)

type fakeRepo struct {
	created   domain.Run
	enqueued  bool
	createErr error
}

func (f *fakeRepo) CreateRun(_ context.Context, r domain.Run) error { f.created = r; return nil }
func (f *fakeRepo) EnqueueRun(context.Context, uuid.UUID, time.Time) error {
	f.enqueued = true
	return nil
}
func (f *fakeRepo) CreateRunAndEnqueue(ctx context.Context, run domain.Run, now time.Time) error {
	if f.createErr != nil {
		return f.createErr
	}
	if err := f.CreateRun(ctx, run); err != nil {
		return err
	}
	return f.EnqueueRun(ctx, run.ID, now)
}

func TestTriggerUsesAtomicRunAndJobPrimitive(t *testing.T) {
	repo := &fakeRepo{createErr: errors.New("enqueue failed")}
	svc, _ := NewService(Config{Repo: repo, Enabled: true})
	if _, err := svc.Trigger(context.Background(), uuid.New(), "FULL", "TJK_HTTP"); err == nil {
		t.Fatal("expected atomic trigger failure")
	}
	if repo.created.ID != uuid.Nil || repo.enqueued {
		t.Fatalf("failed atomic trigger leaked state: created=%s enqueued=%v", repo.created.ID, repo.enqueued)
	}
}
func (*fakeRepo) ListRuns(context.Context, *string, *string, int) ([]domain.Run, bool, error) {
	return nil, false, nil
}
func (*fakeRepo) GetRun(context.Context, uuid.UUID) (domain.Run, error) { return domain.Run{}, nil }
func (*fakeRepo) RequestCancel(context.Context, uuid.UUID, int, time.Time) (domain.Run, error) {
	return domain.Run{}, nil
}
func (*fakeRepo) ListItemErrors(context.Context, uuid.UUID, *string, *string, int) ([]domain.ItemError, bool, error) {
	return nil, false, nil
}
func (*fakeRepo) SetItemErrorStatus(context.Context, uuid.UUID, string, time.Time) (domain.ItemError, error) {
	return domain.ItemError{}, nil
}

func TestTriggerCreatesManualQueuedRun(t *testing.T) {
	repo := &fakeRepo{}
	svc, _ := NewService(Config{Repo: repo, Enabled: true})
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	svc.now = func() time.Time { return now }
	out, err := svc.Trigger(context.Background(), uuid.New(), "full", "TJK_HTTP")
	if err != nil {
		t.Fatal(err)
	}
	if !repo.enqueued || out.Status != domain.RunQueued || out.CreatedAt != now {
		t.Fatalf("unexpected run: %#v", out)
	}
	if string(out.Checkpoint) != `{"page":0}` {
		t.Fatalf("checkpoint = %s", out.Checkpoint)
	}
}

func TestTriggerDisabledDoesNotEnqueue(t *testing.T) {
	repo := &fakeRepo{}
	svc, err := NewService(Config{Repo: repo, Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Trigger(context.Background(), uuid.New(), "FULL", "TJK_HTTP")
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := apperr.As(err)
	if !ok || ae.Kind != apperr.KindDependencyUnavailable {
		t.Fatalf("got %#v", err)
	}
	if repo.enqueued || repo.created.ID != uuid.Nil {
		t.Fatalf("disabled Trigger must not CreateRun/Enqueue: created=%#v enqueued=%v", repo.created, repo.enqueued)
	}
}

func TestTriggerRejectsUnknownAdapter(t *testing.T) {
	svc, _ := NewService(Config{Repo: &fakeRepo{}, Enabled: true})
	_, err := svc.Trigger(context.Background(), uuid.New(), "FULL", "OTHER")
	if err == nil {
		t.Fatal("expected error")
	}
	if e, ok := apperr.As(err); !ok || e.Kind != apperr.KindValidation {
		t.Fatalf("got %#v", err)
	}
}
