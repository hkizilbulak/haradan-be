package jobadmin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainjobdef "github.com/hkizilbulak/haradan-be/internal/domain/jobdef"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

const (
	forbiddenMessage           = "Bu işlem için yetkiniz yok."
	jobNotFoundMessage         = "Görev tanımı bulunamadı."
	staleVersionMessage        = "Görev tanımı başka bir işlem tarafından güncellendi."
	invalidCursorMessage       = "Geçersiz sayfalama imleci."
	providerDisabledMessage    = "Bu görev için gerekli sağlayıcı etkin değil."
	referenceDateUnsupported   = "Bu görev referans tarih desteklemiyor."
	invalidReferenceDateMsg    = "Referans tarih YYYY-MM-DD formatında olmalıdır."
	futureReferenceDateMessage = "Referans tarih gelecekte olamaz."
	defaultHistoryLimit        = 20
	maxHistoryLimit            = 100
	minTimeoutSeconds          = 1
	maxTimeoutSeconds          = 86400
)

// Service implements BO job definition management.
type Service struct {
	repo  Repository
	users UserReader
	caps  ProviderCapabilities
	clock Clock
	loc   *time.Location
}

// Config wires jobadmin dependencies.
type Config struct {
	Repo     Repository
	Users    UserReader
	Caps     ProviderCapabilities
	Clock    Clock
	Location *time.Location
}

