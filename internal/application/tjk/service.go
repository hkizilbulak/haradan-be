package tjk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domain "github.com/hkizilbulak/haradan-be/internal/domain/tjk"
)

type Repository interface {
	CreateRunAndEnqueue(context.Context, domain.Run, time.Time) error
	ListRuns(context.Context, *string, *string, int) ([]domain.Run, bool, error)
	GetRun(context.Context, uuid.UUID) (domain.Run, error)
	RequestCancel(context.Context, uuid.UUID, int, time.Time) (domain.Run, error)
	ListItemErrors(context.Context, uuid.UUID, *string, *string, int) ([]domain.ItemError, bool, error)
	SetItemErrorStatus(context.Context, uuid.UUID, string, time.Time) (domain.ItemError, error)
}

// Config wires the admin TJK sync API service.
type Config struct {
	Repo    Repository
	Enabled bool
	Now     func() time.Time
}

type Service struct {
	repo    Repository
	enabled bool
	now     func() time.Time
}

func NewService(cfg Config) (*Service, error) {
	if cfg.Repo == nil {
		return nil, fmt.Errorf("TJK repository is required")
	}
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{repo: cfg.Repo, enabled: cfg.Enabled, now: now}, nil
}

const tjkDisabledMessage = "TJK senkronizasyonu şu anda kullanılamıyor."

func (s *Service) Trigger(ctx context.Context, actorID uuid.UUID, mode, source string) (domain.Run, error) {
	if !s.enabled {
		return domain.Run{}, apperr.DependencyUnavailable(tjkDisabledMessage)
	}
	mode, source = strings.ToUpper(strings.TrimSpace(mode)), strings.TrimSpace(source)
	if mode != "FULL" && mode != "INCREMENTAL" && mode != "RECONCILIATION" {
		return domain.Run{}, apperr.Validation("Geçersiz TJK senkronizasyon modu.")
	}
	if source != "TJK_HTTP" {
		return domain.Run{}, apperr.Validation("Geçersiz TJK kaynak adaptörü.")
	}
	now := s.now()
	run := domain.Run{ID: uuid.New(), Mode: mode, Status: domain.RunQueued, SourceAdapter: source, Scope: "HORSES", Checkpoint: []byte(`{"page":0}`), TriggerKind: "MANUAL", CreatedByUserID: &actorID, Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateRunAndEnqueue(ctx, run, now); err != nil {
		return domain.Run{}, err
	}
	return run, nil
}

func (s *Service) List(ctx context.Context, cursor, status *string, limit int) ([]domain.Run, bool, error) {
	return s.repo.ListRuns(ctx, cursor, status, normalizedLimit(limit))
}
func (s *Service) Get(ctx context.Context, id uuid.UUID) (domain.Run, error) {
	return s.repo.GetRun(ctx, id)
}
func (s *Service) Cancel(ctx context.Context, id uuid.UUID, version int) (domain.Run, error) {
	if version < 1 {
		return domain.Run{}, apperr.Validation("expectedVersion geçersiz.")
	}
	return s.repo.RequestCancel(ctx, id, version, s.now())
}
func (s *Service) ListErrors(ctx context.Context, runID uuid.UUID, cursor, status *string, limit int) ([]domain.ItemError, bool, error) {
	return s.repo.ListItemErrors(ctx, runID, cursor, status, normalizedLimit(limit))
}
func (s *Service) ResolveError(ctx context.Context, id uuid.UUID) (domain.ItemError, error) {
	return s.repo.SetItemErrorStatus(ctx, id, "RESOLVED", s.now())
}
func (s *Service) IgnoreError(ctx context.Context, id uuid.UUID) (domain.ItemError, error) {
	return s.repo.SetItemErrorStatus(ctx, id, "IGNORED", s.now())
}
func normalizedLimit(v int) int {
	if v <= 0 {
		return 20
	}
	if v > 100 {
		return 100
	}
	return v
}
