package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
)

// NotificationEmailSender delivers template emails (Resend adapter).
type NotificationEmailSender interface {
	SendTemplateEmail(
		ctx context.Context,
		recipient string,
		templateID string,
		subjectFallback *string,
		variables map[string]string,
		idempotencyKey string,
	) error
}

// FanoutConfig configures fan-out and email chunk handlers.
type FanoutConfig struct {
	Repo            RuntimeRepository
	Jobs            JobEnqueuer
	Email           NotificationEmailSender
	Users           VerifiedUserReader
	Clock           Clock
	FanoutBatchSize int
	EmailChunkSize  int
}

// FanoutService processes advert notification fan-out and email chunk jobs.
type FanoutService struct {
	repo       RuntimeRepository
	jobs       JobEnqueuer
	email      NotificationEmailSender
	users      VerifiedUserReader
	clock      Clock
	fanoutSize int
	chunkSize  int
}

// NewFanoutService constructs a FanoutService.
func NewFanoutService(cfg FanoutConfig) (*FanoutService, error) {
	if cfg.Repo == nil || cfg.Jobs == nil {
		return nil, fmt.Errorf("fanout dependencies are required")
	}
	clock := cfg.Clock
	if clock == nil {
		clock = systemClock{}
	}
	fanout := cfg.FanoutBatchSize
	if fanout < 1 {
		fanout = 100
	}
	chunk := cfg.EmailChunkSize
	if chunk < 1 {
		chunk = 25
	}
	return &FanoutService{
		repo: cfg.Repo, jobs: cfg.Jobs, email: cfg.Email, users: cfg.Users,
		clock: clock, fanoutSize: fanout, chunkSize: chunk,
	}, nil
}

type fanoutJobPayload struct {
	NotificationID string `json:"notificationId"`
	AfterUserID    string `json:"afterUserId,omitempty"`
}

type emailChunkPayload struct {
	NotificationID string   `json:"notificationId"`
	UserIDs        []string `json:"userIds"`
}

type expiryEmailPayload struct {
	NotificationID string `json:"notificationId"`
	UserID         string `json:"userId"`
}

