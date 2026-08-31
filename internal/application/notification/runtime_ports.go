package notification

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domaincampaign "github.com/hkizilbulak/haradan-be/internal/domain/campaign"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

// JobEnqueuer inserts durable background jobs with deduplication. WithTx scopes
// the enqueue to a caller transaction so a notification row and its fan-out job
// commit or roll back together (no orphaned outbox job on rollback).
type JobEnqueuer interface {
	WithTx(tx pgx.Tx) JobEnqueuer
	EnqueueJob(ctx context.Context, job domainmedia.BackgroundJob) error
}

// AdvertSnapshot holds advert fields needed for notification rendering.
type AdvertSnapshot struct {
	ID          int64
	OwnerUserID uuid.UUID
	Title       string
	Status      string
}

// PackageAssignmentSnapshot holds assignment fields for notifications.
type PackageAssignmentSnapshot struct {
	ID        uuid.UUID
	AdvertID int64
	PackageID uuid.UUID
	EndsAt    *time.Time
}

// AdvertSnapshotReader loads advert projection data.
type AdvertSnapshotReader interface {
	GetAdvertSnapshot(ctx context.Context, advertID int64) (AdvertSnapshot, error)
}

// PackageSnapshotReader loads package and assignment snapshots.
type PackageSnapshotReader interface {
	GetPackageByID(ctx context.Context, packageID uuid.UUID) (domainpackaging.Package, error)
	GetEffectiveAssignment(ctx context.Context, advertID int64, at time.Time) (PackageAssignmentSnapshot, error)
	GetAssignmentByID(ctx context.Context, assignmentID uuid.UUID) (PackageAssignmentSnapshot, error)
	FindActiveUrgent(ctx context.Context, advertID int64) (domainpackaging.AdvertFeatureActivation, error)
}

// RuntimeRepository persists notifications, user states, and eligibility queries.
type RuntimeRepository interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	WithTx(tx pgx.Tx) RuntimeRepository

	FindActiveTemplateByEventType(ctx context.Context, eventType domainnotification.EventType) (domainnotification.NotificationTemplate, bool, error)
	CreateNotificationEventIdempotent(ctx context.Context, n domainnotification.Notification) (created bool, err error)
	GetNotificationByID(ctx context.Context, id uuid.UUID) (domainnotification.Notification, error)

	// ListUserNotifications pages the inbox by notification created_at DESC,
	// notification id DESC (delivery state timestamps are not the cursor: two
	// notifications delivered in the same fan-out batch must still sort by when
	// the underlying event happened).
	ListUserNotifications(
		ctx context.Context,
		userID uuid.UUID,
		afterCreatedAt *time.Time,
		afterNotificationID *uuid.UUID,
		limit int,
	) ([]domainnotification.InboxItem, error)
	CountUnread(ctx context.Context, userID uuid.UUID) (int, error)
	MarkRead(ctx context.Context, userID, notificationID uuid.UUID, readAt time.Time) error
	MarkAllRead(ctx context.Context, userID uuid.UUID, readAt time.Time) (int64, error)

	// ListEligibleUsersAfterCursor lists ACTIVE users ordered by id ASC, each
	// carrying its own email-verified flag so a single page serves both the
	// delivery-state insert and the QUEUED/NOT_REQUESTED email decision.
	ListEligibleUsersAfterCursor(
		ctx context.Context,
		afterUserID *uuid.UUID,
		limit int,
	) ([]domainnotification.EligibleUser, error)
	InsertUserNotificationStates(ctx context.Context, states []domainnotification.UserNotificationState) (inserted int, err error)

	GetEmailDeliveryStates(
		ctx context.Context,
		notificationID uuid.UUID,
		userIDs []uuid.UUID,
	) ([]domainnotification.UserNotificationState, error)
	MarkEmailAttempt(ctx context.Context, userID, notificationID uuid.UUID, idempotencyKey string, attemptedAt time.Time) error
	MarkEmailSent(ctx context.Context, userID, notificationID uuid.UUID, sentAt time.Time) error
	MarkEmailFailed(ctx context.Context, userID, notificationID uuid.UUID, attemptedAt time.Time, lastError string) error

	FindBestActiveCampaignForExpiry(
		ctx context.Context,
		eventType domainnotification.EventType,
		sourcePackageID uuid.UUID,
		at time.Time,
	) (domaincampaign.Campaign, bool, error)

	ListAssignmentsExpiringOnLocalDay(
		ctx context.Context,
		targetDay time.Time,
		loc *time.Location,
		afterAssignmentID *uuid.UUID,
		limit int,
	) ([]domainpackaging.AdvertPackageAssignment, error)

	// ListActiveAssignmentsPastEndsAt locks up to limit ACTIVE assignments whose
	// ends_at has passed, using FOR UPDATE SKIP LOCKED so concurrent scheduler
	// instances partition the backlog instead of colliding.
	ListActiveAssignmentsPastEndsAt(
		ctx context.Context,
		before time.Time,
		limit int,
	) ([]domainpackaging.AdvertPackageAssignment, error)
	MarkAssignmentExpired(ctx context.Context, id uuid.UUID, expiredAt, updatedAt time.Time) error

	// DeactivateActiveUrgentForAdvert deactivates the advert's ACTIVE URGENT
	// feature activation, if any, reporting whether a row changed.
	DeactivateActiveUrgentForAdvert(
		ctx context.Context,
		advertID int64,
		reason string,
		deactivatedAt, updatedAt time.Time,
	) (bool, error)
}

// VerifiedUserReader loads users for email delivery.
type VerifiedUserReader interface {
	FindByID(ctx context.Context, id uuid.UUID) (domainuser.User, error)
}
