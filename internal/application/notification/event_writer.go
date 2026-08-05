package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
)

const defaultJobMaxAttempts = 10

// EventWriter creates idempotent notification rows and enqueues fan-out jobs.
type EventWriter struct {
	repo        RuntimeRepository
	jobs        JobEnqueuer
	adverts     AdvertSnapshotReader
	packages    PackageSnapshotReader
	clock       Clock
	fanoutBatch int
	frontendURL string
}

// EventWriterConfig wires EventWriter dependencies.
type EventWriterConfig struct {
	Repo            RuntimeRepository
	Jobs            JobEnqueuer
	Adverts         AdvertSnapshotReader
	Packages        PackageSnapshotReader
	Clock           Clock
	FanoutBatchSize int
	// FrontendURL is rendered into the "frontendUrl" template variable for
	// every notification event. It must not contain CR/LF; config.Load already
	// enforces that for FRONTEND_URL.
	FrontendURL string
}

// NewEventWriter constructs an EventWriter.
func NewEventWriter(cfg EventWriterConfig) (*EventWriter, error) {
	if cfg.Repo == nil || cfg.Jobs == nil || cfg.Adverts == nil || cfg.Packages == nil {
		return nil, fmt.Errorf("event writer dependencies are required")
	}
	clock := cfg.Clock
	if clock == nil {
		clock = systemClock{}
	}
	batch := cfg.FanoutBatchSize
	if batch < 1 {
		batch = 100
	}
	return &EventWriter{
		repo:        cfg.Repo,
		jobs:        cfg.Jobs,
		adverts:     cfg.Adverts,
		packages:    cfg.Packages,
		clock:       clock,
		fanoutBatch: batch,
		frontendURL: cfg.FrontendURL,
	}, nil
}

// WritePackageAdvertPublishedInput carries package publish broadcast event data.
type WritePackageAdvertPublishedInput struct {
	AdvertID     uuid.UUID
	AssignmentID uuid.UUID
}

// WriteAdvancedAdvertPublishedInput is a historical alias for WritePackageAdvertPublishedInput.
type WriteAdvancedAdvertPublishedInput = WritePackageAdvertPublishedInput

// WriteUrgentAdvertActivatedInput carries urgent activation event data.
type WriteUrgentAdvertActivatedInput struct {
	AdvertID          uuid.UUID
	AssignmentID      uuid.UUID
	ActivationVersion int
}

// WritePackageExpiryInput carries package expiry reminder event data.
type WritePackageExpiryInput struct {
	AssignmentID uuid.UUID
	EndsAt       time.Time
	Offset       domainnotification.PackageExpiryDayOffset
	OwnerUserID  uuid.UUID
	// OwnerEmailVerified gates the email side of the reminder: the in-app
	// notification is always created for the owner, but an email job is only
	// queued when the owner has a verified address.
	OwnerEmailVerified bool
}

// WritePackageAdvertPublished creates a package advert published notification in tx
// (no-op when template inactive). New fan-out jobs use NOTIFICATION_FANOUT_PACKAGE_ADVERT.
func (w *EventWriter) WritePackageAdvertPublished(ctx context.Context, tx pgx.Tx, in WritePackageAdvertPublishedInput) error {
	return w.writeAdvertEvent(ctx, tx, domainnotification.TemplateEventTypePackageAdvertPublished,
		in.AdvertID, in.AssignmentID, nil, domainmedia.JobNotificationFanoutPackageAdvert)
}

// WriteAdvancedAdvertPublished is a historical alias for WritePackageAdvertPublished.
func (w *EventWriter) WriteAdvancedAdvertPublished(ctx context.Context, tx pgx.Tx, in WriteAdvancedAdvertPublishedInput) error {
	return w.WritePackageAdvertPublished(ctx, tx, in)
}

// WriteUrgentAdvertActivated creates an urgent activation notification in tx.
func (w *EventWriter) WriteUrgentAdvertActivated(ctx context.Context, tx pgx.Tx, in WriteUrgentAdvertActivatedInput) error {
	version := in.ActivationVersion
	return w.writeAdvertEvent(ctx, tx, domainnotification.TemplateEventTypeUrgentAdvertActivated,
		in.AdvertID, in.AssignmentID, &version, domainmedia.JobNotificationFanoutUrgentAdvert)
}