// ProcessAdvertFanout handles NOTIFICATION_FANOUT_* jobs. For each ACTIVE user
// page (id ASC cursor) it inserts delivery states (QUEUED+key when the user
// has a verified email, NOT_REQUESTED otherwise), splits the newly-queued
// users into EmailChunkSize chunks and enqueues one
// EMAIL_SEND_ADVERT_NOTIFICATION_CHUNK job per chunk carrying only user ids
// (never email addresses), then enqueues the next page when more users remain.
func (s *FanoutService) ProcessAdvertFanout(ctx context.Context, jobType domainmedia.JobType, payload json.RawMessage) error {
	var in fanoutJobPayload
	if err := json.Unmarshal(payload, &in); err != nil {
		return fmt.Errorf("invalid fanout payload")
	}
	notificationID, err := uuid.Parse(strings.TrimSpace(in.NotificationID))
	if err != nil {
		return fmt.Errorf("invalid notification id")
	}
	var afterUserID *uuid.UUID
	if strings.TrimSpace(in.AfterUserID) != "" {
		id, err := uuid.Parse(in.AfterUserID)
		if err != nil {
			return fmt.Errorf("invalid after user id")
		}
		afterUserID = &id
	}

	if _, err := s.repo.GetNotificationByID(ctx, notificationID); err != nil {
		return err
	}

	users, err := s.repo.ListEligibleUsersAfterCursor(ctx, afterUserID, s.fanoutSize+1)
	if err != nil {
		return err
	}
	hasMore := len(users) > s.fanoutSize
	if hasMore {
		users = users[:s.fanoutSize]
	}
	if len(users) == 0 {
		return nil
	}

	now := s.clock.Now().UTC()
	states := make([]domainnotification.UserNotificationState, 0, len(users))
	queuedUserIDs := make([]uuid.UUID, 0, len(users))
	for _, u := range users {
		state := domainnotification.UserNotificationState{
			UserID:         u.ID,
			NotificationID: notificationID,
			DeliveredAt:    now,
			EmailStatus:    domainnotification.EmailStatusNotRequested,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if u.HasVerifiedEmail() {
			key := domainnotification.AdvertNotificationEmailIdempotencyKey(notificationID, u.ID)
			state.EmailStatus = domainnotification.EmailStatusQueued
			state.EmailIdempotencyKey = &key
			queuedUserIDs = append(queuedUserIDs, u.ID)
		}
		states = append(states, state)
	}
	if _, err := s.repo.InsertUserNotificationStates(ctx, states); err != nil {
		return err
	}

	for _, chunk := range chunkUserIDs(queuedUserIDs, s.chunkSize) {
		userIDStrings := make([]string, len(chunk))
		for i, id := range chunk {
			userIDStrings[i] = id.String()
		}
		chunkPayload, err := json.Marshal(emailChunkPayload{
			NotificationID: notificationID.String(),
			UserIDs:        userIDStrings,
		})
		if err != nil {
			return err
		}
		dedup := emailChunkDedupKey(notificationID, chunk)
		if err := enqueueJobIgnoringDuplicate(ctx, s.jobs, domainmedia.BackgroundJob{
			ID:               uuid.New(),
			JobType:          domainmedia.JobEmailSendAdvertNotificationChunk,
			Status:           domainmedia.JobQueued,
			Payload:          chunkPayload,
			DeduplicationKey: &dedup,
			MaxAttempts:      defaultJobMaxAttempts,
			AvailableAt:      now,
			Version:          1,
			CreatedAt:        now,
			UpdatedAt:        now,
		}); err != nil {
			return err
		}
	}

	if hasMore {
		last := users[len(users)-1]
		contPayload, err := json.Marshal(fanoutJobPayload{
			NotificationID: notificationID.String(),
			AfterUserID:    last.ID.String(),
		})
		if err != nil {
			return err
		}
		dedup := fanoutPageDedupKey(jobType, notificationID, &last.ID)
		return enqueueJobIgnoringDuplicate(ctx, s.jobs, domainmedia.BackgroundJob{
			ID:               uuid.New(),
			JobType:          jobType,
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

// chunkUserIDs splits a sorted id slice into contiguous groups of at most size.
func chunkUserIDs(ids []uuid.UUID, size int) [][]uuid.UUID {
	if len(ids) == 0 {
		return nil
	}
	if size < 1 {
		size = 1
	}
	out := make([][]uuid.UUID, 0, (len(ids)+size-1)/size)
	for start := 0; start < len(ids); start += size {
		end := start + size
		if end > len(ids) {
			end = len(ids)
		}
		out = append(out, ids[start:end])
	}
	return out
}

// emailChunkDedupKey is the deterministic dedup key for one email chunk job:
// the first and last id of the (already sorted) chunk uniquely identify it
// for a given notification.
func emailChunkDedupKey(notificationID uuid.UUID, userIDs []uuid.UUID) string {
	first := userIDs[0]
	last := userIDs[len(userIDs)-1]
	return string(domainmedia.JobEmailSendAdvertNotificationChunk) + ":" + notificationID.String() +
		":chunk:" + first.String() + ":" + last.String()
}

// ProcessAdvertEmailChunk handles EMAIL_SEND_ADVERT_NOTIFICATION_CHUNK jobs.
// The payload carries only user ids (never emails); each user's address is
// looked up fresh so a chunk always reflects the latest profile. A permanent
// send failure (invalid recipient) is recorded and the chunk continues; a
// transient failure fails the whole job so the worker retries it, and
// already-SENT recipients are skipped on that retry.
func (s *FanoutService) ProcessAdvertEmailChunk(ctx context.Context, payload json.RawMessage) error {
	if s.email == nil {
		return nil
	}
	var in emailChunkPayload
	if err := json.Unmarshal(payload, &in); err != nil {
		return fmt.Errorf("invalid email chunk payload")
	}
	notificationID, err := uuid.Parse(strings.TrimSpace(in.NotificationID))
	if err != nil {
		return fmt.Errorf("invalid notification id")
	}
	if len(in.UserIDs) == 0 {
		return nil
	}
	userIDs := make([]uuid.UUID, 0, len(in.UserIDs))
	for _, raw := range in.UserIDs {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("invalid user id in chunk payload")
		}
		userIDs = append(userIDs, id)
	}

	notification, err := s.repo.GetNotificationByID(ctx, notificationID)
	if err != nil {
		return err
	}
	tmpl, ok, err := s.repo.FindActiveTemplateByEventType(ctx, notification.EventType)
	if err != nil {
		return err
	}
	if !ok || tmpl.ResendTemplateID == nil || strings.TrimSpace(*tmpl.ResendTemplateID) == "" {
		return nil
	}

	states, err := s.repo.GetEmailDeliveryStates(ctx, notificationID, userIDs)
	if err != nil {
		return err
	}
	stateByUser := make(map[uuid.UUID]domainnotification.UserNotificationState, len(states))
	for _, st := range states {
		stateByUser[st.UserID] = st
	}

	for _, userID := range userIDs {
		st, exists := stateByUser[userID]
		if !exists {
			continue
		}
		if st.EmailStatus == domainnotification.EmailStatusSent || st.EmailStatus == domainnotification.EmailStatusNotRequested {
			continue
		}

		user, err := s.users.FindByID(ctx, userID)
		if err != nil {
			if isNotFoundErr(err) {
				now := s.clock.Now().UTC()
				_ = s.repo.MarkEmailFailed(ctx, userID, notificationID, now, "kullanıcı bulunamadı")
				continue
			}
			return err
		}
		if !user.IsActive() || user.EmailVerifiedAt == nil || strings.TrimSpace(user.Email) == "" {
			now := s.clock.Now().UTC()
			_ = s.repo.MarkEmailFailed(ctx, userID, notificationID, now, "kullanıcı artık uygun değil")
			continue
		}

		now := s.clock.Now().UTC()
		idem := domainnotification.AdvertNotificationEmailIdempotencyKey(notificationID, userID)
		if err := s.repo.MarkEmailAttempt(ctx, userID, notificationID, idem, now); err != nil {
			return err
		}
		vars := map[string]string{
			"title": notification.Title,
			"body":  notification.Body,
		}
		if notification.AdvertID != nil {
			vars["advertId"] = fmt.Sprintf("%d", *notification.AdvertID)
		}
		sendErr := s.email.SendTemplateEmail(ctx, user.Email, *tmpl.ResendTemplateID, tmpl.EmailSubjectFallback, vars, idem)
		if sendErr != nil {
			if isPermanentEmailError(sendErr) {
				_ = s.repo.MarkEmailFailed(ctx, userID, notificationID, now, sanitizeEmailError(sendErr))
				continue
			}
			return sendErr
		}
		if err := s.repo.MarkEmailSent(ctx, userID, notificationID, now); err != nil {
			return err
		}
	}
	return nil
}

// ProcessPackageExpiryEmail handles EMAIL_SEND_PACKAGE_EXPIRY_REMINDER jobs.
func (s *FanoutService) ProcessPackageExpiryEmail(ctx context.Context, payload json.RawMessage) error {
	if s.email == nil {
		return nil
	}
	var in expiryEmailPayload
	if err := json.Unmarshal(payload, &in); err != nil {
		return fmt.Errorf("invalid expiry email payload")
	}
	notificationID, err := uuid.Parse(strings.TrimSpace(in.NotificationID))
	if err != nil {
		return fmt.Errorf("invalid notification id")
	}
	userID, err := uuid.Parse(strings.TrimSpace(in.UserID))
	if err != nil {
		return fmt.Errorf("invalid user id")
	}

	states, err := s.repo.GetEmailDeliveryStates(ctx, notificationID, []uuid.UUID{userID})
	if err != nil {
		return err
	}
	if len(states) > 0 {
		switch states[0].EmailStatus {
		case domainnotification.EmailStatusSent, domainnotification.EmailStatusNotRequested:
			return nil
		}
	}

	notification, err := s.repo.GetNotificationByID(ctx, notificationID)
	if err != nil {
		return err
	}
	tmpl, ok, err := s.repo.FindActiveTemplateByEventType(ctx, notification.EventType)
	if err != nil {
		return err
	}
	if !ok || tmpl.ResendTemplateID == nil {
		return nil
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if isNotFoundErr(err) {
			now := s.clock.Now().UTC()
			_ = s.repo.MarkEmailFailed(ctx, userID, notificationID, now, "kullanıcı bulunamadı")
			return nil
		}
		return err
	}
	if !user.IsActive() || user.EmailVerifiedAt == nil {
		now := s.clock.Now().UTC()
		_ = s.repo.MarkEmailFailed(ctx, userID, notificationID, now, "kullanıcı artık uygun değil")
		return nil
	}

	now := s.clock.Now().UTC()
	idem := domainnotification.PackageExpiryEmailIdempotencyKey(notificationID, userID)
	if err := s.repo.MarkEmailAttempt(ctx, userID, notificationID, idem, now); err != nil {
		return err
	}
	vars := map[string]string{
		"title": notification.Title,
		"body":  notification.Body,
	}
	if err := s.email.SendTemplateEmail(ctx, user.Email, *tmpl.ResendTemplateID, tmpl.EmailSubjectFallback, vars, idem); err != nil {
		if isPermanentEmailError(err) {
			_ = s.repo.MarkEmailFailed(ctx, userID, notificationID, now, sanitizeEmailError(err))
			return nil
		}
		return err
	}
	return s.repo.MarkEmailSent(ctx, userID, notificationID, now)
}

// isPermanentEmailError reports whether err reflects a permanent send failure
// (bad recipient, rejected payload) that must not be retried, as opposed to a
// transient dependency failure that should fail the job for worker retry.
func isPermanentEmailError(err error) bool {
	ae, ok := apperr.As(err)
	return ok && ae.Kind == apperr.KindValidation
}

func sanitizeEmailError(err error) string {
	if err == nil {
		return "email delivery failed"
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "email delivery failed"
	}
	if len(msg) > 512 {
		return msg[:512]
	}
	return msg
}
