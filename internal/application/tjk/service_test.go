package tjk

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domain "github.com/hkizilbulak/haradan-be/internal/domain/tjk"
)

type fakeRepo struct {
	created  domain.Run
	enqueued bool
}

func (f *fakeRepo) CreateRun(_ context.Context, r domain.Run) error { f.created = r; return nil }
func (f *fakeRepo) EnqueueRun(context.Context, uuid.UUID, time.Time) error {
	f.enqueued = true
	return nil
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
	svc, _ := NewService(repo)
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
func TestTriggerRejectsUnknownAdapter(t *testing.T) {
	svc, _ := NewService(&fakeRepo{})
	_, err := svc.Trigger(context.Background(), uuid.New(), "FULL", "OTHER")
	if err == nil {
		t.Fatal("expected error")
	}
	if e, ok := apperr.As(err); !ok || e.Kind != apperr.KindValidation {
		t.Fatalf("got %#v", err)
	}
}
