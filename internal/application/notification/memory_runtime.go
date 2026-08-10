package notification

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincampaign "github.com/hkizilbulak/haradan-be/internal/domain/campaign"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

const (
	memoryPackageNotFound    = "Paket bulunamadı."
	memoryAssignmentNotFound = "Aktif paket ataması bulunamadı."
)

type MemoryRuntimeStore struct {
	mu sync.Mutex

	templates     map[domainnotification.TemplateEventType]domainnotification.NotificationTemplate
	notifications map[uuid.UUID]domainnotification.Notification
	eventKeys     map[string]uuid.UUID
	states        map[string]domainnotification.UserNotificationState
	users         map[uuid.UUID]domainuser.User
	packages      map[uuid.UUID]domainpackaging.Package
	assignments   map[uuid.UUID]domainpackaging.AdvertPackageAssignment
	adverts       map[uuid.UUID]AdvertSnapshot
	urgent        map[uuid.UUID]domainpackaging.AdvertFeatureActivation
	campaigns     []domaincampaign.Campaign
	jobs          []domainmedia.BackgroundJob
	jobDedup      map[string]struct{}
}

// NewMemoryRuntimeStore builds an empty runtime store.
func NewMemoryRuntimeStore() *MemoryRuntimeStore {
	return &MemoryRuntimeStore{
		templates:     map[domainnotification.TemplateEventType]domainnotification.NotificationTemplate{},
		notifications: map[uuid.UUID]domainnotification.Notification{},
		eventKeys:     map[string]uuid.UUID{},
		states:        map[string]domainnotification.UserNotificationState{},
		users:         map[uuid.UUID]domainuser.User{},
		packages:      map[uuid.UUID]domainpackaging.Package{},
		assignments:   map[uuid.UUID]domainpackaging.AdvertPackageAssignment{},
		adverts:       map[uuid.UUID]AdvertSnapshot{},
		urgent:        map[uuid.UUID]domainpackaging.AdvertFeatureActivation{},
	}
}

func stateKey(userID, notificationID uuid.UUID) string {
	return userID.String() + "|" + notificationID.String()
}

func (s *MemoryRuntimeStore) PutTemplate(t domainnotification.NotificationTemplate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.templates[t.EventType] = t
}

func (s *MemoryRuntimeStore) PutUser(u domainuser.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[u.ID] = u
}

func (s *MemoryRuntimeStore) PutAdvert(a AdvertSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adverts[a.ID] = a
}

func (s *MemoryRuntimeStore) PutPackage(p domainpackaging.Package) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.packages[p.ID] = p
}

func (s *MemoryRuntimeStore) PutAssignment(a domainpackaging.AdvertPackageAssignment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.assignments[a.ID] = a
}

func (s *MemoryRuntimeStore) PutUrgent(a domainpackaging.AdvertFeatureActivation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.urgent[a.AdvertID] = a
}

func (s *MemoryRuntimeStore) PutCampaign(c domaincampaign.Campaign) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.campaigns = append(s.campaigns, c)
}

func (s *MemoryRuntimeStore) RuntimeRepo() RuntimeRepository { return memoryRuntimeRepo{store: s} }
func (s *MemoryRuntimeStore) JobEnqueuer() JobEnqueuer       { return memoryJobEnqueuer{store: s} }
func (s *MemoryRuntimeStore) AdvertReader() AdvertSnapshotReader {
	return memoryAdvertReader{store: s}
}
func (s *MemoryRuntimeStore) PackageReader() PackageSnapshotReader {
	return memoryPackageReader{store: s}
}
func (s *MemoryRuntimeStore) UserReader() VerifiedUserReader { return memoryVerifiedUsers{store: s} }

func (s *MemoryRuntimeStore) Jobs() []domainmedia.BackgroundJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]domainmedia.BackgroundJob(nil), s.jobs...)
	return out
}