// NewService constructs the jobadmin service.
func NewService(cfg Config) (*Service, error) {
	if cfg.Repo == nil || cfg.Users == nil {
		return nil, fmt.Errorf("jobadmin dependencies are required")
	}
	clock := cfg.Clock
	if clock == nil {
		clock = systemClock{}
	}
	loc := cfg.Location
	if loc == nil {
		loc = domainjobdef.Istanbul()
	}
	return &Service{repo: cfg.Repo, users: cfg.Users, caps: cfg.Caps, clock: clock, loc: loc}, nil
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// ListJobs returns all job definitions (ACTIVE ADMIN).
func (s *Service) ListJobs(ctx context.Context, actorUserID uuid.UUID) ([]domainjobdef.JobDefinition, error) {
	if err := s.requireAdmin(ctx, actorUserID); err != nil {
		return nil, err
	}
	defs, err := s.repo.ListDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	return s.enrichDefinitions(ctx, defs)
}

// GetJob returns one job definition (ACTIVE ADMIN).
func (s *Service) GetJob(ctx context.Context, actorUserID, id uuid.UUID) (domainjobdef.JobDefinition, error) {
	if err := s.requireAdmin(ctx, actorUserID); err != nil {
		return domainjobdef.JobDefinition{}, err
	}
	def, err := s.repo.GetDefinition(ctx, id)
	if err != nil {
		return domainjobdef.JobDefinition{}, err
	}
	out, err := s.enrichDefinitions(ctx, []domainjobdef.JobDefinition{def})
	if err != nil {
		return domainjobdef.JobDefinition{}, err
	}
	return out[0], nil
}

// CreateJobInput holds parameters for registering a new job definition.
type CreateJobInput struct {
	ActorUserID           uuid.UUID
	JobKey                string
	Name                  string
	Description           *string
	JobType               domainjobdef.JobType
	CronExpression        string
	IsActive              bool
	TimeoutSeconds        int
	SupportsReferenceDate bool
	SupportsPageNumber    *bool
}

// CreateJob creates a new job definition (ACTIVE ADMIN).
func (s *Service) CreateJob(ctx context.Context, in CreateJobInput) (domainjobdef.JobDefinition, error) {
	if err := s.requireAdmin(ctx, in.ActorUserID); err != nil {
		return domainjobdef.JobDefinition{}, err
	}
	key := strings.TrimSpace(in.JobKey)
	if key == "" {
		return domainjobdef.JobDefinition{}, apperr.Validation("İş anahtarı (key) zorunludur.", apperr.FieldError{
			Field: "key", Message: "İş anahtarı (key) zorunludur.",
		})
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return domainjobdef.JobDefinition{}, apperr.Validation("Görev adı (name) zorunludur.", apperr.FieldError{
			Field: "name", Message: "Görev adı (name) zorunludur.",
		})
	}
	if !in.JobType.Valid() {
		return domainjobdef.JobDefinition{}, apperr.Validation("Geçersiz görev tipi (jobType).", apperr.FieldError{
			Field: "jobType", Message: "Geçerli bir görev tipi seçiniz (TJK_SYNC, PACKAGE_EXPIRY_SCAN, MEDIA_RECONCILE).",
		})
	}
	cronExpr := strings.TrimSpace(in.CronExpression)
	if err := domainjobdef.ValidateCronExpression(cronExpr); err != nil {
		return domainjobdef.JobDefinition{}, apperr.Validation("Cron ifadesi geçersiz.", apperr.FieldError{
			Field: "cronExpression", Message: "Geçerli 6 alanlı (saniye dahil) cron ifadesi girin. Örn: 0 0 9 * * *",
		})
	}
	timeout := in.TimeoutSeconds
	if timeout <= 0 {
		timeout = 3600
	}
	if timeout < minTimeoutSeconds || timeout > maxTimeoutSeconds {
		return domainjobdef.JobDefinition{}, apperr.Validation("Zaman aşımı geçersiz.", apperr.FieldError{
			Field: "timeoutSeconds", Message: "timeoutSeconds 1 ile 86400 arasında olmalıdır.",
		})
	}

	payload := json.RawMessage(`{}`)
	if in.SupportsPageNumber != nil {
		b, _ := json.Marshal(map[string]any{"supports_page_number": *in.SupportsPageNumber})
		payload = b
	}

	now := s.clock.Now().UTC()
	def := domainjobdef.JobDefinition{
		ID:                    uuid.New(),
		JobKey:                key,
		Name:                  name,
		Description:           in.Description,
		JobType:               in.JobType,
		CronExpression:        cronExpr,
		IsActive:              in.IsActive,
		TimeoutSeconds:        timeout,
		DefaultPayload:        payload,
		SupportsReferenceDate: in.SupportsReferenceDate,
		Version:               1,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	created, err := s.repo.CreateDefinition(ctx, def)
	if err != nil {
		return domainjobdef.JobDefinition{}, err
	}
	out, err := s.enrichDefinitions(ctx, []domainjobdef.JobDefinition{created})
	if err != nil {
		return created, nil
	}
	return out[0], nil
}

// UpdateJobInput is the optimistic patch for a job definition.
type UpdateJobInput struct {
	ActorUserID           uuid.UUID
	JobID                 uuid.UUID
	ExpectedVersion       int
	Name                  *string
	Description           *string
	CronExpression        *string
	IsActive              *bool
	TimeoutSeconds        *int
	SupportsReferenceDate *bool
	SupportsPageNumber    *bool
}

// UpdateJob updates cron/active/timeout with optimistic concurrency.
// job_key and job_type are immutable.
func (s *Service) UpdateJob(ctx context.Context, in UpdateJobInput) (domainjobdef.JobDefinition, error) {
	if err := s.requireAdmin(ctx, in.ActorUserID); err != nil {
		return domainjobdef.JobDefinition{}, err
	}
	if in.ExpectedVersion < 1 {
		return domainjobdef.JobDefinition{}, apperr.Validation("expectedVersion geçersiz.", apperr.FieldError{
			Field: "expectedVersion", Message: "expectedVersion 1 veya daha büyük olmalıdır.",
		})
	}
	current, err := s.repo.GetDefinition(ctx, in.JobID)
	if err != nil {
		return domainjobdef.JobDefinition{}, err
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return domainjobdef.JobDefinition{}, apperr.Validation("Görev adı (name) zorunludur.", apperr.FieldError{
				Field: "name", Message: "Görev adı (name) zorunludur.",
			})
		}
		current.Name = name
	}
	if in.Description != nil {
		current.Description = in.Description
	}
	if in.CronExpression != nil {
		expr := strings.TrimSpace(*in.CronExpression)
		if err := domainjobdef.ValidateCronExpression(expr); err != nil {
			return domainjobdef.JobDefinition{}, apperr.Validation("Cron ifadesi geçersiz.", apperr.FieldError{
				Field: "cronExpression", Message: "Geçerli 6 alanlı (saniye dahil) cron ifadesi girin.",
			})
		}
		current.CronExpression = expr
	}
	if in.IsActive != nil {
		current.IsActive = *in.IsActive
	}
	if in.TimeoutSeconds != nil {
		if *in.TimeoutSeconds < minTimeoutSeconds || *in.TimeoutSeconds > maxTimeoutSeconds {
			return domainjobdef.JobDefinition{}, apperr.Validation("Zaman aşımı geçersiz.", apperr.FieldError{
				Field: "timeoutSeconds", Message: "timeoutSeconds 1 ile 86400 arasında olmalıdır.",
			})
		}
		current.TimeoutSeconds = *in.TimeoutSeconds
	}
	if in.SupportsReferenceDate != nil {
		current.SupportsReferenceDate = *in.SupportsReferenceDate
	}
	if in.SupportsPageNumber != nil {
		var pl map[string]any
		if len(current.DefaultPayload) > 0 {
			_ = json.Unmarshal(current.DefaultPayload, &pl)
		}
		if pl == nil {
			pl = make(map[string]any)
		}
		pl["supports_page_number"] = *in.SupportsPageNumber
		b, _ := json.Marshal(pl)
		current.DefaultPayload = b
	}
	current.UpdatedAt = s.clock.Now().UTC()
	updated, err := s.repo.UpdateDefinitionOptimistic(ctx, current, in.ExpectedVersion)
	if err != nil {
		return domainjobdef.JobDefinition{}, err
	}
	out, err := s.enrichDefinitions(ctx, []domainjobdef.JobDefinition{updated})
	if err != nil {
		return domainjobdef.JobDefinition{}, err
	}
	return out[0], nil
}

// RunJobInput is a manual job trigger request.
type RunJobInput struct {
	ActorUserID   uuid.UUID
	JobID         uuid.UUID
	ReferenceDate *string // YYYY-MM-DD, optional
	PageNumber    *int    // >= 0, optional starting page
}

// RunJobResult identifies the enqueued work.
type RunJobResult struct {
	BackgroundJobID uuid.UUID
	TJKSyncRunID    *uuid.UUID
	AlreadyExists   bool
}

// RunJob enqueues a MANUAL execution for the definition.
func (s *Service) RunJob(ctx context.Context, in RunJobInput) (RunJobResult, error) {
	if err := s.requireAdmin(ctx, in.ActorUserID); err != nil {
		return RunJobResult{}, err
	}
	def, err := s.repo.GetDefinition(ctx, in.JobID)
	if err != nil {
		return RunJobResult{}, err
	}
	if !s.caps.Allows(def.JobType) {
		return RunJobResult{}, apperr.InvalidState(providerDisabledMessage)
	}
	if in.PageNumber != nil && *in.PageNumber < 0 {
		return RunJobResult{}, apperr.Validation("Sayfa numarası 0 veya daha büyük olmalıdır.", apperr.FieldError{
			Field: "pageNumber", Message: "Sayfa numarası 0 veya daha büyük olmalıdır.",
		})
	}
	var refDate *time.Time
	if in.ReferenceDate != nil && strings.TrimSpace(*in.ReferenceDate) != "" {
		if !def.SupportsReferenceDate {
			return RunJobResult{}, apperr.Validation(referenceDateUnsupported, apperr.FieldError{
				Field: "referenceDate", Message: referenceDateUnsupported,
			})
		}
		parsed, err := parseReferenceDate(strings.TrimSpace(*in.ReferenceDate), s.loc)
		if err != nil {
			return RunJobResult{}, apperr.Validation(invalidReferenceDateMsg, apperr.FieldError{
				Field: "referenceDate", Message: invalidReferenceDateMsg,
			})
		}
		if isFutureDate(parsed, s.clock.Now().UTC(), s.loc) {
			return RunJobResult{}, apperr.Validation(futureReferenceDateMessage, apperr.FieldError{
				Field: "referenceDate", Message: futureReferenceDateMessage,
			})
		}
		refDate = &parsed
	}

	now := s.clock.Now().UTC()
	runID := uuid.New()
	dedup := domainjobdef.ManualRunDedupKey(def.JobKey, refDate, runID)
	payload, err := buildRunPayload(def, refDate, in.PageNumber)
	if err != nil {
		return RunJobResult{}, apperr.Internal(err)
	}
	actor := in.ActorUserID
	out, err := s.repo.Enqueue(ctx, EnqueueRequest{
		Definition:        def,
		ExecutionType:     domainjobdef.ExecutionTypeManual,
		TriggeredByUserID: &actor,
		ReferenceDate:     refDate,
		DeduplicationKey:  dedup,
		Payload:           payload,
		AvailableAt:       now,
		Now:               now,
	})
	if err != nil {
		return RunJobResult{}, err
	}
	return RunJobResult{
		BackgroundJobID: out.BackgroundJobID,
		TJKSyncRunID:    out.TJKSyncRunID,
		AlreadyExists:   out.AlreadyExists,
	}, nil
}

// CancelJob cancels active background jobs for a job definition.
func (s *Service) CancelJob(ctx context.Context, actorUserID, jobID uuid.UUID) error {
	if err := s.requireAdmin(ctx, actorUserID); err != nil {
		return err
	}
	if _, err := s.repo.GetDefinition(ctx, jobID); err != nil {
		return err
	}
	return s.repo.CancelActiveJob(ctx, jobID, s.clock.Now().UTC())
}

// ListHistoryResult is cursor-paginated sanitized history.
type ListHistoryResult struct {
	Items      []domainjobdef.JobExecution
	NextCursor *string
	HasMore    bool
}

// ListHistory returns sanitized execution history for a definition.
func (s *Service) ListHistory(ctx context.Context, actorUserID, jobID uuid.UUID, cursor *string, limit *int) (ListHistoryResult, error) {
	if err := s.requireAdmin(ctx, actorUserID); err != nil {
		return ListHistoryResult{}, err
	}
	if _, err := s.repo.GetDefinition(ctx, jobID); err != nil {
		return ListHistoryResult{}, err
	}
	n := defaultHistoryLimit
	if limit != nil && *limit > 0 {
		n = *limit
	}
	if n > maxHistoryLimit {
		n = maxHistoryLimit
	}
	var filter HistoryFilter
	filter.Limit = n + 1
	if cursor != nil && strings.TrimSpace(*cursor) != "" {
		createdAt, id, err := decodeHistoryCursor(strings.TrimSpace(*cursor))
		if err != nil {
			return ListHistoryResult{}, apperr.Validation(invalidCursorMessage)
		}
		filter.AfterCreatedAt = &createdAt
		filter.AfterID = &id
	}
	rows, err := s.repo.ListHistory(ctx, jobID, filter)
	if err != nil {
		return ListHistoryResult{}, err
	}
	hasMore := len(rows) > n
	if hasMore {
		rows = rows[:n]
	}
	for i := range rows {
		rows[i].LastError = domainjobdef.SanitizeLastError(rows[i].LastError)
	}
	out := ListHistoryResult{Items: rows, HasMore: hasMore}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		c := encodeHistoryCursor(last.CreatedAt, last.ID)
		out.NextCursor = &c
	}
	return out, nil
}