func (w *EventWriter) writeAdvertEvent(
	ctx context.Context,
	tx pgx.Tx,
	eventType domainnotification.EventType,
	advertID uuid.UUID,
	assignmentID uuid.UUID,
	activationVersion *int,
	jobType domainmedia.JobType,
) error {
	repo := w.repo.WithTx(tx)
	tmpl, ok, err := repo.FindActiveTemplateByEventType(ctx, eventType)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	advert, err := w.adverts.GetAdvertSnapshot(ctx, advertID)
	if err != nil {
		return err
	}
	asg, err := w.packages.GetAssignmentByID(ctx, assignmentID)
	if err != nil {
		return err
	}
	pkg, err := w.packages.GetPackageByID(ctx, asg.PackageID)
	if err != nil {
		return err
	}

	// URGENT_ADVERT_ACTIVATED is itself the urgent event; the row it refers to
	// is created earlier in this same (uncommitted) transaction, so it must not
	// be re-read through the non-tx packages snapshot reader (it would not be
	// visible yet). PACKAGE_ADVERT_PUBLISHED may safely check whether URGENT
	// is already active, since that activation (if any) was committed in a
	// prior transaction.
	isUrgent := eventType == domainnotification.TemplateEventTypeUrgentAdvertActivated
	if !isUrgent {
		if _, err := w.packages.FindActiveUrgent(ctx, advertID); err == nil {
			isUrgent = true
		} else if !isNotFoundErr(err) {
			return err
		}
	}

	vars := domainnotification.TemplateVars{
		"advertId":           advertID.String(),
		"advertTitle":        advert.Title,
		"packageCode":        string(pkg.Code),
		"packageDisplayName": pkg.DisplayName,
		"isUrgent":           strconv.FormatBool(isUrgent),
		"frontendUrl":        w.frontendURL,
	}

	title, err := domainnotification.RenderTitle(eventType, tmpl.InAppTitleTemplate, vars)
	if err != nil {
		return err
	}
	body, err := domainnotification.RenderBody(eventType, tmpl.InAppBodyTemplate, vars)
	if err != nil {
		return err
	}

	var eventKey string
	switch eventType {
	case domainnotification.TemplateEventTypePackageAdvertPublished:
		eventKey = domainnotification.PackageAdvertPublishedEventKey(advertID, assignmentID)
	case domainnotification.TemplateEventTypeUrgentAdvertActivated:
		if activationVersion == nil {
			return fmt.Errorf("urgent activation version is required")
		}
		eventKey = domainnotification.UrgentAdvertActivatedEventKey(advertID, *activationVersion)
	default:
		return fmt.Errorf("unsupported advert event type")
	}

	payload, err := json.Marshal(map[string]any{
		"advertId": advertID.String(),
	})
	if err != nil {
		return fmt.Errorf("marshal notification payload: %w", err)
	}

	now := w.clock.Now().UTC()
	n := domainnotification.Notification{
		ID:                  uuid.New(),
		EventType:           eventType,
		EventKey:            eventKey,
		AdvertID:            &advertID,
		PackageAssignmentID: &assignmentID,
		TemplateID:          &tmpl.ID,
		Title:               title,
		Body:                body,
		Payload:             payload,
		CreatedAt:           now,
	}
	created, err := repo.CreateNotificationEventIdempotent(ctx, n)
	if err != nil {
		return err
	}
	if !created {
		return nil
	}

	dedup := fanoutPageDedupKey(jobType, n.ID, nil)
	payloadJob, err := json.Marshal(fanoutJobPayload{NotificationID: n.ID.String()})
	if err != nil {
		return fmt.Errorf("marshal fanout job payload: %w", err)
	}
	// The fan-out job must commit or roll back with the notification row it
	// references: enqueue through the same tx, never through the pool.
	return enqueueJobIgnoringDuplicate(ctx, w.jobs.WithTx(tx), domainmedia.BackgroundJob{
		ID:               uuid.New(),
		JobType:          jobType,
		Status:           domainmedia.JobQueued,
		Payload:          payloadJob,
		DeduplicationKey: &dedup,
		AttemptCount:     0,
		MaxAttempts:      defaultJobMaxAttempts,
		AvailableAt:      now,
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	})
}