func (s *MemoryRuntimeStore) Notifications() []domainnotification.Notification {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domainnotification.Notification, 0, len(s.notifications))
	for _, n := range s.notifications {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

type memoryRuntimeRepo struct{ store *MemoryRuntimeStore }

func (m memoryRuntimeRepo) BeginTx(context.Context) (pgx.Tx, error) {
	return &memoryRuntimeTx{store: m.store}, nil
}

func (m memoryRuntimeRepo) WithTx(pgx.Tx) RuntimeRepository { return m }

func (m memoryRuntimeRepo) FindActiveTemplateByEventType(_ context.Context, eventType domainnotification.EventType) (domainnotification.NotificationTemplate, bool, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	t, ok := m.store.templates[eventType]
	if !ok || !t.IsActive {
		return domainnotification.NotificationTemplate{}, false, nil
	}
	return t, true, nil
}

func (m memoryRuntimeRepo) CreateNotificationEventIdempotent(_ context.Context, n domainnotification.Notification) (bool, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	if id, ok := m.store.eventKeys[n.EventKey]; ok {
		_ = id
		return false, nil
	}
	m.store.notifications[n.ID] = n
	m.store.eventKeys[n.EventKey] = n.ID
	return true, nil
}

func (m memoryRuntimeRepo) GetNotificationByID(_ context.Context, id uuid.UUID) (domainnotification.Notification, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	n, ok := m.store.notifications[id]
	if !ok {
		return domainnotification.Notification{}, apperr.NotFound("Bildirim bulunamadı.")
	}
	return n, nil
}

func (m memoryRuntimeRepo) ListUserNotifications(_ context.Context, userID uuid.UUID, afterCreatedAt *time.Time, afterNotificationID *uuid.UUID, limit int) ([]domainnotification.InboxItem, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	items := make([]domainnotification.InboxItem, 0)
	for _, st := range m.store.states {
		if st.UserID != userID {
			continue
		}
		n, ok := m.store.notifications[st.NotificationID]
		if !ok {
			continue
		}
		items = append(items, domainnotification.InboxItem{Notification: n, State: st})
	}
	// Cursor sorts by notification created_at DESC, id DESC: two notifications
	// delivered in the same fan-out batch must still order by the event they
	// describe, not by the (batched, near-simultaneous) delivery timestamp.
	sort.Slice(items, func(i, j int) bool {
		if items[i].Notification.CreatedAt.Equal(items[j].Notification.CreatedAt) {
			return items[i].Notification.ID.String() > items[j].Notification.ID.String()
		}
		return items[i].Notification.CreatedAt.After(items[j].Notification.CreatedAt)
	})
	if afterCreatedAt != nil && afterNotificationID != nil {
		filtered := items[:0]
		for _, it := range items {
			if it.Notification.CreatedAt.After(*afterCreatedAt) {
				continue
			}
			if it.Notification.CreatedAt.Equal(*afterCreatedAt) && it.Notification.ID.String() >= afterNotificationID.String() {
				continue
			}
			filtered = append(filtered, it)
		}
		items = filtered
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (m memoryRuntimeRepo) CountUnread(_ context.Context, userID uuid.UUID) (int, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	count := 0
	for _, st := range m.store.states {
		if st.UserID == userID && st.ReadAt == nil {
			count++
		}
	}
	return count, nil
}

func (m memoryRuntimeRepo) MarkRead(_ context.Context, userID, notificationID uuid.UUID, readAt time.Time) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	key := stateKey(userID, notificationID)
	st, ok := m.store.states[key]
	if !ok {
		return apperr.NotFound(userNotFoundMessage)
	}
	if st.ReadAt == nil {
		st.ReadAt = &readAt
		st.UpdatedAt = readAt
		m.store.states[key] = st
	}
	return nil
}

func (m memoryRuntimeRepo) MarkAllRead(_ context.Context, userID uuid.UUID, readAt time.Time) (int64, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	var n int64
	for key, st := range m.store.states {
		if st.UserID != userID || st.ReadAt != nil {
			continue
		}
		st.ReadAt = &readAt
		st.UpdatedAt = readAt
		m.store.states[key] = st
		n++
	}
	return n, nil
}

func (m memoryRuntimeRepo) ListEligibleUsersAfterCursor(_ context.Context, afterUserID *uuid.UUID, limit int) ([]domainnotification.EligibleUser, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	ids := make([]uuid.UUID, 0, len(m.store.users))
	for id, u := range m.store.users {
		if !u.IsActive() {
			continue
		}
		if afterUserID != nil && id.String() <= afterUserID.String() {
			continue
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	if len(ids) > limit {
		ids = ids[:limit]
	}
	out := make([]domainnotification.EligibleUser, 0, len(ids))
	for _, id := range ids {
		u := m.store.users[id]
		out = append(out, domainnotification.EligibleUser{
			ID:            id,
			Email:         u.Email,
			EmailVerified: u.EmailVerifiedAt != nil,
		})
	}
	return out, nil
}

func (m memoryRuntimeRepo) InsertUserNotificationStates(_ context.Context, states []domainnotification.UserNotificationState) (int, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	inserted := 0
	for _, st := range states {
		key := stateKey(st.UserID, st.NotificationID)
		if _, ok := m.store.states[key]; ok {
			continue
		}
		m.store.states[key] = st
		inserted++
	}
	return inserted, nil
}

func (m memoryRuntimeRepo) GetEmailDeliveryStates(_ context.Context, notificationID uuid.UUID, userIDs []uuid.UUID) ([]domainnotification.UserNotificationState, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	out := make([]domainnotification.UserNotificationState, 0, len(userIDs))
	for _, uid := range userIDs {
		if st, ok := m.store.states[stateKey(uid, notificationID)]; ok {
			out = append(out, st)
		}
	}
	return out, nil
}

func (m memoryRuntimeRepo) MarkEmailAttempt(_ context.Context, userID, notificationID uuid.UUID, idempotencyKey string, attemptedAt time.Time) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	key := stateKey(userID, notificationID)
	st, ok := m.store.states[key]
	if !ok {
		return apperr.NotFound(userNotFoundMessage)
	}
	st.EmailStatus = domainnotification.EmailStatusQueued
	st.EmailIdempotencyKey = &idempotencyKey
	st.EmailAttemptCount++
	st.EmailLastAttemptAt = &attemptedAt
	st.UpdatedAt = attemptedAt
	m.store.states[key] = st
	return nil
}

func (m memoryRuntimeRepo) MarkEmailSent(_ context.Context, userID, notificationID uuid.UUID, sentAt time.Time) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	key := stateKey(userID, notificationID)
	st, ok := m.store.states[key]
	if !ok {
		return apperr.NotFound(userNotFoundMessage)
	}
	st.EmailStatus = domainnotification.EmailStatusSent
	st.EmailSentAt = &sentAt
	st.UpdatedAt = sentAt
	m.store.states[key] = st
	return nil
}

func (m memoryRuntimeRepo) MarkEmailFailed(_ context.Context, userID, notificationID uuid.UUID, attemptedAt time.Time, lastError string) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	key := stateKey(userID, notificationID)
	st, ok := m.store.states[key]
	if !ok {
		return apperr.NotFound(userNotFoundMessage)
	}
	st.EmailStatus = domainnotification.EmailStatusFailed
	st.EmailLastAttemptAt = &attemptedAt
	st.EmailLastError = &lastError
	st.UpdatedAt = attemptedAt
	m.store.states[key] = st
	return nil
}

func (m memoryRuntimeRepo) FindBestActiveCampaignForExpiry(_ context.Context, eventType domainnotification.EventType, sourcePackageID uuid.UUID, at time.Time) (domaincampaign.Campaign, bool, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	var (
		best     domaincampaign.Campaign
		found    bool
		specific bool
	)
	for _, c := range m.store.campaigns {
		if !c.IsActive || c.EventType != domaincampaign.CampaignEventType(eventType) {
			continue
		}
		if at.Before(c.StartsAt) {
			continue
		}
		if c.EndsAt != nil && !at.Before(*c.EndsAt) {
			continue
		}
		matchSpecific := c.SourcePackageID != nil && *c.SourcePackageID == sourcePackageID
		matchGeneric := c.SourcePackageID == nil
		if !matchSpecific && !matchGeneric {
			continue
		}
		betterTieBreak := c.CreatedAt.After(best.CreatedAt) ||
			(c.CreatedAt.Equal(best.CreatedAt) && c.ID.String() > best.ID.String())
		if !found ||
			(matchSpecific && !specific) ||
			(matchSpecific == specific && betterTieBreak) {
			best = c
			found = true
			specific = matchSpecific
		}
	}
	return best, found, nil
}

func (m memoryRuntimeRepo) ListAssignmentsExpiringOnLocalDay(_ context.Context, targetDay time.Time, loc *time.Location, afterAssignmentID *uuid.UUID, limit int) ([]domainpackaging.AdvertPackageAssignment, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	ids := make([]uuid.UUID, 0)
	for id, a := range m.store.assignments {
		if a.Status != domainpackaging.AssignmentStatusActive || a.EndsAt == nil {
			continue
		}
		if !domainnotification.AssignmentEndsOnLocalDay(*a.EndsAt, targetDay, loc) {
			continue
		}
		if afterAssignmentID != nil && id.String() <= afterAssignmentID.String() {
			continue
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	if len(ids) > limit {
		ids = ids[:limit]
	}
	out := make([]domainpackaging.AdvertPackageAssignment, 0, len(ids))
	for _, id := range ids {
		out = append(out, m.store.assignments[id])
	}
	return out, nil
}

func (m memoryRuntimeRepo) ListActiveAssignmentsPastEndsAt(_ context.Context, before time.Time, limit int) ([]domainpackaging.AdvertPackageAssignment, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	ids := make([]uuid.UUID, 0)
	for id, a := range m.store.assignments {
		if a.Status != domainpackaging.AssignmentStatusActive || a.EndsAt == nil {
			continue
		}
		if a.EndsAt.After(before) {
			continue
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	if len(ids) > limit {
		ids = ids[:limit]
	}
	out := make([]domainpackaging.AdvertPackageAssignment, 0, len(ids))
	for _, id := range ids {
		out = append(out, m.store.assignments[id])
	}
	return out, nil
}

func (m memoryRuntimeRepo) MarkAssignmentExpired(_ context.Context, assignmentID uuid.UUID, expiredAt, updatedAt time.Time) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	a, ok := m.store.assignments[assignmentID]
	if !ok {
		return apperr.NotFound(memoryAssignmentNotFound)
	}
	a.Status = domainpackaging.AssignmentStatusExpired
	a.ExpiredAt = &expiredAt
	a.UpdatedAt = updatedAt
	m.store.assignments[assignmentID] = a
	return nil
}

// DeactivateActiveUrgentForAdvert deactivates the advert's active URGENT
// feature activation (if any), mirroring the packaging domain's
// deactivateUrgentForPackageLoss behavior for the expiry-driven case.
func (m memoryRuntimeRepo) DeactivateActiveUrgentForAdvert(_ context.Context, advertID uuid.UUID, reason string, deactivatedAt, updatedAt time.Time) (bool, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	a, ok := m.store.urgent[advertID]
	if !ok || !a.IsActive() {
		return false, nil
	}
	a.Status = domainpackaging.FeatureActivationStatusDeactivated
	a.DeactivatedAt = &deactivatedAt
	r := reason
	a.Reason = &r
	a.UpdatedAt = updatedAt
	m.store.urgent[advertID] = a
	return true, nil
}

type memoryJobEnqueuer struct{ store *MemoryRuntimeStore }

// WithTx is a no-op for the in-memory adapter: the memory store has no real
// transaction boundary, so enqueues are always visible immediately. It
// satisfies the JobEnqueuer interface used by production code paths that
// scope enqueues to the caller's pgx.Tx.
func (m memoryJobEnqueuer) WithTx(_ pgx.Tx) JobEnqueuer { return m }

func (m memoryJobEnqueuer) EnqueueJob(_ context.Context, job domainmedia.BackgroundJob) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	if m.store.jobDedup == nil {
		m.store.jobDedup = map[string]struct{}{}
	}
	if job.DeduplicationKey != nil {
		if _, ok := m.store.jobDedup[*job.DeduplicationKey]; ok {
			return apperr.Conflict("İş kaydı zaten kuyruğa alındı.")
		}
		m.store.jobDedup[*job.DeduplicationKey] = struct{}{}
	}
	m.store.jobs = append(m.store.jobs, job)
	return nil
}

type memoryAdvertReader struct{ store *MemoryRuntimeStore }

func (m memoryAdvertReader) GetAdvertSnapshot(_ context.Context, advertID uuid.UUID) (AdvertSnapshot, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	a, ok := m.store.adverts[advertID]
	if !ok {
		return AdvertSnapshot{}, apperr.NotFound("İlan bulunamadı.")
	}
	return a, nil
}

type memoryPackageReader struct{ store *MemoryRuntimeStore }

func (m memoryPackageReader) GetPackageByID(_ context.Context, packageID uuid.UUID) (domainpackaging.Package, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	p, ok := m.store.packages[packageID]
	if !ok {
		return domainpackaging.Package{}, apperr.NotFound(memoryPackageNotFound)
	}
	return p, nil
}

func (m memoryPackageReader) GetEffectiveAssignment(_ context.Context, advertID uuid.UUID, at time.Time) (PackageAssignmentSnapshot, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	for _, a := range m.store.assignments {
		if a.AdvertID != advertID || !a.IsEffectiveAt(at) {
			continue
		}
		return PackageAssignmentSnapshot{ID: a.ID, AdvertID: a.AdvertID, PackageID: a.PackageID, EndsAt: a.EndsAt}, nil
	}
	return PackageAssignmentSnapshot{}, apperr.NotFound(memoryAssignmentNotFound)
}

func (m memoryPackageReader) GetAssignmentByID(_ context.Context, assignmentID uuid.UUID) (PackageAssignmentSnapshot, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	a, ok := m.store.assignments[assignmentID]
	if !ok {
		return PackageAssignmentSnapshot{}, apperr.NotFound(memoryAssignmentNotFound)
	}
	return PackageAssignmentSnapshot{ID: a.ID, AdvertID: a.AdvertID, PackageID: a.PackageID, EndsAt: a.EndsAt}, nil
}

func (m memoryPackageReader) FindActiveUrgent(_ context.Context, advertID uuid.UUID) (domainpackaging.AdvertFeatureActivation, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	a, ok := m.store.urgent[advertID]
	if !ok || !a.IsActive() {
		return domainpackaging.AdvertFeatureActivation{}, apperr.NotFound("Acil özellik bulunamadı.")
	}
	return a, nil
}

type memoryVerifiedUsers struct{ store *MemoryRuntimeStore }

func (m memoryVerifiedUsers) FindByID(_ context.Context, id uuid.UUID) (domainuser.User, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	u, ok := m.store.users[id]
	if !ok {
		return domainuser.User{}, apperr.NotFound("Kullanıcı bulunamadı.")
	}
	return u, nil
}

type memoryRuntimeTx struct{ store *MemoryRuntimeStore }

func (t *memoryRuntimeTx) Begin(context.Context) (pgx.Tx, error) { return t, nil }
func (t *memoryRuntimeTx) Commit(context.Context) error          { return nil }
func (t *memoryRuntimeTx) Rollback(context.Context) error        { return nil }
func (t *memoryRuntimeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *memoryRuntimeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (t *memoryRuntimeTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (t *memoryRuntimeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *memoryRuntimeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (t *memoryRuntimeTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (t *memoryRuntimeTx) QueryRow(context.Context, string, ...any) pgx.Row        { return nil }
func (t *memoryRuntimeTx) Conn() *pgx.Conn                                         { return nil }

var _ RuntimeRepository = memoryRuntimeRepo{}

// EmptyPayload returns canonical empty JSON object bytes.
func EmptyPayload() []byte { return []byte(`{}`) }
