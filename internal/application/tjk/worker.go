package tjk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"

	domain "github.com/hkizilbulak/haradan-be/internal/domain/tjk"
)

// PageFetcher is deliberately small so the application layer remains independent
// of the HTTP adapter and can be fixture-tested.
type PageFetcher interface {
	FetchPage(context.Context, string) ([]domain.HorseInput, error)
}

// WorkerRepository atomically claims, persists and finalizes durable TJK jobs.
type WorkerRepository interface {
	ClaimTJKJob(context.Context, string, time.Time, time.Time) (uuid.UUID, domain.Run, bool, error)
	ApplyTJKPage(context.Context, uuid.UUID, domain.Run, []domain.HorseInput, string, time.Time) error
	FinishTJKRun(context.Context, uuid.UUID, domain.Run, time.Time) error
	// FailTJKJob marks the job failed. When retryable is true and attempts remain
	// (attempt_count < max_attempts), the job is requeued with backoff.
	FailTJKJob(context.Context, uuid.UUID, string, time.Time, bool) error
}

type Worker struct {
	repo     WorkerRepository
	fetcher  PageFetcher
	workerID string
	now      func() time.Time
}

func NewWorker(repo WorkerRepository, fetcher PageFetcher, workerID string) (*Worker, error) {
	if repo == nil || fetcher == nil || workerID == "" {
		return nil, fmt.Errorf("TJK worker dependencies are required")
	}
	return &Worker{repo: repo, fetcher: fetcher, workerID: workerID, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (w *Worker) ProcessOnce(ctx context.Context, lease time.Duration) (bool, error) {
	now := w.now()
	jobID, run, ok, err := w.repo.ClaimTJKJob(ctx, w.workerID, now, now.Add(lease))
	if err != nil || !ok {
		return ok, err
	}
	if run.CancelRequestedAt != nil {
		return true, w.repo.FinishTJKRun(ctx, jobID, run, now)
	}
	page := checkpointPage(run.Checkpoint)
	horses, err := w.fetcher.FetchPage(ctx, strconv.Itoa(page))
	if err != nil {
		retryable := isRetryable(err)
		failCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = w.repo.FailTJKJob(failCtx, jobID, "TJK sayfası alınamadı", now, retryable)
		return true, err
	}
	if len(horses) == 0 {
		return true, w.repo.FinishTJKRun(ctx, jobID, run, now)
	}
	return true, w.repo.ApplyTJKPage(ctx, jobID, run, horses, strconv.Itoa(page+1), now)
}

type retryableError interface {
	Retryable() bool
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var r retryableError
	return errors.As(err, &r) && r.Retryable()
}

// checkpointPage returns the legacy 0-based TJK PageNumber stored in the run
// checkpoint. Missing/invalid checkpoints start at page 0.
func checkpointPage(raw json.RawMessage) int {
	var v struct {
		Page json.RawMessage `json:"page"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return 0
	}
	var page int
	if json.Unmarshal(v.Page, &page) == nil && page >= 0 {
		return page
	}
	var text string
	if json.Unmarshal(v.Page, &text) == nil {
		if n, err := strconv.Atoi(text); err == nil && n >= 0 {
			return n
		}
	}
	return 0
}