func (s *Service) requireAdmin(ctx context.Context, actorUserID uuid.UUID) error {
	actor, err := s.users.FindByID(ctx, actorUserID)
	if err != nil {
		return err
	}
	if actor.Role != domainuser.RoleAdmin || !actor.IsActive() {
		return apperr.Forbidden(apperr.CodeForbidden, forbiddenMessage)
	}
	return nil
}

func (s *Service) enrichDefinitions(ctx context.Context, defs []domainjobdef.JobDefinition) ([]domainjobdef.JobDefinition, error) {
	if len(defs) == 0 {
		return defs, nil
	}
	ids := make([]uuid.UUID, len(defs))
	for i := range defs {
		ids[i] = defs[i].ID
	}
	lastRuns, err := s.repo.ListLastRuns(ctx, ids)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	for i := range defs {
		if lr, ok := lastRuns[defs[i].ID]; ok {
			at := lr.LastRunAt
			status := lr.LastStatus
			defs[i].LastRunAt = &at
			defs[i].LastStatus = &status
			defs[i].LastDurationMs = lr.LastDurationMs
		}
		defs[i].NextRunAt = domainjobdef.NextRunAt(defs[i], now)
	}
	return defs, nil
}

func parseReferenceDate(v string, loc *time.Location) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", v, loc)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

func isFutureDate(d, nowUTC time.Time, loc *time.Location) bool {
	today := nowUTC.In(loc)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)
	day := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
	return day.After(today)
}

func buildRunPayload(def domainjobdef.JobDefinition, refDate *time.Time, pageNumber *int) (json.RawMessage, error) {
	base := map[string]any{}
	raw := def.DefaultPayload
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(raw, &base); err != nil {
		base = map[string]any{}
	}
	base["timeoutSeconds"] = def.TimeoutSeconds
	if refDate != nil {
		base["referenceDate"] = refDate.In(domainjobdef.Istanbul()).Format("2006-01-02")
	}
	if pageNumber != nil && *pageNumber >= 0 {
		base["page"] = *pageNumber
		base["pageNumber"] = *pageNumber
	}
	return json.Marshal(base)
}

func encodeHistoryCursor(createdAt time.Time, id uuid.UUID) string {
	raw := createdAt.UTC().Format(time.RFC3339Nano) + "|" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeHistoryCursor(cursor string) (time.Time, uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, fmt.Errorf("bad cursor")
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	return ts, id, nil
}
