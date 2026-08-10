package tjk_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	apptjk "github.com/hkizilbulak/haradan-be/internal/application/tjk"
	domain "github.com/hkizilbulak/haradan-be/internal/domain/tjk"
)

type fakeRepo struct {
	run         domain.Run
	applied     string
	appliedIn   []domain.HorseInput
	finished    bool
	failed      bool
	retryable   *bool
	claimCount  int
	failMessage string
}

func (f *fakeRepo) ClaimTJKJob(context.Context, string, time.Time, time.Time) (uuid.UUID, domain.Run, bool, error) {
	f.claimCount++
	return uuid.New(), f.run, true, nil
}
func (f *fakeRepo) ApplyTJKPage(_ context.Context, _ uuid.UUID, _ domain.Run, horses []domain.HorseInput, next string, _ time.Time) error {
	f.applied = next
	f.appliedIn = append([]domain.HorseInput(nil), horses...)
	return nil
}
func (f *fakeRepo) FinishTJKRun(context.Context, uuid.UUID, domain.Run, time.Time) error {
	f.finished = true
	return nil
}
func (f *fakeRepo) FailTJKJob(_ context.Context, _ uuid.UUID, message string, _ time.Time, retryable bool) error {
	f.failed = true
	f.failMessage = message
	f.retryable = &retryable
	return nil
}

type fakeFetcher struct {
	cursor string
	horses []domain.HorseInput
	err    error
}

func (f *fakeFetcher) FetchPage(_ context.Context, cursor string) ([]domain.HorseInput, error) {
	f.cursor = cursor
	if f.err != nil {
		return nil, f.err
	}
	return f.horses, nil
}

type transientErr struct{}

func (transientErr) Error() string   { return "temporary upstream" }
func (transientErr) Retryable() bool { return true }

type permanentErr struct{}

func (permanentErr) Error() string   { return "permanent upstream" }
func (permanentErr) Retryable() bool { return false }

func TestWorkerStartsAtPageZero(t *testing.T) {
	repo := &fakeRepo{run: domain.Run{ID: uuid.New(), Checkpoint: json.RawMessage(`{"page":0}`)}}
	fetcher := &fakeFetcher{horses: []domain.HorseInput{{Number: "1", Name: "A", Detail: json.RawMessage(`{"pedigree":[{"father":"F","mother":"M"}]}`)}}}
	w, err := apptjk.NewWorker(repo, fetcher, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := w.ProcessOnce(context.Background(), time.Second)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if fetcher.cursor != "0" {
		t.Fatalf("cursor = %q", fetcher.cursor)
	}
	if repo.applied != "1" {
		t.Fatalf("next checkpoint = %q", repo.applied)
	}
	if len(repo.appliedIn) != 1 || len(repo.appliedIn[0].Detail) == 0 {
		t.Fatalf("expected detail on upsert payload: %#v", repo.appliedIn)
	}
}

func TestWorkerUsesCheckpointPage(t *testing.T) {
	repo := &fakeRepo{run: domain.Run{ID: uuid.New(), Checkpoint: json.RawMessage(`{"page":"3"}`)}}
	fetcher := &fakeFetcher{horses: []domain.HorseInput{{Number: "1", Name: "A"}}}
	w, err := apptjk.NewWorker(repo, fetcher, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.ProcessOnce(context.Background(), time.Second); err != nil {
		t.Fatal(err)
	}
	if fetcher.cursor != "3" || repo.applied != "4" {
		t.Fatalf("cursor=%q next=%q", fetcher.cursor, repo.applied)
	}
}

func TestWorkerFinishesOnEmptyPage(t *testing.T) {
	repo := &fakeRepo{run: domain.Run{ID: uuid.New(), Checkpoint: json.RawMessage(`{"page":0}`)}}
	fetcher := &fakeFetcher{}
	w, err := apptjk.NewWorker(repo, fetcher, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.ProcessOnce(context.Background(), time.Second); err != nil {
		t.Fatal(err)
	}
	if !repo.finished || repo.failed {
		t.Fatalf("finished=%v failed=%v", repo.finished, repo.failed)
	}
}

func TestWorkerRetriesTransientFetchErrors(t *testing.T) {
	repo := &fakeRepo{run: domain.Run{ID: uuid.New(), Checkpoint: json.RawMessage(`{"page":0}`)}}
	fetcher := &fakeFetcher{err: transientErr{}}
	w, err := apptjk.NewWorker(repo, fetcher, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.ProcessOnce(context.Background(), time.Second)
	if err == nil {
		t.Fatal("expected fetch error")
	}
	if !repo.failed || repo.retryable == nil || !*repo.retryable {
		t.Fatalf("failed=%v retryable=%v", repo.failed, repo.retryable)
	}
}

func TestWorkerFailsPermanentFetchErrorsWithoutRetry(t *testing.T) {
	repo := &fakeRepo{run: domain.Run{ID: uuid.New(), Checkpoint: json.RawMessage(`{"page":0}`)}}
	fetcher := &fakeFetcher{err: permanentErr{}}
	w, err := apptjk.NewWorker(repo, fetcher, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.ProcessOnce(context.Background(), time.Second)
	if err == nil {
		t.Fatal("expected fetch error")
	}
	if !repo.failed || repo.retryable == nil || *repo.retryable {
		t.Fatalf("failed=%v retryable=%v", repo.failed, repo.retryable)
	}
}

func TestWorkerTreatsDeadlineAsRetryable(t *testing.T) {
	repo := &fakeRepo{run: domain.Run{ID: uuid.New(), Checkpoint: json.RawMessage(`{}`)}}
	fetcher := &fakeFetcher{err: context.DeadlineExceeded}
	w, err := apptjk.NewWorker(repo, fetcher, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.ProcessOnce(context.Background(), time.Second)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v", err)
	}
	if repo.retryable == nil || !*repo.retryable {
		t.Fatalf("retryable=%v", repo.retryable)
	}
	// Empty checkpoint must still request page 0.
	if fetcher.cursor != "0" {
		t.Fatalf("cursor = %q", fetcher.cursor)
	}
}
