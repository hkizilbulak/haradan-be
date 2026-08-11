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
	FetchPage(context.Context, string) (domain.PageResult, error)
}

// WorkerRepository atomically claims, persists and finalizes durable TJK jobs.
type WorkerRepository interface {
	ClaimTJKJob(context.Context, string, time.Time, time.Time) (domain.PageJob, domain.Run, bool, error)
	ApplyTJKPage(context.Context, domain.PageJob, domain.Run, domain.PageResult, time.Time) error
	FinishTJKRun(context.Context, domain.PageJob, domain.Run, time.Time) error
	// FailTJKJob marks the job failed. When retryable is true and attempts remain
	// (attempt_count < max_attempts), the job is requeued with backoff.
	FailTJKJob(context.Context, domain.PageJob, uuid.UUID, string, time.Time, bool) error
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
	job, run, ok, err := w.repo.ClaimTJKJob(ctx, w.workerID, now, now.Add(lease))
	if err != nil || !ok {
		return ok, err
	}
	if run.CancelRequestedAt != nil {
		return true, w.repo.FinishTJKRun(ctx, job, run, now)
	}
	checkpoint := parseCheckpoint(run.Checkpoint)
	if job.Page != checkpoint.Page {
		err := fmt.Errorf("TJK job page does not match run checkpoint")
		w.fail(job, run.ID, "TJK sayfa kimliği checkpoint ile uyuşmuyor", now, false)
		return true, err
	}
	if job.Page >= maxTraversalPages {
		err := fmt.Errorf("TJK page safety ceiling reached")
		w.fail(job, run.ID, "TJK sayfa güvenlik sınırına ulaştı", now, false)
		return true, err
	}
	page, err := w.fetcher.FetchPage(ctx, strconv.Itoa(job.Page))
	if err != nil {
		w.fail(job, run.ID, "TJK sayfası alınamadı", now, isRetryable(err))
		return true, err
	}
	if page.EndOfSource {
		if len(page.Horses) != 0 {
			err := fmt.Errorf("TJK EOF page contains horses")
			w.fail(job, run.ID, "TJK sayfa sonu yanıtı tutarsız", now, true)
			return true, err
		}
		if err := w.repo.FinishTJKRun(ctx, job, run, now); err != nil {
			w.fail(job, run.ID, "TJK senkronizasyonu tamamlanamadı", now, true)
			return true, err
		}
		return true, nil
	}
	if page.Fingerprint == "" {
		err := fmt.Errorf("TJK page fingerprint is missing")
		w.fail(job, run.ID, "TJK sayfa kimliği üretilemedi", now, true)
		return true, err
	}
	if checkpoint.LastFingerprint != "" && checkpoint.LastFingerprint == page.Fingerprint {
		err := fmt.Errorf("TJK provider returned a repeated page")
		w.fail(job, run.ID, "TJK kaynağı ilerlemeyen aynı sayfayı döndürdü", now, true)
		return true, err
	}
	if err := w.repo.ApplyTJKPage(ctx, job, run, page, now); err != nil {
		w.fail(job, run.ID, "TJK sayfası kalıcılaştırılamadı", now, true)
		return true, err
	}
	return true, nil
}

const maxTraversalPages = 100000

type checkpoint struct {
	Page            int    `json:"page"`
	LastFingerprint string `json:"lastFingerprint,omitempty"`
	PagesProcessed  int    `json:"pagesProcessed,omitempty"`
	SourceProcessed int    `json:"sourceProcessed,omitempty"`
	SourceTotal     *int   `json:"sourceTotal,omitempty"`
}

func parseCheckpoint(raw json.RawMessage) checkpoint {
	var cp checkpoint
	if json.Unmarshal(raw, &cp) == nil && cp.Page >= 0 {
		return cp
	}
	var legacy struct {
		Page string `json:"page"`
	}
	if json.Unmarshal(raw, &legacy) == nil {
		if page, err := strconv.Atoi(legacy.Page); err == nil && page >= 0 {
			cp.Page = page
			return cp
		}
	}
	return checkpoint{}
}

func (w *Worker) fail(job domain.PageJob, runID uuid.UUID, message string, now time.Time, retryable bool) {
	failCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = w.repo.FailTJKJob(failCtx, job, runID, message, now, retryable)
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
	return parseCheckpoint(raw).Page
}
