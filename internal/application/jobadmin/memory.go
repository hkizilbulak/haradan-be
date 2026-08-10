package jobadmin

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainjobdef "github.com/hkizilbulak/haradan-be/internal/domain/jobdef"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

// MemoryStore is an in-memory Repository + UserReader for unit tests.
type MemoryStore struct {
	mu          sync.Mutex
	defs        map[uuid.UUID]domainjobdef.JobDefinition
	byKey       map[string]uuid.UUID
	history     []domainjobdef.JobExecution
	users       map[uuid.UUID]domainuser.User
	dedup       map[string]uuid.UUID
	enqueueHook func(EnqueueRequest) error
}

// NewMemoryStore builds an empty memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		defs:  map[uuid.UUID]domainjobdef.JobDefinition{},
		byKey: map[string]uuid.UUID{},
		users: map[uuid.UUID]domainuser.User{},
		dedup: map[string]uuid.UUID{},
	}
}

// SeedDefinition inserts a definition.
func (m *MemoryStore) SeedDefinition(def domainjobdef.JobDefinition) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if def.DefaultPayload == nil {
		def.DefaultPayload = json.RawMessage(`{}`)
	}
	m.defs[def.ID] = def
	m.byKey[def.JobKey] = def.ID
}

// SeedUser inserts a user.
func (m *MemoryStore) SeedUser(u domainuser.User) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.ID] = u
}

// SetEnqueueHook injects an enqueue failure for tests.
func (m *MemoryStore) SetEnqueueHook(fn func(EnqueueRequest) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueueHook = fn
}

// FindByID implements UserReader.
func (m *MemoryStore) FindByID(_ context.Context, id uuid.UUID) (domainuser.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return domainuser.User{}, apperr.NotFound("Kullanıcı bulunamadı.")
	}
	return u, nil
}

// ListDefinitions implements Repository.
func (m *MemoryStore) ListDefinitions(_ context.Context) ([]domainjobdef.JobDefinition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domainjobdef.JobDefinition, 0, len(m.defs))
	for _, d := range m.defs {
		out = append(out, d)
	}
	return out, nil
}

// GetDefinition implements Repository.
func (m *MemoryStore) GetDefinition(_ context.Context, id uuid.UUID) (domainjobdef.JobDefinition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.defs[id]
	if !ok {
		return domainjobdef.JobDefinition{}, apperr.NotFound(jobNotFoundMessage)
	}
	return d, nil
}

// UpdateDefinitionOptimistic implements Repository.
func (m *MemoryStore) UpdateDefinitionOptimistic(_ context.Context, def domainjobdef.JobDefinition, expectedVersion int) (domainjobdef.JobDefinition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.defs[def.ID]
	if !ok {
		return domainjobdef.JobDefinition{}, apperr.NotFound(jobNotFoundMessage)
	}
	if cur.Version != expectedVersion {
		return domainjobdef.JobDefinition{}, apperr.StaleVersion(staleVersionMessage)
	}
	def.Version = cur.Version + 1
	def.JobKey = cur.JobKey
	def.JobType = cur.JobType
	m.defs[def.ID] = def
	return def, nil
}

// ListHistory implements Repository.
func (m *MemoryStore) ListHistory(_ context.Context, definitionID uuid.UUID, f HistoryFilter) ([]domainjobdef.JobExecution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domainjobdef.JobExecution, 0)
	for _, h := range m.history {
		if h.JobDefinitionID == nil || *h.JobDefinitionID != definitionID {
			continue
		}
		if f.AfterCreatedAt != nil && f.AfterID != nil {
			if h.CreatedAt.After(*f.AfterCreatedAt) {
				continue
			}
			if h.CreatedAt.Equal(*f.AfterCreatedAt) && h.ID.String() >= f.AfterID.String() {
				continue
			}
		}
		out = append(out, h)
		if f.Limit > 0 && len(out) >= f.Limit {
			break
		}
	}
	return out, nil
}

// ListLastRuns returns the latest linked background job per definition (batch).
func (m *MemoryStore) ListLastRuns(_ context.Context, definitionIDs []uuid.UUID) (map[uuid.UUID]domainjobdef.LastRunSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	wanted := make(map[uuid.UUID]struct{}, len(definitionIDs))
	for _, id := range definitionIDs {
		wanted[id] = struct{}{}
	}
	out := make(map[uuid.UUID]domainjobdef.LastRunSummary)
	for _, h := range m.history {
		if h.JobDefinitionID == nil {
			continue
		}
		defID := *h.JobDefinitionID
		if _, ok := wanted[defID]; !ok {
			continue
		}
		if _, seen := out[defID]; seen {
			continue
		}
		runAt := h.CreatedAt
		if h.StartedAt != nil {
			runAt = *h.StartedAt
		}
		summary := domainjobdef.LastRunSummary{
			DefinitionID: defID,
			LastRunAt:    runAt,
			LastStatus:   h.Status,
		}
		if h.StartedAt != nil && h.CompletedAt != nil {
			ms := int(h.CompletedAt.Sub(*h.StartedAt).Milliseconds())
			if ms < 0 {
				ms = 0
			}
			summary.LastDurationMs = &ms
		}
		out[defID] = summary
	}
	return out, nil
}

// Enqueue implements Repository.
func (m *MemoryStore) Enqueue(_ context.Context, req EnqueueRequest) (EnqueueResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.enqueueHook != nil {
		if err := m.enqueueHook(req); err != nil {
			return EnqueueResult{}, err
		}
	}
	if id, ok := m.dedup[req.DeduplicationKey]; ok {
		return EnqueueResult{BackgroundJobID: id, AlreadyExists: true}, nil
	}
	jobID := uuid.New()
	defID := req.Definition.ID
	execType := req.ExecutionType
	queueType, _ := domainjobdef.QueueJobType(req.Definition.JobType)
	var tjkRunID *uuid.UUID
	if req.Definition.JobType == domainjobdef.JobTypeTJKSync {
		id := uuid.New()
		tjkRunID = &id
	}
	m.dedup[req.DeduplicationKey] = jobID
	m.history = append([]domainjobdef.JobExecution{{
		ID:                jobID,
		JobDefinitionID:   &defID,
		BackgroundJobType: queueType,
		Status:            "QUEUED",
		ExecutionType:     &execType,
		TriggeredByUserID: req.TriggeredByUserID,
		ReferenceDate:     req.ReferenceDate,
		AttemptCount:      0,
		MaxAttempts:       3,
		AvailableAt:       req.AvailableAt,
		CreatedAt:         req.Now,
		UpdatedAt:         req.Now,
	}}, m.history...)
	return EnqueueResult{BackgroundJobID: jobID, TJKSyncRunID: tjkRunID}, nil
}

// SeedExecution inserts a history row (newest-first prepend).
func (m *MemoryStore) SeedExecution(exec domainjobdef.JobExecution) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = append([]domainjobdef.JobExecution{exec}, m.history...)
}

// History returns recorded executions (test helper).
func (m *MemoryStore) History() []domainjobdef.JobExecution {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]domainjobdef.JobExecution(nil), m.history...)
}

// DedupKeys returns known dedup keys (test helper).
func (m *MemoryStore) DedupKeys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.dedup))
	for k := range m.dedup {
		out = append(out, k)
	}
	return out
}
