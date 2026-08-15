package notification

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincampaign "github.com/hkizilbulak/haradan-be/internal/domain/campaign"
	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

const (
	notificationNotFoundMessage = "Bildirim bulunamadı."
	stateNotFoundMessage        = "Bildirim bulunamadı."
)

const notificationColumns = `id, event_type, event_key, advert_id, package_assignment_id, campaign_id,
template_id, title, body, payload, created_at`

const userStateColumns = `user_id, notification_id, delivered_at, read_at, email_status, email_idempotency_key,
email_attempt_count, email_last_attempt_at, email_sent_at, email_last_error, created_at, updated_at`

// FindActiveTemplateByEventType loads an active template or ok=false.
func (r *Repository) FindActiveTemplateByEventType(
	ctx context.Context,
	eventType domainnotification.EventType,
) (domainnotification.NotificationTemplate, bool, error) {
	q := `SELECT ` + templateColumns + ` FROM hrd_notification_templates WHERE event_type = $1 AND is_active = true`
	t, err := scanTemplate(r.db.QueryRow(ctx, q, string(eventType)))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainnotification.NotificationTemplate{}, false, nil
	}
	if err != nil {
		return domainnotification.NotificationTemplate{}, false, apperr.Internal(fmt.Errorf("find active template: %w", pg.SanitizeErr(err)))
	}
	return t, true, nil
}

// CreateNotificationEventIdempotent inserts a notification unless event_key exists.
func (r *Repository) CreateNotificationEventIdempotent(
	ctx context.Context,
	n domainnotification.Notification,
) (bool, error) {
	const q = `
INSERT INTO hrd_notifications (
  id, event_type, event_key, advert_id, package_assignment_id, campaign_id, template_id,
  title, body, payload, created_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11)
ON CONFLICT (event_key) DO NOTHING
RETURNING id`
	var inserted uuid.UUID
	err := r.db.QueryRow(ctx, q,
		n.ID, string(n.EventType), n.EventKey, n.AdvertID, n.PackageAssignmentID, n.CampaignID, n.TemplateID,
		n.Title, n.Body, payloadOrEmpty(n.Payload), n.CreatedAt,
	).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, apperr.Internal(fmt.Errorf("create notification: %w", pg.SanitizeErr(err)))
	}
	return true, nil
}

// GetNotificationByID loads one notification row.
func (r *Repository) GetNotificationByID(ctx context.Context, id uuid.UUID) (domainnotification.Notification, error) {
	q := `SELECT ` + notificationColumns + ` FROM hrd_notifications WHERE id = $1`
	n, err := scanNotification(r.db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainnotification.Notification{}, apperr.NotFound(notificationNotFoundMessage)
	}
	if err != nil {
		return domainnotification.Notification{}, apperr.Internal(fmt.Errorf("get notification: %w", pg.SanitizeErr(err)))
	}
	return n, nil
}

