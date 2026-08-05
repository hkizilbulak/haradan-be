package jobscheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	appjobadmin "github.com/hkizilbulak/haradan-be/internal/application/jobadmin"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainjobdef "github.com/hkizilbulak/haradan-be/internal/domain/jobdef"
)

// DefinitionSource loads job definitions for scheduling.
type DefinitionSource interface {
	ListDefinitions(ctx context.Context) ([]domainjobdef.JobDefinition, error)
}

// Enqueuer enqueues scheduled occurrences.
type Enqueuer interface {
	Enqueue(ctx context.Context, req appjobadmin.EnqueueRequest) (appjobadmin.EnqueueResult, error)
}

// Clock supplies instants for tests.
type Clock interface {
	Now() time.Time
}

// Config configures the DB-driven job definition scheduler.
type Config struct {
	Definitions     DefinitionSource
	Enqueuer        Enqueuer
	Capabilities    appjobadmin.ProviderCapabilities
	RefreshInterval time.Duration
	Location        *time.Location
	Logger          *slog.Logger
	Clock           Clock
}

// Scheduler reads active job definitions and enqueues durable occurrence jobs
// with multi-replica dedup via (job_definition_id, deduplication_key).
type Scheduler struct {
	defs     DefinitionSource
	enqueuer Enqueuer
	caps     appjobadmin.ProviderCapabilities
	refresh  time.Duration
	loc      *time.Location
	logger   *slog.Logger
	clock    Clock

	mu       sync.Mutex
	cron     *cron.Cron
	entries  map[uuid.UUID]cron.EntryID
	versions map[uuid.UUID]int
	crons    map[uuid.UUID]string
	active   map[uuid.UUID]bool

	wg sync.WaitGroup
}

// New validates config and builds a scheduler. No I/O.
func New(cfg Config) (*Scheduler, error) {
	if cfg.Definitions == nil || cfg.Enqueuer == nil {
		return nil, fmt.Errorf("jobscheduler dependencies are required")
	}
	if cfg.RefreshInterval <= 0 {
		return nil, fmt.Errorf("refresh interval must be greater than zero")
	}
	loc := cfg.Location
	if loc == nil {
		loc = domainjobdef.Istanbul()
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	clock := cfg.Clock
	if clock == nil {
		clock = systemClock{}
	}
	return &Scheduler{
		defs:     cfg.Definitions,
		enqueuer: cfg.Enqueuer,
		caps:     cfg.Capabilities,
		refresh:  cfg.RefreshInterval,
		loc:      loc,
		logger:   logger,
		clock:    clock,
		entries:  map[uuid.UUID]cron.EntryID{},
		versions: map[uuid.UUID]int{},
		crons:    map[uuid.UUID]string{},
		active:   map[uuid.UUID]bool{},
	}, nil
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// Run loads definitions, starts cron, and refreshes on an interval until ctx
// is cancelled. The refresh ticker and cron are always stopped before return.
func (s *Scheduler) Run(ctx context.Context) {
	s.wg.Add(1)
	defer s.wg.Done()

	s.cron = cron.New(cron.WithSeconds(), cron.WithLocation(s.loc))
	s.refreshOnce(ctx)
	s.cron.Start()
	defer func() {
		stopCtx := s.cron.Stop()
		<-stopCtx.Done()
	}()

	ticker := time.NewTicker(s.refresh)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshOnce(ctx)
		}
	}
}

// Wait joins a Run goroutine.
func (s *Scheduler) Wait() {
	s.wg.Wait()
}

// RefreshForTest reloads schedules once (unit tests).
func (s *Scheduler) RefreshForTest(ctx context.Context) {
	s.mu.Lock()
	if s.cron == nil {
		s.cron = cron.New(cron.WithSeconds(), cron.WithLocation(s.loc))
	}
	s.mu.Unlock()
	s.refreshOnce(ctx)
}

// FireForTest enqueues a scheduled occurrence as if cron fired at occurrence.
func (s *Scheduler) FireForTest(ctx context.Context, def domainjobdef.JobDefinition, occurrence time.Time) error {
	return s.enqueueOccurrence(ctx, def, occurrence)
}

func (s *Scheduler) refreshOnce(ctx context.Context) {
	defs, err := s.defs.ListDefinitions(ctx)
	if err != nil {
		s.logger.Error("job definition refresh failed", "err", err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cron == nil {
		return
	}

	seen := map[uuid.UUID]struct{}{}
	for _, def := range defs {
		seen[def.ID] = struct{}{}
		shouldSchedule := def.IsActive && s.caps.Allows(def.JobType)
		prevActive := s.active[def.ID]
		prevCron := s.crons[def.ID]
		prevVersion := s.versions[def.ID]

		if !shouldSchedule {
			s.removeLocked(def.ID)
			continue
		}
		if prevActive && prevCron == def.CronExpression && prevVersion == def.Version {
			continue
		}
		s.removeLocked(def.ID)
		if err := s.addLocked(def); err != nil {
			s.logger.Error("schedule job definition failed",
				"jobKey", def.JobKey, "err", err.Error())
			continue
		}
	}
	for id := range s.entries {
		if _, ok := seen[id]; !ok {
			s.removeLocked(id)
		}
	}
}

func (s *Scheduler) addLocked(def domainjobdef.JobDefinition) error {
	if err := domainjobdef.ValidateCronExpression(def.CronExpression); err != nil {
		return err
	}
	defCopy := def
	entryID, err := s.cron.AddFunc(def.CronExpression, func() {
		occ := s.clock.Now().In(s.loc)
		if err := s.enqueueOccurrence(context.Background(), defCopy, occ); err != nil {
			s.logger.Error("enqueue scheduled job failed",
				"jobKey", defCopy.JobKey, "err", err.Error())
		}
	})
	if err != nil {
		return err
	}
	s.entries[def.ID] = entryID
	s.versions[def.ID] = def.Version
	s.crons[def.ID] = def.CronExpression
	s.active[def.ID] = true
	return nil
}

func (s *Scheduler) removeLocked(id uuid.UUID) {
	if entryID, ok := s.entries[id]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, id)
	}
	delete(s.versions, id)
	delete(s.crons, id)
	delete(s.active, id)
}

func (s *Scheduler) enqueueOccurrence(ctx context.Context, def domainjobdef.JobDefinition, occurrence time.Time) error {
	if !s.caps.Allows(def.JobType) {
		return nil
	}
	local := occurrence.In(s.loc)
	dedup := domainjobdef.ScheduledOccurrenceDedupKey(def.JobKey, local)
	payload, err := buildScheduledPayload(def)
	if err != nil {
		return err
	}
	timeout := time.Duration(def.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = time.Hour
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, err = s.enqueuer.Enqueue(runCtx, appjobadmin.EnqueueRequest{
		Definition:       def,
		ExecutionType:    domainjobdef.ExecutionTypeScheduled,
		DeduplicationKey: dedup,
		Payload:          payload,
		AvailableAt:      s.clock.Now().UTC(),
		Now:              s.clock.Now().UTC(),
	})
	if err != nil {
		if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindConflict {
			return nil
		}
		return err
	}
	return nil
}

func buildScheduledPayload(def domainjobdef.JobDefinition) (json.RawMessage, error) {
	base := map[string]any{}
	raw := def.DefaultPayload
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(raw, &base); err != nil {
		base = map[string]any{}
	}
	base["timeoutSeconds"] = def.TimeoutSeconds
	return json.Marshal(base)
}

// ScheduledCountForTest returns active cron entries.
func (s *Scheduler) ScheduledCountForTest() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}
