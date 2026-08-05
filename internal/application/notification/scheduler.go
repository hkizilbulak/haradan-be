package notification

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ExpirySchedulerConfig configures the daily expiry-scan scheduler.
type ExpirySchedulerConfig struct {
	Scanner  *ExpiryScanService
	Interval time.Duration
	Logger   *slog.Logger
}

// ExpiryScheduler ticks at Interval and ensures today's local-calendar-day
// PACKAGE_EXPIRY_SCAN job is enqueued (ExpiryScanService.EnqueueDailyExpiryScan
// dedups per day and sets available_at to the configured scan hour), so a
// worker claims and runs it once per day regardless of how many API/worker
// instances are running: the job table's dedup key, not scheduler count, is
// what prevents duplicate scans.
type ExpiryScheduler struct {
	scanner  *ExpiryScanService
	interval time.Duration
	logger   *slog.Logger

	wg sync.WaitGroup
}

// NewExpiryScheduler validates config and builds a scheduler. It performs no I/O.
func NewExpiryScheduler(cfg ExpirySchedulerConfig) (*ExpiryScheduler, error) {
	if cfg.Scanner == nil {
		return nil, fmt.Errorf("expiry scanner is required")
	}
	if cfg.Interval <= 0 {
		return nil, fmt.Errorf("interval must be greater than zero")
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &ExpiryScheduler{scanner: cfg.Scanner, interval: cfg.Interval, logger: logger}, nil
}

// Run ensures the daily scan job once immediately and again on every tick,
// blocking until ctx is cancelled. Callers should run it in its own
// goroutine (`go scheduler.Run(ctx)`) and cancel ctx for shutdown; the ticker
// is always stopped via defer before Run returns, so no ticker goroutine
// outlives the call.
func (s *ExpiryScheduler) Run(ctx context.Context) {
	s.wg.Add(1)
	defer s.wg.Done()

	s.ensureOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.ensureOnce(ctx)
		}
	}
}

// Wait blocks until a Run call started for this scheduler has returned, so a
// caller that cancelled Run's context can join it before the process exits.
func (s *ExpiryScheduler) Wait() {
	s.wg.Wait()
}

func (s *ExpiryScheduler) ensureOnce(ctx context.Context) {
	if err := s.scanner.EnqueueDailyExpiryScan(ctx); err != nil {
		s.logger.Error("ensure daily expiry scan job failed", "err", err.Error())
	}
}
