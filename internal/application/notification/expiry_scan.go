package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
)

const (
	dailyExpiryScanDedupPrefix       = "PACKAGE_EXPIRY_SCAN"
	packageExpiredUrgentReason       = "PACKAGE_EXPIRED"
	defaultExpiryScanHour            = 9
	maxExpireDueAssignmentBatchLoops = 1000
)

// ExpiryScanConfig configures package expiry scan jobs.
type ExpiryScanConfig struct {
	Writer    *EventWriter
	Repo      RuntimeRepository
	Jobs      JobEnqueuer
	Adverts   AdvertSnapshotReader
	Users     VerifiedUserReader
	Clock     Clock
	Timezone  *time.Location
	BatchSize int
	// ScanHour is the local hour (0-23) at which the daily scan job becomes
	// available for a worker to claim.
	ScanHour int
}

// ExpiryScanService scans assignments and emits expiry reminders.
type ExpiryScanService struct {
	writer    *EventWriter
	repo      RuntimeRepository
	jobs      JobEnqueuer
	adverts   AdvertSnapshotReader
	users     VerifiedUserReader
	clock     Clock
	loc       *time.Location
	batchSize int
	scanHour  int
}

// NewExpiryScanService constructs an ExpiryScanService.
func NewExpiryScanService(cfg ExpiryScanConfig) (*ExpiryScanService, error) {
	if cfg.Writer == nil || cfg.Repo == nil || cfg.Jobs == nil || cfg.Adverts == nil || cfg.Users == nil {
		return nil, fmt.Errorf("expiry scan dependencies are required")
	}
	clock := cfg.Clock
	if clock == nil {
		clock = systemClock{}
	}
	loc := cfg.Timezone
	if loc == nil {
		var err error
		loc, err = time.LoadLocation("Europe/Istanbul")
		if err != nil {
			return nil, fmt.Errorf("load expiry timezone: %w", err)
		}
	}
	batch := cfg.BatchSize
	if batch < 1 {
		batch = 100
	}
	scanHour := cfg.ScanHour
	if scanHour < 0 || scanHour > 23 {
		scanHour = defaultExpiryScanHour
	}
	return &ExpiryScanService{
		writer: cfg.Writer, repo: cfg.Repo, jobs: cfg.Jobs, adverts: cfg.Adverts, users: cfg.Users,
		clock: clock, loc: loc, batchSize: batch, scanHour: scanHour,
	}, nil
}

// ProcessExpiryScan handles PACKAGE_EXPIRY_REMINDER_SCAN jobs. The initial
// invocation (no offset/cursor in the payload) also expires assignments whose
// ends_at has passed and deactivates any URGENT feature they were carrying,
// before dispatching the 10D/3D reminder scans.
func (s *ExpiryScanService) ProcessExpiryScan(ctx context.Context, payload json.RawMessage) error {
	var in struct {
		Offset            string `json:"offset"`
		AfterAssignmentID string `json:"afterAssignmentId,omitempty"`
	}
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &in)
	}
	offset := domainnotification.PackageExpiryDayOffset(in.Offset)
	if !offset.Valid() {
		if err := s.ExpireDueAssignments(ctx); err != nil {
			return err
		}
		for _, off := range []domainnotification.PackageExpiryDayOffset{
			domainnotification.PackageExpiryDayOffset10D,
			domainnotification.PackageExpiryDayOffset3D,
		} {
			if err := s.scanOffset(ctx, off, nil); err != nil {
				return err
			}
		}
		return nil
	}
	var after *uuid.UUID
	if in.AfterAssignmentID != "" {
		id, err := uuid.Parse(in.AfterAssignmentID)
		if err != nil {
			return fmt.Errorf("invalid after assignment id")
		}
		after = &id
	}
	return s.scanOffset(ctx, offset, after)
}

// ExpireDueAssignments expires ACTIVE assignments whose ends_at has passed and
// deactivates any URGENT feature activation they carried (reason
// PACKAGE_EXPIRED). It processes batches under FOR UPDATE SKIP LOCKED so
// concurrent scheduler instances partition the backlog instead of colliding,
// looping until fewer than a full batch remains.
func (s *ExpiryScanService) ExpireDueAssignments(ctx context.Context) error {
	now := s.clock.Now().UTC()
	for i := 0; i < maxExpireDueAssignmentBatchLoops; i++ {
		processed, err := s.expireDueAssignmentsBatch(ctx, now)
		if err != nil {
			return err
		}
		if processed < s.batchSize {
			return nil
		}
	}
	return nil
}

