package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
)

const (
	userNotFoundMessage = "Bildirim bulunamadı."
)

// UserNotificationService implements current-user notification inbox use cases.
type UserNotificationService struct {
	repo  RuntimeRepository
	clock Clock
}

// UserNotificationConfig wires UserNotificationService dependencies.
type UserNotificationConfig struct {
	Repo  RuntimeRepository
	Clock Clock
}

// NewUserNotificationService constructs a UserNotificationService.
func NewUserNotificationService(cfg UserNotificationConfig) (*UserNotificationService, error) {
	if cfg.Repo == nil {
		return nil, fmt.Errorf("user notification repo is required")
	}
	clock := cfg.Clock
	if clock == nil {
		clock = systemClock{}
	}
	return &UserNotificationService{repo: cfg.Repo, clock: clock}, nil
}

// ListUserNotificationsInput carries inbox pagination.
type ListUserNotificationsInput struct {
	UserID uuid.UUID
	Cursor *string
	Limit  int
}

// ListUserNotificationsResult is a paginated inbox page.
type ListUserNotificationsResult struct {
	Items      []domainnotification.InboxItem
	HasMore    bool
	NextCursor *string
}

// ListUserNotifications returns notifications for the current user only,
// paged by the underlying event's created_at/id (not the delivery
// timestamp): notifications delivered in the same fan-out batch must still
// sort by when the event happened, matching RuntimeRepository.ListUserNotifications.
func (s *UserNotificationService) ListUserNotifications(ctx context.Context, in ListUserNotificationsInput) (ListUserNotificationsResult, error) {
	limit := normalizeInboxLimit(in.Limit)
	var afterCreatedAt *time.Time
	var afterNotificationID *uuid.UUID
	if in.Cursor != nil && *in.Cursor != "" {
		createdAt, nid, err := decodeInboxCursor(*in.Cursor)
		if err != nil {
			return ListUserNotificationsResult{}, err
		}
		afterCreatedAt = &createdAt
		afterNotificationID = &nid
	}
	rows, err := s.repo.ListUserNotifications(ctx, in.UserID, afterCreatedAt, afterNotificationID, limit+1)
	if err != nil {
		return ListUserNotificationsResult{}, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := ListUserNotificationsResult{Items: rows, HasMore: hasMore}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		cursor := encodeInboxCursor(last.Notification.CreatedAt, last.Notification.ID)
		out.NextCursor = &cursor
	}
	return out, nil
}

// CountUnread returns unread notification count for the current user.
func (s *UserNotificationService) CountUnread(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.repo.CountUnread(ctx, userID)
}

// MarkRead marks one notification read for the current user.
func (s *UserNotificationService) MarkRead(ctx context.Context, userID, notificationID uuid.UUID) error {
	now := s.clock.Now().UTC()
	if err := s.repo.MarkRead(ctx, userID, notificationID, now); err != nil {
		return err
	}
	return nil
}

// MarkAllRead marks every unread notification read for the current user.
func (s *UserNotificationService) MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	now := s.clock.Now().UTC()
	return s.repo.MarkAllRead(ctx, userID, now)
}

func normalizeInboxLimit(limit int) int {
	if limit < 1 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func encodeInboxCursor(notificationCreatedAt time.Time, notificationID uuid.UUID) string {
	return notificationCreatedAt.UTC().Format(time.RFC3339Nano) + "|" + notificationID.String()
}

func decodeInboxCursor(cursor string) (time.Time, uuid.UUID, error) {
	parts := splitInboxCursor(cursor)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, apperr.Validation("Geçersiz sayfalama imleci.")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, apperr.Validation("Geçersiz sayfalama imleci.")
	}
	id, err := uuid.Parse(parts[1])
	if err != nil || id == uuid.Nil {
		return time.Time{}, uuid.Nil, apperr.Validation("Geçersiz sayfalama imleci.")
	}
	return createdAt, id, nil
}

func splitInboxCursor(cursor string) []string {
	// notification id is last segment after final pipe
	idx := -1
	for i := len(cursor) - 1; i >= 0; i-- {
		if cursor[i] == '|' {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return nil
	}
	return []string{cursor[:idx], cursor[idx+1:]}
}
