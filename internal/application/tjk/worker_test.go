package tjk_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func (f *fakeRepo) ClaimTJKJob(context.Context, string, time.Time, time.Time) (domain.PageJob, domain.Run, bool, error) {
	f.claimCount++
	page := 0
	var cp struct {
		Page json.RawMessage `json:"page"`
	}
	if json.Unmarshal(f.run.Checkpoint, &cp) == nil {
		if json.Unmarshal(cp.Page, &page) != nil {
			var text string
			if json.Unmarshal(cp.Page, &text) == nil {
				_, _ = fmt.Sscanf(text, "%d", &page)
			}
		}
	}
	return domain.PageJob{ID: uuid.New(), Page: page}, f.run, true, nil
}
func (f *fakeRepo) ApplyTJKPage(_ context.Context, job domain.PageJob, _ domain.Run, page domain.PageResult, _ time.Time) error {
	f.applied = fmt.Sprintf("%d", job.Page+1)
	f.appliedIn = append([]domain.HorseInput(nil), page.Horses...)
	return nil
}
func (f *fakeRepo) FinishTJKRun(context.Context, domain.PageJob, domain.Run, time.Time) error {
	f.finished = true
	return nil
}
func (f *fakeRepo) FailTJKJob(_ context.Context, _ domain.PageJob, _ uuid.UUID, message string, _ time.Time, retryable bool) error {
	f.failed = true
	f.failMessage = message
	f.retryable = &retryable
	return nil
}

type fakeFetcher struct {
	cursor string
	page   domain.PageResult
	err    error
}

func (f *fakeFetcher) FetchPage(_ context.Context, cursor string) (domain.PageResult, error) {
	f.cursor = cursor
	if f.err != nil {
		return domain.PageResult{}, f.err
	}
	return f.page, nil
}

type transientErr struct{}

func (transientErr) Error() string   { return "temporary upstream" }
func (transientErr) Retryable() bool { return true }

type permanentErr struct{}

func (permanentErr) Error() string   { return "permanent upstream" }
func (permanentErr) Retryable() bool { return false }

func TestWorkerStartsAtPageZero(t *testing.T) {
	repo := &fakeRepo{run: domain.Run{ID: uuid.New(), Checkpoint: json.RawMessage(`{"page":0}`)}}
	fetcher := &fakeFetcher{page: domain.PageResult{Fingerprint: "page-0", Horses: []domain.HorseInput{{Number: "1", Name: "A", Detail: json.RawMessage(`{"pedigree":[{"father":"F","mother":"M"}]}`)}}}}
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
	fetcher := &fakeFetcher{page: domain.PageResult{Fingerprint: "page-3", Horses: []domain.HorseInput{{Number: "1", Name: "A"}}}}
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
	fetcher := &fakeFetcher{page: domain.PageResult{EndOfSource: true}}
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

func TestWorkerDoesNotFinishOnUnverifiedEmptyResult(t *testing.T) {
	repo := &fakeRepo{run: domain.Run{ID: uuid.New(), Checkpoint: json.RawMessage(`{"page":0}`)}}
	w, err := apptjk.NewWorker(repo, &fakeFetcher{}, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.ProcessOnce(context.Background(), time.Second); err == nil {
		t.Fatal("expected unverified empty result to fail")
	}
	if repo.finished || !repo.failed {
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

func TestWorkerRejectsRepeatedPageFingerprint(t *testing.T) {
	repo := &fakeRepo{run: domain.Run{ID: uuid.New(), Checkpoint: json.RawMessage(`{"page":2,"lastFingerprint":"same"}`)}}
	fetcher := &fakeFetcher{page: domain.PageResult{
		Fingerprint: "same", Horses: []domain.HorseInput{{Number: "1", Name: "A"}},
	}}
	w, err := apptjk.NewWorker(repo, fetcher, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.ProcessOnce(context.Background(), time.Second); err == nil {
		t.Fatal("expected repeated page failure")
	}
	if !repo.failed || repo.retryable == nil || !*repo.retryable || repo.applied != "" {
		t.Fatalf("failed=%v retryable=%v applied=%q", repo.failed, repo.retryable, repo.applied)
	}
}