// ListUserNotifications returns inbox rows for one user, paged by the
// underlying event's created_at/id (not the delivery timestamp, so
// notifications delivered in the same fan-out batch still sort by when the
// event happened).
func (r *Repository) ListUserNotifications(
	ctx context.Context,
	userID uuid.UUID,
	afterCreatedAt *time.Time,
	afterNotificationID *uuid.UUID,
	limit int,
) ([]domainnotification.InboxItem, error) {
	args := []any{userID, limit}
	cursorSQL := ``
	if afterCreatedAt != nil && afterNotificationID != nil {
		cursorSQL = ` AND (n.created_at, n.id) < ($3, $4)`
		args = append(args, *afterCreatedAt, *afterNotificationID)
	}
	q := `
SELECT ` + notificationColumns + `, s.user_id, s.delivered_at, s.read_at, s.email_status, s.email_idempotency_key,
s.email_attempt_count, s.email_last_attempt_at, s.email_sent_at, s.email_last_error, s.created_at, s.updated_at
FROM hrd_user_notification_states s
JOIN hrd_notifications n ON n.id = s.notification_id
WHERE s.user_id = $1` + cursorSQL + `
ORDER BY n.created_at DESC, n.id DESC
LIMIT $2`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list user notifications: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()
	out := make([]domainnotification.InboxItem, 0)
	for rows.Next() {
		item, err := scanInboxItem(rows)
		if err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan user notification: %w", pg.SanitizeErr(err)))
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate user notifications: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// CountUnread counts unread notifications for a user.
func (r *Repository) CountUnread(ctx context.Context, userID uuid.UUID) (int, error) {
	const q = `SELECT COUNT(*) FROM hrd_user_notification_states WHERE user_id = $1 AND read_at IS NULL`
	var count int
	if err := r.db.QueryRow(ctx, q, userID).Scan(&count); err != nil {
		return 0, apperr.Internal(fmt.Errorf("count unread notifications: %w", pg.SanitizeErr(err)))
	}
	return count, nil
}

// MarkRead sets read_at for one user notification. Already-read rows are a
// successful no-op so the HTTP mark-read path stays idempotent.
func (r *Repository) MarkRead(ctx context.Context, userID, notificationID uuid.UUID, readAt time.Time) error {
	const q = `
UPDATE hrd_user_notification_states
SET read_at = COALESCE(read_at, $3),
    updated_at = CASE WHEN read_at IS NULL THEN $3 ELSE updated_at END
WHERE user_id = $1 AND notification_id = $2`
	tag, err := r.db.Exec(ctx, q, userID, notificationID, readAt)
	if err != nil {
		return apperr.Internal(fmt.Errorf("mark notification read: %w", pg.SanitizeErr(err)))
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound(stateNotFoundMessage)
	}
	return nil
}

// MarkAllRead sets read_at for all unread rows of a user.
func (r *Repository) MarkAllRead(ctx context.Context, userID uuid.UUID, readAt time.Time) (int64, error) {
	const q = `
UPDATE hrd_user_notification_states
SET read_at = $2, updated_at = $2
WHERE user_id = $1 AND read_at IS NULL`
	tag, err := r.db.Exec(ctx, q, userID, readAt)
	if err != nil {
		return 0, apperr.Internal(fmt.Errorf("mark all notifications read: %w", pg.SanitizeErr(err)))
	}
	return tag.RowsAffected(), nil
}

// ListEligibleUsersAfterCursor lists ACTIVE users ordered by id ASC, each
// carrying its own email-verified flag so callers can decide QUEUED vs
// NOT_REQUESTED per user from a single page.
func (r *Repository) ListEligibleUsersAfterCursor(
	ctx context.Context,
	afterUserID *uuid.UUID,
	limit int,
) ([]domainnotification.EligibleUser, error) {
	cursorSQL := ``
	args := []any{limit}
	if afterUserID != nil {
		cursorSQL = ` AND id > $2`
		args = []any{limit, *afterUserID}
	}
	q := fmt.Sprintf(`
SELECT id, email, email_verified_at FROM hrd_users
WHERE status = 'ACTIVE'%s
ORDER BY id ASC
LIMIT $1`, cursorSQL)
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list eligible users: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()
	out := make([]domainnotification.EligibleUser, 0)
	for rows.Next() {
		var (
			u               domainnotification.EligibleUser
			emailVerifiedAt *time.Time
		)
		if err := rows.Scan(&u.ID, &u.Email, &emailVerifiedAt); err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan eligible user: %w", pg.SanitizeErr(err)))
		}
		u.EmailVerified = emailVerifiedAt != nil
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate eligible users: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// InsertUserNotificationStates bulk inserts fan-out rows (ignores duplicates).
func (r *Repository) InsertUserNotificationStates(
	ctx context.Context,
	states []domainnotification.UserNotificationState,
) (int, error) {
	if len(states) == 0 {
		return 0, nil
	}
	inserted := 0
	for _, st := range states {
		const q = `
INSERT INTO hrd_user_notification_states (
  user_id, notification_id, delivered_at, read_at, email_status, email_idempotency_key,
  email_attempt_count, email_last_attempt_at, email_sent_at, email_last_error, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT (user_id, notification_id) DO NOTHING`
		tag, err := r.db.Exec(ctx, q,
			st.UserID, st.NotificationID, st.DeliveredAt, st.ReadAt, string(st.EmailStatus), st.EmailIdempotencyKey,
			st.EmailAttemptCount, st.EmailLastAttemptAt, st.EmailSentAt, st.EmailLastError, st.CreatedAt, st.UpdatedAt,
		)
		if err != nil {
			return inserted, apperr.Internal(fmt.Errorf("insert user notification state: %w", pg.SanitizeErr(err)))
		}
		inserted += int(tag.RowsAffected())
	}
	return inserted, nil
}

// GetEmailDeliveryStates loads email columns for notification/user pairs.
func (r *Repository) GetEmailDeliveryStates(
	ctx context.Context,
	notificationID uuid.UUID,
	userIDs []uuid.UUID,
) ([]domainnotification.UserNotificationState, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	q := `SELECT ` + userStateColumns + ` FROM hrd_user_notification_states WHERE notification_id = $1 AND user_id = ANY($2)`
	rows, err := r.db.Query(ctx, q, notificationID, userIDs)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("get email delivery states: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()
	out := make([]domainnotification.UserNotificationState, 0)
	for rows.Next() {
		st, err := scanUserState(rows)
		if err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan email state: %w", pg.SanitizeErr(err)))
		}
		out = append(out, st)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate email states: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// MarkEmailAttempt records a queued email attempt.
func (r *Repository) MarkEmailAttempt(
	ctx context.Context,
	userID, notificationID uuid.UUID,
	idempotencyKey string,
	attemptedAt time.Time,
) error {
	const q = `
UPDATE hrd_user_notification_states
SET email_status = 'QUEUED',
    email_idempotency_key = $3,
    email_attempt_count = email_attempt_count + 1,
    email_last_attempt_at = $4,
    updated_at = $4
WHERE user_id = $1 AND notification_id = $2`
	tag, err := r.db.Exec(ctx, q, userID, notificationID, idempotencyKey, attemptedAt)
	if err != nil {
		return apperr.Internal(fmt.Errorf("mark email attempt: %w", pg.SanitizeErr(err)))
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound(stateNotFoundMessage)
	}
	return nil
}

// MarkEmailSent marks email delivery success.
func (r *Repository) MarkEmailSent(ctx context.Context, userID, notificationID uuid.UUID, sentAt time.Time) error {
	const q = `
UPDATE hrd_user_notification_states
SET email_status = 'SENT', email_sent_at = $3, updated_at = $3
WHERE user_id = $1 AND notification_id = $2`
	tag, err := r.db.Exec(ctx, q, userID, notificationID, sentAt)
	if err != nil {
		return apperr.Internal(fmt.Errorf("mark email sent: %w", pg.SanitizeErr(err)))
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound(stateNotFoundMessage)
	}
	return nil
}

// MarkEmailFailed marks email delivery failure.
func (r *Repository) MarkEmailFailed(
	ctx context.Context,
	userID, notificationID uuid.UUID,
	attemptedAt time.Time,
	lastError string,
) error {
	const q = `
UPDATE hrd_user_notification_states
SET email_status = 'FAILED', email_last_attempt_at = $3, email_last_error = $4, updated_at = $3
WHERE user_id = $1 AND notification_id = $2`
	tag, err := r.db.Exec(ctx, q, userID, notificationID, attemptedAt, lastError)
	if err != nil {
		return apperr.Internal(fmt.Errorf("mark email failed: %w", pg.SanitizeErr(err)))
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound(stateNotFoundMessage)
	}
	return nil
}

// FindBestActiveCampaignForExpiry selects the best active expiry campaign.
func (r *Repository) FindBestActiveCampaignForExpiry(
	ctx context.Context,
	eventType domainnotification.EventType,
	sourcePackageID uuid.UUID,
	at time.Time,
) (domaincampaign.Campaign, bool, error) {
	const q = `
SELECT id, code, name, event_type, source_package_id, target_package_id, title, description,
email_subject, email_heading, email_body, cta_label, cta_url, badge_text, image_asset_id,
display_original_price_amount_minor, display_campaign_price_amount_minor, currency_code,
starts_at, ends_at, is_active, created_by_user_id, version, created_at, updated_at
FROM hrd_campaigns
WHERE is_active = true
  AND event_type = $1
  AND starts_at <= $2
  AND (ends_at IS NULL OR ends_at > $2)
  AND (source_package_id IS NULL OR source_package_id = $3)
ORDER BY (source_package_id IS NULL) ASC, created_at DESC, id DESC
LIMIT 1`
	row := r.db.QueryRow(ctx, q, string(eventType), at, sourcePackageID)
	c, err := scanCampaign(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domaincampaign.Campaign{}, false, nil
	}
	if err != nil {
		return domaincampaign.Campaign{}, false, apperr.Internal(fmt.Errorf("find expiry campaign: %w", pg.SanitizeErr(err)))
	}
	return c, true, nil
}

// ListAssignmentsExpiringOnLocalDay lists ACTIVE assignments whose ends_at falls on targetDay in loc.
func (r *Repository) ListAssignmentsExpiringOnLocalDay(
	ctx context.Context,
	targetDay time.Time,
	loc *time.Location,
	afterAssignmentID *uuid.UUID,
	limit int,
) ([]domainpackaging.AdvertPackageAssignment, error) {
	if loc == nil {
		loc = time.UTC
	}
	local := targetDay.In(loc)
	localStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	localEnd := localStart.Add(24 * time.Hour)
	startUTC := localStart.UTC()
	endUTC := localEnd.UTC()
	cursorSQL := ``
	args := []any{startUTC, endUTC, limit}
	if afterAssignmentID != nil {
		cursorSQL = ` AND id > $4`
		args = append(args, *afterAssignmentID)
	}
	q := `
SELECT id, advert_id, package_id, status, starts_at, ends_at, assigned_by_user_id, assigned_at,
superseded_at, expired_at, cancelled_at, reason, source, version, created_at, updated_at
FROM hrd_advert_package_assignments
WHERE status = 'ACTIVE'
  AND ends_at IS NOT NULL
  AND ends_at >= $1
  AND ends_at < $2` + cursorSQL + `
ORDER BY id ASC
LIMIT $3`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list expiring assignments: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()
	out := make([]domainpackaging.AdvertPackageAssignment, 0)
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan expiring assignment: %w", pg.SanitizeErr(err)))
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate expiring assignments: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// ListActiveAssignmentsPastEndsAt locks up to limit ACTIVE assignments whose
// ends_at has passed, using FOR UPDATE SKIP LOCKED so concurrent scheduler
// instances partition the backlog instead of colliding on the same rows.
func (r *Repository) ListActiveAssignmentsPastEndsAt(
	ctx context.Context,
	before time.Time,
	limit int,
) ([]domainpackaging.AdvertPackageAssignment, error) {
	const q = `
SELECT id, advert_id, package_id, status, starts_at, ends_at, assigned_by_user_id, assigned_at,
superseded_at, expired_at, cancelled_at, reason, source, version, created_at, updated_at
FROM hrd_advert_package_assignments
WHERE status = 'ACTIVE'
  AND ends_at IS NOT NULL
  AND ends_at <= $1
ORDER BY id ASC
LIMIT $2
FOR UPDATE SKIP LOCKED`
	rows, err := r.db.Query(ctx, q, before, limit)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list past-due assignments: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()
	out := make([]domainpackaging.AdvertPackageAssignment, 0)
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan past-due assignment: %w", pg.SanitizeErr(err)))
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate past-due assignments: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// MarkAssignmentExpired sets an ACTIVE assignment to EXPIRED.
func (r *Repository) MarkAssignmentExpired(ctx context.Context, id uuid.UUID, expiredAt, updatedAt time.Time) error {
	const q = `
UPDATE hrd_advert_package_assignments
SET status = 'EXPIRED', expired_at = $2, updated_at = $3
WHERE id = $1 AND status = 'ACTIVE'`
	tag, err := r.db.Exec(ctx, q, id, expiredAt, updatedAt)
	if err != nil {
		return apperr.Internal(fmt.Errorf("mark assignment expired: %w", pg.SanitizeErr(err)))
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("Paket ataması bulunamadı.")
	}
	return nil
}

// DeactivateActiveUrgentForAdvert deactivates the advert's ACTIVE URGENT
// feature activation, if any, mirroring the packaging domain's
// DeactivateActiveFeature for the expiry-driven case (reason PACKAGE_EXPIRED).
func (r *Repository) DeactivateActiveUrgentForAdvert(
	ctx context.Context,
	advertID uuid.UUID,
	reason string,
	deactivatedAt, updatedAt time.Time,
) (bool, error) {
	const q = `
UPDATE hrd_advert_feature_activations
SET status = 'DEACTIVATED', deactivated_at = $2, reason = $3, updated_at = $4
WHERE advert_id = $1 AND feature_code IN ('URGENT', 'FEATURED') AND status = 'ACTIVE'`
	tag, err := r.db.Exec(ctx, q, advertID, deactivatedAt, reason, updatedAt)
	if err != nil {
		return false, apperr.Internal(fmt.Errorf("deactivate urgent for advert: %w", pg.SanitizeErr(err)))
	}
	return tag.RowsAffected() > 0, nil
}

func payloadOrEmpty(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte(`{}`)
	}
	return raw
}

func scanNotification(row rowScanner) (domainnotification.Notification, error) {
	var (
		n         domainnotification.Notification
		eventType string
		payload   []byte
	)
	if err := row.Scan(
		&n.ID, &eventType, &n.EventKey, &n.AdvertID, &n.PackageAssignmentID, &n.CampaignID, &n.TemplateID,
		&n.Title, &n.Body, &payload, &n.CreatedAt,
	); err != nil {
		return domainnotification.Notification{}, err
	}
	n.EventType = domainnotification.EventType(eventType)
	n.Payload = payloadOrEmpty(payload)
	return n, nil
}

func scanUserState(row rowScanner) (domainnotification.UserNotificationState, error) {
	var (
		st          domainnotification.UserNotificationState
		emailStatus string
	)
	if err := row.Scan(
		&st.UserID, &st.NotificationID, &st.DeliveredAt, &st.ReadAt, &emailStatus, &st.EmailIdempotencyKey,
		&st.EmailAttemptCount, &st.EmailLastAttemptAt, &st.EmailSentAt, &st.EmailLastError, &st.CreatedAt, &st.UpdatedAt,
	); err != nil {
		return domainnotification.UserNotificationState{}, err
	}
	st.EmailStatus = domainnotification.EmailStatus(emailStatus)
	return st, nil
}

func scanInboxItem(row rowScanner) (domainnotification.InboxItem, error) {
	var (
		n           domainnotification.Notification
		st          domainnotification.UserNotificationState
		eventType   string
		payload     []byte
		emailStatus string
	)
	if err := row.Scan(
		&n.ID, &eventType, &n.EventKey, &n.AdvertID, &n.PackageAssignmentID, &n.CampaignID, &n.TemplateID,
		&n.Title, &n.Body, &payload, &n.CreatedAt,
		&st.UserID, &st.DeliveredAt, &st.ReadAt, &emailStatus, &st.EmailIdempotencyKey,
		&st.EmailAttemptCount, &st.EmailLastAttemptAt, &st.EmailSentAt, &st.EmailLastError, &st.CreatedAt, &st.UpdatedAt,
	); err != nil {
		return domainnotification.InboxItem{}, err
	}
	n.EventType = domainnotification.EventType(eventType)
	n.Payload = payloadOrEmpty(payload)
	st.NotificationID = n.ID
	st.EmailStatus = domainnotification.EmailStatus(emailStatus)
	return domainnotification.InboxItem{Notification: n, State: st}, nil
}

func scanCampaign(row rowScanner) (domaincampaign.Campaign, error) {
	var (
		c         domaincampaign.Campaign
		eventType string
	)
	if err := row.Scan(
		&c.ID, &c.Code, &c.Name, &eventType, &c.SourcePackageID, &c.TargetPackageID, &c.Title, &c.Description,
		&c.EmailSubject, &c.EmailHeading, &c.EmailBody, &c.CTALabel, &c.CTAURL, &c.BadgeText, &c.ImageAssetID,
		&c.DisplayOriginalPriceAmountMinor, &c.DisplayCampaignPriceAmountMinor, &c.CurrencyCode,
		&c.StartsAt, &c.EndsAt, &c.IsActive, &c.CreatedByUserID, &c.Version, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return domaincampaign.Campaign{}, err
	}
	c.EventType = domaincampaign.CampaignEventType(eventType)
	return c, nil
}

func scanAssignment(row rowScanner) (domainpackaging.AdvertPackageAssignment, error) {
	var (
		a      domainpackaging.AdvertPackageAssignment
		status string
		source string
	)
	if err := row.Scan(
		&a.ID, &a.AdvertID, &a.PackageID, &status, &a.StartsAt, &a.EndsAt, &a.AssignedByUserID, &a.AssignedAt,
		&a.SupersededAt, &a.ExpiredAt, &a.CancelledAt, &a.Reason, &source, &a.Version, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return domainpackaging.AdvertPackageAssignment{}, err
	}
	a.Status = domainpackaging.AssignmentStatus(status)
	a.Source = domainpackaging.AssignmentSource(source)
	return a, nil
}
