// Package jobdef holds BO-managed scheduled job definition aggregates and
// helpers aligned with hrd_job_definitions / hrd_background_jobs metadata.
package jobdef

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// JobType is a hrd_job_definitions.job_type value.
type JobType string

const (
	JobTypeTJKSync           JobType = "TJK_SYNC"
	JobTypePackageExpiryScan JobType = "PACKAGE_EXPIRY_SCAN"
	JobTypeMediaReconcile    JobType = "MEDIA_RECONCILE"
)

// Valid reports whether t is a known job definition type.
func (t JobType) Valid() bool {
	switch t {
	case JobTypeTJKSync, JobTypePackageExpiryScan, JobTypeMediaReconcile:
		return true
	}
	return false
}

// ExecutionType is a hrd_background_jobs.execution_type value.
type ExecutionType string

const (
	ExecutionTypeScheduled ExecutionType = "SCHEDULED"
	ExecutionTypeManual    ExecutionType = "MANUAL"
)

// Valid reports whether t is a known execution type.
func (t ExecutionType) Valid() bool {
	switch t {
	case ExecutionTypeScheduled, ExecutionTypeManual:
		return true
	}
	return false
}

// JobDefinition mirrors hrd_job_definitions.
type JobDefinition struct {
	ID                    uuid.UUID
	JobKey                string
	Name                  string
	Description           *string
	JobType               JobType
	CronExpression        string
	IsActive              bool
	TimeoutSeconds        int
	DefaultPayload        json.RawMessage
	SupportsReferenceDate bool
	Version               int
	CreatedAt             time.Time
	UpdatedAt             time.Time

	// Optional BO list/detail enrichment (not persisted on hrd_job_definitions).
	LastRunAt      *time.Time
	LastStatus     *string
	LastDurationMs *int
	NextRunAt      *time.Time
}

// LastRunSummary is the latest linked background job for a definition.
type LastRunSummary struct {
	DefinitionID   uuid.UUID
	LastRunAt      time.Time
	LastStatus     string
	LastDurationMs *int
}

// JobExecution is the BO-safe history projection of a hrd_background_jobs row
// linked to a job definition. Raw payloads are never included.
type JobExecution struct {
	ID                uuid.UUID
	JobDefinitionID   *uuid.UUID
	BackgroundJobType string
	Status            string
	ExecutionType     *ExecutionType
	TriggeredByUserID *uuid.UUID
	ReferenceDate     *time.Time
	AttemptCount      int
	MaxAttempts       int
	AvailableAt       time.Time
	StartedAt         *time.Time
	CompletedAt       *time.Time
	LastError         *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Istanbul returns Europe/Istanbul; panics only if the zone database is broken.
func Istanbul() *time.Location {
	loc, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		return time.FixedZone("Europe/Istanbul", 3*60*60)
	}
	return loc
}

// ScheduledOccurrenceDedupKey builds the multi-replica occurrence key:
// job_key + scheduled occurrence timestamp (RFC3339 in Europe/Istanbul).
func ScheduledOccurrenceDedupKey(jobKey string, occurrence time.Time) string {
	key := strings.TrimSpace(jobKey)
	local := occurrence.In(Istanbul())
	return key + ":" + local.Format(time.RFC3339)
}

// ManualRunDedupKey builds a dedup key for a manual run. When referenceDate is
// set it is date-scoped so the same calendar day cannot double-enqueue while
// QUEUED/LEASED/SUCCEEDED; otherwise the run id makes each click unique.
func ManualRunDedupKey(jobKey string, referenceDate *time.Time, runID uuid.UUID) string {
	key := strings.TrimSpace(jobKey)
	if referenceDate != nil {
		d := referenceDate.In(Istanbul())
		return key + ":MANUAL:" + d.Format("2006-01-02")
	}
	return key + ":MANUAL:" + runID.String()
}

// QueueJobType maps a definition job type onto the durable hrd_background_jobs
// job_type claimed by workers.
func QueueJobType(t JobType) (string, bool) {
	switch t {
	case JobTypeTJKSync:
		return "TJK_SYNC_BATCH", true
	case JobTypePackageExpiryScan:
		return "PACKAGE_EXPIRY_REMINDER_SCAN", true
	case JobTypeMediaReconcile:
		return "MEDIA_RECONCILE", true
	default:
		return "", false
	}
}
