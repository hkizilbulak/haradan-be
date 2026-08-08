package jobadmin

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	domainjobdef "github.com/hkizilbulak/haradan-be/internal/domain/jobdef"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

// Clock supplies UTC instants.
type Clock interface {
	Now() time.Time
}

// UserReader loads actor accounts for ACTIVE ADMIN checks.
type UserReader interface {
	FindByID(ctx context.Context, id uuid.UUID) (domainuser.User, error)
}

// HistoryFilter is cursor pagination for job execution history.
type HistoryFilter struct {
	AfterCreatedAt *time.Time
	AfterID        *uuid.UUID
	Limit          int
}

// EnqueueRequest is a durable background job enqueue for a job definition run.
type EnqueueRequest struct {
	Definition        domainjobdef.JobDefinition
	ExecutionType     domainjobdef.ExecutionType
	TriggeredByUserID *uuid.UUID
	ReferenceDate     *time.Time
	DeduplicationKey  string
	Payload           json.RawMessage
	AvailableAt       time.Time
	Now               time.Time
}

// EnqueueResult is the identity of the enqueued work unit.
type EnqueueResult struct {
	BackgroundJobID uuid.UUID
	TJKSyncRunID    *uuid.UUID
	AlreadyExists   bool
}

// Repository persists job definitions and enqueues linked background jobs.
type Repository interface {
	ListDefinitions(ctx context.Context) ([]domainjobdef.JobDefinition, error)
	GetDefinition(ctx context.Context, id uuid.UUID) (domainjobdef.JobDefinition, error)
	UpdateDefinitionOptimistic(ctx context.Context, def domainjobdef.JobDefinition, expectedVersion int) (domainjobdef.JobDefinition, error)
	ListHistory(ctx context.Context, definitionID uuid.UUID, f HistoryFilter) ([]domainjobdef.JobExecution, error)
	ListLastRuns(ctx context.Context, definitionIDs []uuid.UUID) (map[uuid.UUID]domainjobdef.LastRunSummary, error)
	Enqueue(ctx context.Context, req EnqueueRequest) (EnqueueResult, error)
}

// ProviderCapabilities gates schedule/manual enqueue by infra readiness.
type ProviderCapabilities struct {
	TJKEnabled    bool
	B2Enabled     bool
	TinifyEnabled bool
}

// Allows reports whether jobType may be enqueued on this process.
func (c ProviderCapabilities) Allows(jobType domainjobdef.JobType) bool {
	switch jobType {
	case domainjobdef.JobTypeTJKSync:
		return c.TJKEnabled
	case domainjobdef.JobTypeMediaReconcile:
		return c.B2Enabled && c.TinifyEnabled
	case domainjobdef.JobTypePackageExpiryScan:
		return true
	default:
		return false
	}
}