func (s *ExpiryScanService) expireDueAssignmentsBatch(ctx context.Context, now time.Time) (int, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	repo := s.repo.WithTx(tx)
	assignments, err := repo.ListActiveAssignmentsPastEndsAt(ctx, now, s.batchSize)
	if err != nil {
		return 0, err
	}
	for _, asg := range assignments {
		if err := repo.MarkAssignmentExpired(ctx, asg.ID, now, now); err != nil {
			return 0, err
		}
		if _, err := repo.DeactivateActiveUrgentForAdvert(ctx, asg.AdvertID, packageExpiredUrgentReason, now, now); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(assignments), nil
}

func (s *ExpiryScanService) scanOffset(ctx context.Context, offset domainnotification.PackageExpiryDayOffset, after *uuid.UUID) error {
	now := s.clock.Now().UTC()
	targetDay := domainnotification.PackageExpiryTargetDay(now, s.loc, offset)
	assignments, err := s.repo.ListAssignmentsExpiringOnLocalDay(ctx, targetDay, s.loc, after, s.batchSize+1)
	if err != nil {
		return err
	}
	hasMore := len(assignments) > s.batchSize
	if hasMore {
		assignments = assignments[:s.batchSize]
	}
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, asg := range assignments {
		if asg.EndsAt == nil {
			continue
		}
		advert, err := s.adverts.GetAdvertSnapshot(ctx, asg.AdvertID)
		if err != nil {
			return err
		}
		ownerVerified := false
		if owner, err := s.users.FindByID(ctx, advert.OwnerUserID); err == nil {
			ownerVerified = owner.IsActive() && owner.EmailVerifiedAt != nil && strings.TrimSpace(owner.Email) != ""
		} else if !isNotFoundErr(err) {
			return err
		}
		if err := s.writer.WritePackageExpiryReminder(ctx, tx, WritePackageExpiryInput{
			AssignmentID:       asg.ID,
			EndsAt:             *asg.EndsAt,
			Offset:             offset,
			OwnerUserID:        advert.OwnerUserID,
			OwnerEmailVerified: ownerVerified,
		}); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	if hasMore {
		last := assignments[len(assignments)-1]
		contPayload, err := json.Marshal(map[string]string{
			"offset":            string(offset),
			"afterAssignmentId": last.ID.String(),
		})
		if err != nil {
			return err
		}
		dedup := string(domainmedia.JobPackageExpiryReminderScan) + ":" + string(offset) + ":" + last.ID.String()
		now := s.clock.Now().UTC()
		return enqueueJobIgnoringDuplicate(ctx, s.jobs, domainmedia.BackgroundJob{
			ID:               uuid.New(),
			JobType:          domainmedia.JobPackageExpiryReminderScan,
			Status:           domainmedia.JobQueued,
			Payload:          contPayload,
			DeduplicationKey: &dedup,
			MaxAttempts:      defaultJobMaxAttempts,
			AvailableAt:      now,
			Version:          1,
			CreatedAt:        now,
			UpdatedAt:        now,
		})
	}
	return nil
}

// EnqueueDailyExpiryScan enqueues today's scan job, deduplicated per local
// calendar day, available starting at ScanHour local time (so a job enqueued
// earlier in the day cannot be claimed before the configured hour).
func (s *ExpiryScanService) EnqueueDailyExpiryScan(ctx context.Context) error {
	now := s.clock.Now().UTC()
	local := now.In(s.loc)
	scanAt := time.Date(local.Year(), local.Month(), local.Day(), s.scanHour, 0, 0, 0, s.loc)
	dedup := dailyExpiryScanDedupPrefix + ":" + local.Format("2006-01-02")
	payload := json.RawMessage(`{}`)
	return enqueueJobIgnoringDuplicate(ctx, s.jobs, domainmedia.BackgroundJob{
		ID:               uuid.New(),
		JobType:          domainmedia.JobPackageExpiryReminderScan,
		Status:           domainmedia.JobQueued,
		Payload:          payload,
		DeduplicationKey: &dedup,
		MaxAttempts:      defaultJobMaxAttempts,
		AvailableAt:      scanAt.UTC(),
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	})
}