// WritePackageExpiryReminder creates an owner-scoped expiry notification in tx.
func (w *EventWriter) WritePackageExpiryReminder(ctx context.Context, tx pgx.Tx, in WritePackageExpiryInput) error {
	eventType, ok := domainnotification.EventTypeForExpiryOffset(in.Offset)
	if !ok {
		return fmt.Errorf("invalid expiry offset")
	}
	repo := w.repo.WithTx(tx)
	tmpl, active, err := repo.FindActiveTemplateByEventType(ctx, eventType)
	if err != nil {
		return err
	}
	if !active {
		return nil
	}

	asg, err := w.packages.GetAssignmentByID(ctx, in.AssignmentID)
	if err != nil {
		return err
	}
	pkg, err := w.packages.GetPackageByID(ctx, asg.PackageID)
	if err != nil {
		return err
	}
	advert, err := w.adverts.GetAdvertSnapshot(ctx, asg.AdvertID)
	if err != nil {
		return err
	}

	daysRemaining := "5"
	if in.Offset == domainnotification.PackageExpiryDayOffset1D {
		daysRemaining = "1"
	}
	endsAtStr := in.EndsAt.UTC().Format(time.RFC3339Nano)
	vars := domainnotification.TemplateVars{
		"advertId":            asg.AdvertID.String(),
		"advertTitle":         advert.Title,
		"packageCode":         string(pkg.Code),
		"packageDisplayName":  pkg.DisplayName,
		"endsAt":              endsAtStr,
		"daysRemaining":       daysRemaining,
		"campaignTitle":       "",
		"campaignDescription": "",
		"campaignCtaLabel":    "",
		"campaignCtaUrl":      "",
		"frontendUrl":         w.frontendURL,
	}
	campaign, campaignFound, err := repo.FindBestActiveCampaignForExpiry(ctx, eventType, asg.PackageID, w.clock.Now().UTC())
	if err != nil {
		return err
	}
	if campaignFound {
		vars["campaignTitle"] = campaign.Title
		vars["campaignDescription"] = derefOrEmpty(campaign.Description)
		vars["campaignCtaLabel"] = derefOrEmpty(campaign.CTALabel)
		vars["campaignCtaUrl"] = derefOrEmpty(campaign.CTAURL)
	}

	title, err := domainnotification.RenderTitle(eventType, tmpl.InAppTitleTemplate, vars)
	if err != nil {
		return err
	}
	body, err := domainnotification.RenderBody(eventType, tmpl.InAppBodyTemplate, vars)
	if err != nil {
		return err
	}

	eventKey := domainnotification.PackageExpiryEventKey(in.AssignmentID, in.EndsAt, in.Offset)
	payload, err := json.Marshal(map[string]any{
		"advertId":     asg.AdvertID.String(),
		"assignmentId": in.AssignmentID.String(),
		"ownerUserId":  in.OwnerUserID.String(),
		"endsAt":       endsAtStr,
		"offset":       string(in.Offset),
	})
	if err != nil {
		return fmt.Errorf("marshal notification payload: %w", err)
	}

	now := w.clock.Now().UTC()
	var campaignID *uuid.UUID
	if campaignFound {
		campaignID = &campaign.ID
	}

	n := domainnotification.Notification{
		ID:                  uuid.New(),
		EventType:           eventType,
		EventKey:            eventKey,
		AdvertID:            &asg.AdvertID,
		PackageAssignmentID: &in.AssignmentID,
		CampaignID:          campaignID,
		TemplateID:          &tmpl.ID,
		Title:               title,
		Body:                body,
		Payload:             payload,
		CreatedAt:           now,
	}
	created, err := repo.CreateNotificationEventIdempotent(ctx, n)
	if err != nil {
		return err
	}
	if !created {
		return nil
	}

	emailStatus := domainnotification.EmailStatusNotRequested
	var emailIdempotencyKey *string
	if in.OwnerEmailVerified {
		key := domainnotification.PackageExpiryEmailIdempotencyKey(n.ID, in.OwnerUserID)
		emailStatus = domainnotification.EmailStatusQueued
		emailIdempotencyKey = &key
	}
	state := domainnotification.UserNotificationState{
		UserID:              in.OwnerUserID,
		NotificationID:      n.ID,
		DeliveredAt:         now,
		EmailStatus:         emailStatus,
		EmailIdempotencyKey: emailIdempotencyKey,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if _, err := repo.InsertUserNotificationStates(ctx, []domainnotification.UserNotificationState{state}); err != nil {
		return err
	}
	if emailStatus != domainnotification.EmailStatusQueued {
		return nil
	}

	dedup := string(domainmedia.JobEmailSendPackageExpiryReminder) + ":" + n.ID.String() + ":" + in.OwnerUserID.String()
	jobPayload, err := json.Marshal(map[string]string{
		"notificationId": n.ID.String(),
		"userId":         in.OwnerUserID.String(),
	})
	if err != nil {
		return fmt.Errorf("marshal expiry email job payload: %w", err)
	}
	return enqueueJobIgnoringDuplicate(ctx, w.jobs.WithTx(tx), domainmedia.BackgroundJob{
		ID:               uuid.New(),
		JobType:          domainmedia.JobEmailSendPackageExpiryReminder,
		Status:           domainmedia.JobQueued,
		Payload:          jobPayload,
		DeduplicationKey: &dedup,
		AttemptCount:     0,
		MaxAttempts:      defaultJobMaxAttempts,
		AvailableAt:      now,
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	})
}

// EffectiveBroadcastAssignment returns the effective assignment at now whose
// package has broadcast_on_publish enabled, if any.
func EffectiveBroadcastAssignment(ctx context.Context, packages PackageSnapshotReader, advertID uuid.UUID, now time.Time) (PackageAssignmentSnapshot, domainpackaging.Package, bool, error) {
	asg, err := packages.GetEffectiveAssignment(ctx, advertID, now)
	if err != nil {
		if isNotFoundErr(err) {
			return PackageAssignmentSnapshot{}, domainpackaging.Package{}, false, nil
		}
		return PackageAssignmentSnapshot{}, domainpackaging.Package{}, false, err
	}
	pkg, err := packages.GetPackageByID(ctx, asg.PackageID)
	if err != nil {
		return PackageAssignmentSnapshot{}, domainpackaging.Package{}, false, err
	}
	if !pkg.EmitsPublishBroadcast() {
		return PackageAssignmentSnapshot{}, domainpackaging.Package{}, false, nil
	}
	return asg, pkg, true, nil
}

// EffectiveAdvancedAssignment is a historical alias for EffectiveBroadcastAssignment.
func EffectiveAdvancedAssignment(ctx context.Context, packages PackageSnapshotReader, advertID uuid.UUID, now time.Time) (PackageAssignmentSnapshot, domainpackaging.Package, bool, error) {
	return EffectiveBroadcastAssignment(ctx, packages, advertID, now)
}

// fanoutPageDedupKey is the deterministic dedup key for one fan-out page job:
// the first page of a notification uses "start"; continuation pages use the
// cursor they resume after.
func fanoutPageDedupKey(jobType domainmedia.JobType, notificationID uuid.UUID, afterUserID *uuid.UUID) string {
	after := "start"
	if afterUserID != nil {
		after = afterUserID.String()
	}
	return string(jobType) + ":" + notificationID.String() + ":after:" + after
}

// enqueueJobIgnoringDuplicate treats a dedup-key collision as success: the job
// the caller wanted is already queued (mirrors the media worker's
// enqueueIgnoringDuplicate so retries and re-derived continuation jobs stay
// idempotent instead of failing forever on CONFLICT).
func enqueueJobIgnoringDuplicate(ctx context.Context, enqueuer JobEnqueuer, job domainmedia.BackgroundJob) error {
	err := enqueuer.EnqueueJob(ctx, job)
	if err == nil {
		return nil
	}
	if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindConflict {
		return nil
	}
	return err
}

func derefOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
