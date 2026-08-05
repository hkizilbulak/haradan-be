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

type HorseInput struct {
	Number, Name, Race, Sire, Dam string
}
