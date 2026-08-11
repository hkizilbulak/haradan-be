package tjk

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type RunStatus string

const (
	RunQueued         RunStatus = "QUEUED"
	RunRunning        RunStatus = "RUNNING"
	RunSucceeded      RunStatus = "SUCCEEDED"
	RunPartialSuccess RunStatus = "PARTIAL_SUCCESS"
	RunFailed         RunStatus = "FAILED"
	RunCancelled      RunStatus = "CANCELLED"
)

type Run struct {
	ID                                                                                               uuid.UUID
	Mode                                                                                             string
	Status                                                                                           RunStatus
	SourceAdapter                                                                                    string
	Scope                                                                                            string
	Checkpoint                                                                                       json.RawMessage
	TriggerKind                                                                                      string
	CreatedByUserID                                                                                  *uuid.UUID
	CancelRequestedAt, CancelledAt, StartedAt, CompletedAt                                           *time.Time
	TotalCount, CreatedCount, UpdatedCount, UnchangedCount, SkippedCount, FailedCount, ConflictCount int
	LastErrorSummary                                                                                 *string
	Version                                                                                          int
	CreatedAt, UpdatedAt                                                                             time.Time
}

type ItemError struct {
	ID, RunID                   uuid.UUID
	TJKNumber                   *string
	HorseID                     *uuid.UUID
	ErrorClass, Status, Message string
	CreatedAt                   time.Time
	ResolvedAt                  *time.Time
}

// HorseInput is the normalized TJK sync upsert payload. Detail holds only
// controlled domain-detail JSONB keys (pedigree/siblings/statistics); advert
// rows are never written by this path.
type HorseInput struct {
	Number, Name, Race, Sire, Dam string
	BirthYear                     *int
	Gender                        *string
	Coat                          *string
	Detail                        json.RawMessage
	EnrichmentIssues              []EnrichmentIssue
}

// EnrichmentIssue records a best-effort provider subrequest that did not
// complete. Messages are deliberately safe summaries; raw provider responses
// and URLs are never persisted.
type EnrichmentIssue struct {
	Component string
	Message   string
}

// PageResult is the explicit source-page contract used by the sync worker.
// EndOfSource may only be true when the adapter recognized provider-owned EOF
// evidence. An empty Horses slice by itself is never an EOF signal.
type PageResult struct {
	Horses       []HorseInput
	EndOfSource  bool
	Fingerprint  string
	SourceTotal  *int
	SkippedCount int
}

// PageJob is the durable identity of one TJK source page.
type PageJob struct {
	ID   uuid.UUID
	Page int
}
