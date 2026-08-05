package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

// MediaJobHandler is the media processing surface the runner invokes.
type MediaJobHandler interface {
	ProcessValidateAndNormalize(ctx context.Context, assetID uuid.UUID) error
	ProcessGenerateVariant(ctx context.Context, assetID uuid.UUID, profile string) error
	ProcessDeleteObjects(ctx context.Context, payload []byte) error
	ProcessReconcile(ctx context.Context, payload []byte) error
}

// NotificationJobHandler owns notification runtime job dispatch. It remains
// separate from MediaJobHandler so workers can claim in-app fan-out jobs when
// media/email providers are unavailable.
type NotificationJobHandler interface {
	ProcessAdvertFanout(ctx context.Context, jobType domainmedia.JobType, payload []byte) error
	ProcessAdvertEmailChunk(ctx context.Context, payload []byte) error
	ProcessExpiryScan(ctx context.Context, payload []byte) error
	ProcessPackageExpiryEmail(ctx context.Context, payload []byte) error
}

// Config configures the media background job runner.
type Config struct {
	WorkerID              string
	Concurrency           int
	PollInterval          time.Duration
	LeaseDuration         time.Duration
	JobTimeout            time.Duration
	ShutdownTimeout       time.Duration
	RetryBaseDelay        time.Duration
	RetryMaxDelay         time.Duration
	LeaseRecoveryInterval time.Duration
	SupportedJobTypes     []domainmedia.JobType
	RecoveryBatchSize     int

	Queue               appmedia.JobQueue
	Handler             MediaJobHandler
	NotificationHandler NotificationJobHandler
	Logger              *slog.Logger
	Clock               func() time.Time
	Backoff             Backoff
}

// Runner polls, claims, and processes media background jobs.
type Runner struct {
	cfg Config

	loopsWG sync.WaitGroup
}

// NewRunner validates config and builds a runner. It performs no I/O.
func NewRunner(cfg Config) (*Runner, error) {
	if cfg.Queue == nil {
		return nil, fmt.Errorf("job queue is required")
	}
	if cfg.Handler == nil {
		return nil, fmt.Errorf("media job handler is required")
	}
	if cfg.Concurrency <= 0 {
		return nil, fmt.Errorf("concurrency must be greater than zero")
	}
	if cfg.PollInterval <= 0 || cfg.LeaseDuration <= 0 || cfg.JobTimeout <= 0 {
		return nil, fmt.Errorf("poll, lease and job timeout durations must be greater than zero")
	}
	if cfg.JobTimeout >= cfg.LeaseDuration {
		return nil, fmt.Errorf("job timeout must be less than lease duration")
	}
	if cfg.ShutdownTimeout <= 0 || cfg.LeaseRecoveryInterval <= 0 {
		return nil, fmt.Errorf("shutdown and lease recovery intervals must be greater than zero")
	}
	if cfg.RetryBaseDelay <= 0 || cfg.RetryMaxDelay < cfg.RetryBaseDelay {
		return nil, fmt.Errorf("retry delays are invalid")
	}
	if cfg.WorkerID == "" {
		return nil, fmt.Errorf("worker id is required")
	}
	if len(cfg.SupportedJobTypes) == 0 {
		cfg.SupportedJobTypes = []domainmedia.JobType{
			domainmedia.JobValidateAndNormalize,
			domainmedia.JobGenerateVariant,
		}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Clock == nil {
		cfg.Clock = func() time.Time { return time.Now().UTC() }
	}
	if cfg.RecoveryBatchSize <= 0 {
		cfg.RecoveryBatchSize = 100
	}
	if cfg.Backoff.Base == 0 {
		cfg.Backoff.Base = cfg.RetryBaseDelay
	}
	if cfg.Backoff.Max == 0 {
		cfg.Backoff.Max = cfg.RetryMaxDelay
	}
	return &Runner{cfg: cfg}, nil
}

// Run blocks until ctx is cancelled, then drains in-flight work up to ShutdownTimeout.
//
// Shutdown semantics:
//  1. stopClaim cancels new claims (and recovery loop).
//  2. In-flight jobs keep running under jobRoot until they finish or ShutdownTimeout.
//  3. On timeout, hardCancel cancels job contexts; canceled work is requeued
//     (attempt_count already incremented at claim time stays as-is).
func (r *Runner) Run(ctx context.Context) error {
	claimCtx, stopClaim := context.WithCancel(context.Background())
	jobRoot, hardCancel := context.WithCancel(context.Background())
	defer hardCancel()

	r.recoverOnce(claimCtx)

	r.loopsWG.Add(1)
	go func() {
		defer r.loopsWG.Done()
		r.recoveryLoop(claimCtx)
	}()

	for i := 0; i < r.cfg.Concurrency; i++ {
		r.loopsWG.Add(1)
		go func() {
			defer r.loopsWG.Done()
			r.workerLoop(claimCtx, jobRoot)
		}()
	}

	<-ctx.Done()
	r.cfg.Logger.Info("worker shutdown requested", "workerId", r.cfg.WorkerID)
	stopClaim()

	done := make(chan struct{})
	go func() {
		r.loopsWG.Wait()
		close(done)
	}()

	timer := time.NewTimer(r.cfg.ShutdownTimeout)
	defer timer.Stop()

	select {
	case <-done:
		r.cfg.Logger.Info("in-flight jobs drained", "workerId", r.cfg.WorkerID)
	case <-timer.C:
		r.cfg.Logger.Warn("shutdown timeout; cancelling in-flight jobs", "workerId", r.cfg.WorkerID)
		hardCancel()
		<-done
	}

	return nil
}

func (r *Runner) recoverOnce(ctx context.Context) {
	n, err := r.cfg.Queue.RecoverExpiredJobLeases(ctx, r.cfg.Clock(), r.cfg.RecoveryBatchSize)
	if err != nil {
		r.cfg.Logger.Error("lease recovery failed", "workerId", r.cfg.WorkerID, "err", "dependency unavailable")
		return
	}
	if n > 0 {
		r.cfg.Logger.Info("recovered expired leases", "workerId", r.cfg.WorkerID, "count", n)
	}
}

func (r *Runner) recoveryLoop(ctx context.Context) {
	ticker := time.NewTicker(r.cfg.LeaseRecoveryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.recoverOnce(ctx)
		}
	}
}

func (r *Runner) workerLoop(claimCtx, jobRoot context.Context) {
	for {
		if claimCtx.Err() != nil {
			return
		}

		now := r.cfg.Clock()
		job, ok, err := r.cfg.Queue.ClaimNextJob(claimCtx, appmedia.ClaimJobParams{
			LeaseOwner:     r.cfg.WorkerID,
			Now:            now,
			LeaseUntil:     now.Add(r.cfg.LeaseDuration),
			SupportedTypes: r.cfg.SupportedJobTypes,
		})
		if err != nil {
			if claimCtx.Err() != nil {
				return
			}
			r.cfg.Logger.Error("claim failed", "workerId", r.cfg.WorkerID, "err", "dependency unavailable")
			r.sleep(claimCtx, r.cfg.PollInterval)
			continue
		}
		if !ok {
			r.sleep(claimCtx, r.cfg.PollInterval)
			continue
		}

		// Finish the claimed job even after stopClaim; shutdown waits on loopsWG.
		r.processClaimed(jobRoot, claimCtx, job)
	}
}

func (r *Runner) processClaimed(jobRoot, claimCtx context.Context, job domainmedia.BackgroundJob) {
	start := r.cfg.Clock()
	guard := appmedia.JobLeaseGuard{
		JobID:      job.ID,
		LeaseOwner: r.cfg.WorkerID,
		Version:    job.Version,
	}

	finalized := false
	defer func() {
		if rec := recover(); rec != nil {
			r.cfg.Logger.Error("job panic recovered",
				"workerId", r.cfg.WorkerID,
				"jobId", job.ID.String(),
				"jobType", string(job.JobType),
				"attempt", job.AttemptCount,
			)
			if finalized {
				return
			}
			if err := r.retryOrDead(context.Background(), guard, job, safeInternalErrorMessage); err != nil {
				r.cfg.Logger.Error("panic retry failed",
					"workerId", r.cfg.WorkerID, "jobId", job.ID.String(), "err", "dependency unavailable")
			}
		}
	}()

	jobCtx, cancel := context.WithTimeout(jobRoot, r.cfg.JobTimeout)
	defer cancel()

	err := r.dispatch(jobCtx, job)
	shuttingDown := claimCtx.Err() != nil || jobRoot.Err() != nil
	outcome := classifyProcessError(err, shuttingDown)
	duration := r.cfg.Clock().Sub(start)

	switch outcome.Kind {
	case outcomeSuccess:
		if markErr := r.cfg.Queue.MarkJobSucceeded(context.Background(), guard, r.cfg.Clock()); markErr != nil {
			r.cfg.Logger.Error("mark succeeded failed",
				"workerId", r.cfg.WorkerID, "jobId", job.ID.String(), "err", "dependency unavailable")
			return
		}
		finalized = true
		r.cfg.Logger.Info("job succeeded",
			"workerId", r.cfg.WorkerID,
			"jobId", job.ID.String(),
			"jobType", string(job.JobType),
			"attempt", job.AttemptCount,
			"outcome", "succeeded",
			"duration", duration.String(),
		)
	case outcomePermanentFail:
		if markErr := r.cfg.Queue.MarkJobFailed(context.Background(), guard, r.cfg.Clock(), outcome.LastError); markErr != nil {
			r.cfg.Logger.Error("mark failed failed",
				"workerId", r.cfg.WorkerID, "jobId", job.ID.String(), "err", "dependency unavailable")
			return
		}
		finalized = true
		r.cfg.Logger.Info("job failed permanently",
			"workerId", r.cfg.WorkerID,
			"jobId", job.ID.String(),
			"jobType", string(job.JobType),
			"attempt", job.AttemptCount,
			"outcome", "failed",
			"duration", duration.String(),
		)
	case outcomeShutdownRequeue:
		if markErr := r.retryOrDead(context.Background(), guard, job, outcome.LastError); markErr != nil {
			r.cfg.Logger.Error("shutdown requeue failed",
				"workerId", r.cfg.WorkerID, "jobId", job.ID.String(), "err", "dependency unavailable")
			return
		}
		finalized = true
		r.cfg.Logger.Info("job requeued on shutdown",
			"workerId", r.cfg.WorkerID,
			"jobId", job.ID.String(),
			"jobType", string(job.JobType),
			"attempt", job.AttemptCount,
			"outcome", "requeue",
			"duration", duration.String(),
		)
	case outcomeTransientRetry:
		dead := job.AttemptCount >= job.MaxAttempts
		if markErr := r.retryOrDead(context.Background(), guard, job, outcome.LastError); markErr != nil {
			r.cfg.Logger.Error("retry failed",
				"workerId", r.cfg.WorkerID, "jobId", job.ID.String(), "err", "dependency unavailable")
			return
		}
		finalized = true
		outcomeLabel := "retry"
		if dead {
			outcomeLabel = "dead"
		}
		r.cfg.Logger.Info("job transient outcome",
			"workerId", r.cfg.WorkerID,
			"jobId", job.ID.String(),
			"jobType", string(job.JobType),
			"attempt", job.AttemptCount,
			"outcome", outcomeLabel,
			"duration", duration.String(),
		)
	}
}

func (r *Runner) retryOrDead(
	ctx context.Context,
	guard appmedia.JobLeaseGuard,
	job domainmedia.BackgroundJob,
	lastError string,
) error {
	now := r.cfg.Clock()
	delay := r.cfg.Backoff.Delay(job.AttemptCount)
	if lastError == safeShutdownErrorMessage {
		delay = 0
	}
	return r.cfg.Queue.RetryOrDeadLetterJob(ctx, appmedia.RetryJobParams{
		JobLeaseGuard:   guard,
		Now:             now,
		NextAvailableAt: now.Add(delay),
		LastError:       lastError,
		AttemptCount:    job.AttemptCount,
		MaxAttempts:     job.MaxAttempts,
	})
}

func (r *Runner) dispatch(ctx context.Context, job domainmedia.BackgroundJob) error {
	switch job.JobType {
	case domainmedia.JobNotificationFanoutAdvancedAdvert, domainmedia.JobNotificationFanoutUrgentAdvert:
		if r.cfg.NotificationHandler == nil {
			return apperr.Validation(safeUnsupportedJobMessage)
		}
		return r.cfg.NotificationHandler.ProcessAdvertFanout(ctx, job.JobType, []byte(job.Payload))
	case domainmedia.JobEmailSendAdvertNotificationChunk:
		if r.cfg.NotificationHandler == nil {
			return apperr.Validation(safeUnsupportedJobMessage)
		}
		return r.cfg.NotificationHandler.ProcessAdvertEmailChunk(ctx, []byte(job.Payload))
	case domainmedia.JobPackageExpiryReminderScan:
		if r.cfg.NotificationHandler == nil {
			return apperr.Validation(safeUnsupportedJobMessage)
		}
		return r.cfg.NotificationHandler.ProcessExpiryScan(ctx, []byte(job.Payload))
	case domainmedia.JobEmailSendPackageExpiryReminder:
		if r.cfg.NotificationHandler == nil {
			return apperr.Validation(safeUnsupportedJobMessage)
		}
		return r.cfg.NotificationHandler.ProcessPackageExpiryEmail(ctx, []byte(job.Payload))
	case domainmedia.JobDeleteObjects:
		return r.cfg.Handler.ProcessDeleteObjects(ctx, []byte(job.Payload))
	case domainmedia.JobReconcile:
		return r.cfg.Handler.ProcessReconcile(ctx, []byte(job.Payload))
	}
	parsed, err := parseMediaJob(job.JobType, job.Payload)
	if err != nil {
		return err
	}
	switch parsed.JobType {
	case domainmedia.JobValidateAndNormalize:
		return r.cfg.Handler.ProcessValidateAndNormalize(ctx, parsed.AssetID)
	case domainmedia.JobGenerateVariant:
		return r.cfg.Handler.ProcessGenerateVariant(ctx, parsed.AssetID, parsed.Profile)
	default:
		return apperr.Validation(safeUnsupportedJobMessage)
	}
}

func (r *Runner) sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
