package notification_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	appnotification "github.com/hkizilbulak/haradan-be/internal/application/notification"
	domaincampaign "github.com/hkizilbulak/haradan-be/internal/domain/campaign"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

// findJobPayload returns the payload of the first enqueued job of jobType,
// failing the test if none was enqueued.
func findJobPayload(t *testing.T, store *appnotification.MemoryRuntimeStore, jobType domainmedia.JobType) []byte {
	t.Helper()
	for _, j := range store.Jobs() {
		if j.JobType == jobType {
			return j.Payload
		}
	}
	t.Fatalf("no job of type %s was enqueued", jobType)
	return nil
}

func TestEventWriterDedupAndInactiveTemplateNoOp(t *testing.T) {
	t.Parallel()
	store := appnotification.NewMemoryRuntimeStore()
	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}

	advertID := uuid.New()
	asgID := uuid.New()
	pkgID := uuid.New()
	store.PutAdvert(appnotification.AdvertSnapshot{
		ID: advertID, OwnerUserID: uuid.New(), Title: "Test Horse", Status: "PUBLISHED",
	})
	store.PutPackage(domainpackaging.Package{ID: pkgID, Code: domainpackaging.PackageCode("ADVANCED"), DisplayName: "Advanced", BroadcastOnPublish: true})
	store.PutAssignment(domainpackaging.AdvertPackageAssignment{
		ID: asgID, AdvertID: advertID, PackageID: pkgID, Status: domainpackaging.AssignmentStatusActive,
		StartsAt: now.Add(-time.Hour),
	})

	writer, err := appnotification.NewEventWriter(appnotification.EventWriterConfig{
		Repo: store.RuntimeRepo(), Jobs: store.JobEnqueuer(),
		Adverts: store.AdvertReader(), Packages: store.PackageReader(), Clock: clock,
	})
	if err != nil {
		t.Fatal(err)
	}

	tx, err := store.RuntimeRepo().BeginTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	in := appnotification.WritePackageAdvertPublishedInput{AdvertID: advertID, AssignmentID: asgID}
	if err := writer.WritePackageAdvertPublished(context.Background(), tx, in); err != nil {
		t.Fatalf("inactive template should no-op: %v", err)
	}
	if len(store.Notifications()) != 0 {
		t.Fatal("expected no notification when template inactive")
	}

	store.PutTemplate(domainnotification.NotificationTemplate{
		ID: uuid.New(), EventType: domainnotification.TemplateEventTypePackageAdvertPublished,
		Name: "T", InAppTitleTemplate: "{{.advertTitle}}", InAppBodyTemplate: "{{.advertTitle}}",
		IsActive: true, Version: 1,
	})

	if err := writer.WritePackageAdvertPublished(context.Background(), tx, in); err != nil {
		t.Fatal(err)
	}
	if err := writer.WritePackageAdvertPublished(context.Background(), tx, in); err != nil {
		t.Fatal(err)
	}
	if len(store.Notifications()) != 1 {
		t.Fatalf("expected dedup notification, got %d", len(store.Notifications()))
	}
	if len(store.Jobs()) != 1 {
		t.Fatalf("expected one fanout job, got %d", len(store.Jobs()))
	}
}

func TestFanoutInsertsEligibleUsersAndSkipsSentEmail(t *testing.T) {
	t.Parallel()
	store := appnotification.NewMemoryRuntimeStore()
	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}

	u1 := uuid.New()
	u2 := uuid.New()
	store.PutUser(domainuser.User{ID: u1, Email: "a@example.com", Status: domainuser.StatusActive, EmailVerifiedAt: &now})
	store.PutUser(domainuser.User{ID: u2, Email: "b@example.com", Status: domainuser.StatusActive, EmailVerifiedAt: &now})

	nID := uuid.New()
	store.PutTemplate(domainnotification.NotificationTemplate{
		ID: uuid.New(), EventType: domainnotification.TemplateEventTypePackageAdvertPublished,
		Name: "T", InAppTitleTemplate: "x", InAppBodyTemplate: "y",
		ResendTemplateID: strPtr("tmpl_test"), IsActive: true, Version: 1,
	})
	repo := store.RuntimeRepo()
	_, _ = repo.CreateNotificationEventIdempotent(context.Background(), domainnotification.Notification{
		ID: nID, EventType: domainnotification.TemplateEventTypePackageAdvertPublished,
		EventKey: "k", Title: "T", Body: "B", Payload: appnotification.EmptyPayload(), CreatedAt: now,
	})
	// u2 already has a SENT state from an earlier run; the fanout insert must
	// leave it untouched (ON CONFLICT DO NOTHING) and the chunk handler must
	// skip it on any re-delivery instead of re-sending.
	_, _ = repo.InsertUserNotificationStates(context.Background(), []domainnotification.UserNotificationState{{
		UserID: u2, NotificationID: nID, DeliveredAt: now, EmailStatus: domainnotification.EmailStatusSent,
		EmailIdempotencyKey: strPtr(domainnotification.AdvertNotificationEmailIdempotencyKey(nID, u2)),
		EmailSentAt:         &now, CreatedAt: now, UpdatedAt: now,
	}})

	sender := &recordingEmailSender{}
	svc, err := appnotification.NewFanoutService(appnotification.FanoutConfig{
		Repo: repo, Jobs: store.JobEnqueuer(), Email: sender, Users: store.UserReader(),
		Clock: clock, FanoutBatchSize: 10, EmailChunkSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	payload := []byte(`{"notificationId":"` + nID.String() + `"}`)
	if err := svc.ProcessAdvertFanout(context.Background(), domainmedia.JobNotificationFanoutPackageAdvert, payload); err != nil {
		t.Fatal(err)
	}
	chunkPayload := findJobPayload(t, store, domainmedia.JobEmailSendAdvertNotificationChunk)
	if err := svc.ProcessAdvertEmailChunk(context.Background(), chunkPayload); err != nil {
		t.Fatal(err)
	}
	if sender.sent != 1 {
		t.Fatalf("expected one email (skip SENT user), got %d", sender.sent)
	}
}

// TestFanoutQueuesVerifiedSkipsUnverified pins the product rule that only
// users with a verified, non-blank email get EmailStatusQueued (+ a
// deterministic idempotency key); everyone else gets NOT_REQUESTED and must
// never appear in an email chunk payload (no address, no id).
func TestFanoutQueuesVerifiedSkipsUnverified(t *testing.T) {
	t.Parallel()
	store := appnotification.NewMemoryRuntimeStore()
	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}

	verified := uuid.New()
	unverified := uuid.New()
	store.PutUser(domainuser.User{ID: verified, Email: "verified@example.com", Status: domainuser.StatusActive, EmailVerifiedAt: &now})
	store.PutUser(domainuser.User{ID: unverified, Email: "unverified@example.com", Status: domainuser.StatusActive})

	nID := uuid.New()
	store.PutTemplate(domainnotification.NotificationTemplate{
		ID: uuid.New(), EventType: domainnotification.TemplateEventTypePackageAdvertPublished,
		Name: "T", InAppTitleTemplate: "x", InAppBodyTemplate: "y",
		ResendTemplateID: strPtr("tmpl_test"), IsActive: true, Version: 1,
	})
	repo := store.RuntimeRepo()
	_, _ = repo.CreateNotificationEventIdempotent(context.Background(), domainnotification.Notification{
		ID: nID, EventType: domainnotification.TemplateEventTypePackageAdvertPublished,
		EventKey: "k", Title: "T", Body: "B", Payload: appnotification.EmptyPayload(), CreatedAt: now,
	})

	svc, err := appnotification.NewFanoutService(appnotification.FanoutConfig{
		Repo: repo, Jobs: store.JobEnqueuer(), Email: &recordingEmailSender{}, Users: store.UserReader(),
		Clock: clock, FanoutBatchSize: 10, EmailChunkSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"notificationId":"` + nID.String() + `"}`)
	if err := svc.ProcessAdvertFanout(context.Background(), domainmedia.JobNotificationFanoutPackageAdvert, payload); err != nil {
		t.Fatal(err)
	}

	states, err := repo.GetEmailDeliveryStates(context.Background(), nID, []uuid.UUID{verified, unverified})
	if err != nil {
		t.Fatal(err)
	}
	byUser := map[uuid.UUID]domainnotification.UserNotificationState{}
	for _, st := range states {
		byUser[st.UserID] = st
	}
	if got := byUser[verified].EmailStatus; got != domainnotification.EmailStatusQueued {
		t.Fatalf("verified user email status = %q, want QUEUED", got)
	}
	if byUser[verified].EmailIdempotencyKey == nil {
		t.Fatal("verified user must have a deterministic idempotency key")
	}
	if got := byUser[unverified].EmailStatus; got != domainnotification.EmailStatusNotRequested {
		t.Fatalf("unverified user email status = %q, want NOT_REQUESTED", got)
	}

	chunkPayload := findJobPayload(t, store, domainmedia.JobEmailSendAdvertNotificationChunk)
	if strings.Contains(string(chunkPayload), unverified.String()) {
		t.Fatalf("chunk payload must not include the unverified user: %s", chunkPayload)
	}
	if strings.Contains(string(chunkPayload), "@") {
		t.Fatalf("chunk payload must never carry an email address: %s", chunkPayload)
	}
	if !strings.Contains(string(chunkPayload), verified.String()) {
		t.Fatalf("chunk payload must include the verified user id: %s", chunkPayload)
	}
}

// TestExpireDueAssignmentsDeactivatesUrgent pins that expiring an assignment
// also deactivates any active URGENT feature it was carrying, with reason
// PACKAGE_EXPIRED, mirroring the packaging domain's own loss-of-package path.
func TestExpireDueAssignmentsDeactivatesUrgent(t *testing.T) {
	t.Parallel()
	store := appnotification.NewMemoryRuntimeStore()
	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}

	advertID := uuid.New()
	asgID := uuid.New()
	pkgID := uuid.New()
	ownerID := uuid.New()
	pastEndsAt := now.Add(-time.Hour)
	store.PutAdvert(appnotification.AdvertSnapshot{ID: advertID, OwnerUserID: ownerID, Title: "Test Horse", Status: "PUBLISHED"})
	store.PutPackage(domainpackaging.Package{ID: pkgID, Code: domainpackaging.PackageCode("ADVANCED"), DisplayName: "Advanced", BroadcastOnPublish: true})
	store.PutAssignment(domainpackaging.AdvertPackageAssignment{
		ID: asgID, AdvertID: advertID, PackageID: pkgID, Status: domainpackaging.AssignmentStatusActive,
		StartsAt: now.Add(-48 * time.Hour), EndsAt: &pastEndsAt,
	})
	store.PutUrgent(domainpackaging.AdvertFeatureActivation{
		ID: uuid.New(), AdvertID: advertID, PackageAssignmentID: asgID, FeatureCode: domainpackaging.FeatureCodeUrgent,
		Status: domainpackaging.FeatureActivationStatusActive, ActivatedByUserID: ownerID, ActivatedAt: now.Add(-48 * time.Hour),
		ActivationVersion: 1, CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: now.Add(-48 * time.Hour),
	})

	writer, err := appnotification.NewEventWriter(appnotification.EventWriterConfig{
		Repo: store.RuntimeRepo(), Jobs: store.JobEnqueuer(),
		Adverts: store.AdvertReader(), Packages: store.PackageReader(), Clock: clock,
	})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := appnotification.NewExpiryScanService(appnotification.ExpiryScanConfig{
		Writer: writer, Repo: store.RuntimeRepo(), Jobs: store.JobEnqueuer(),
		Adverts: store.AdvertReader(), Users: store.UserReader(), Clock: clock,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.ExpireDueAssignments(context.Background()); err != nil {
		t.Fatal(err)
	}

	stillDue, err := store.RuntimeRepo().ListActiveAssignmentsPastEndsAt(context.Background(), now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(stillDue) != 0 {
		t.Fatalf("expected no ACTIVE assignments past ends_at after expiry, got %d", len(stillDue))
	}
	if _, err := store.PackageReader().GetAssignmentByID(context.Background(), asgID); err != nil {
		t.Fatal(err)
	}

	urgent, err := store.PackageReader().FindActiveUrgent(context.Background(), advertID)
	if err == nil {
		t.Fatalf("expected urgent feature to be deactivated, still active: %+v", urgent)
	}
}

func TestPackageExpiryUsesIstanbulCalendarDays(t *testing.T) {
	t.Parallel()
	loc, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		t.Fatal(err)
	}
	endsAt := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	if got := domainnotification.CalendarDaysUntil(endsAt, now, loc); got != 5 {
		t.Fatalf("calendar days=%d want 5", got)
	}
	target := domainnotification.PackageExpiryTargetDay(now, loc, domainnotification.PackageExpiryDayOffset5D)
	if target.In(loc).Day() != 10 {
		t.Fatalf("target day=%v", target.In(loc))
	}
}

func TestFindBestActiveCampaignPrefersSpecificPackage(t *testing.T) {
	t.Parallel()
	store := appnotification.NewMemoryRuntimeStore()
	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	pkgID := uuid.New()
	generic := domaincampaignFixture(now, nil)
	specific := domaincampaignFixture(now, &pkgID)
	specific.Code = "SPECIFIC"
	specific.CreatedAt = now.Add(time.Hour)
	store.PutCampaign(generic)
	store.PutCampaign(specific)

	got, ok, err := store.RuntimeRepo().FindBestActiveCampaignForExpiry(
		context.Background(), domainnotification.TemplateEventTypePackageExpiry5Days, pkgID, now,
	)
	if err != nil || !ok {
		t.Fatalf("err=%v ok=%v", err, ok)
	}
	if got.Code != "SPECIFIC" {
		t.Fatalf("got %q want SPECIFIC", got.Code)
	}
}

// TestListUserNotificationsCursorOrdersByEventCreatedAt pins that the inbox
// cursor sorts and pages by the notification's created_at/id (when the
// underlying event happened), not by delivered_at: two notifications
// delivered in the same fan-out batch (same delivered_at) must still come
// back in event order.
func TestListUserNotificationsCursorOrdersByEventCreatedAt(t *testing.T) {
	t.Parallel()
	store := appnotification.NewMemoryRuntimeStore()
	repo := store.RuntimeRepo()
	userID := uuid.New()
	deliveredAt := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)

	oldest := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	middle := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	newest := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	n1 := domainnotification.Notification{ID: uuid.New(), EventType: domainnotification.TemplateEventTypePackageAdvertPublished, EventKey: "k1", Title: "1", Body: "b", Payload: appnotification.EmptyPayload(), CreatedAt: oldest}
	n2 := domainnotification.Notification{ID: uuid.New(), EventType: domainnotification.TemplateEventTypePackageAdvertPublished, EventKey: "k2", Title: "2", Body: "b", Payload: appnotification.EmptyPayload(), CreatedAt: middle}
	n3 := domainnotification.Notification{ID: uuid.New(), EventType: domainnotification.TemplateEventTypePackageAdvertPublished, EventKey: "k3", Title: "3", Body: "b", Payload: appnotification.EmptyPayload(), CreatedAt: newest}
	for _, n := range []domainnotification.Notification{n1, n2, n3} {
		if _, err := repo.CreateNotificationEventIdempotent(context.Background(), n); err != nil {
			t.Fatal(err)
		}
		// All three are delivered together in the same fan-out batch: same
		// delivered_at for every row, so only created_at/id can order them.
		if _, err := repo.InsertUserNotificationStates(context.Background(), []domainnotification.UserNotificationState{{
			UserID: userID, NotificationID: n.ID, DeliveredAt: deliveredAt,
			EmailStatus: domainnotification.EmailStatusNotRequested, CreatedAt: deliveredAt, UpdatedAt: deliveredAt,
		}}); err != nil {
			t.Fatal(err)
		}
	}

	svc, err := appnotification.NewUserNotificationService(appnotification.UserNotificationConfig{Repo: repo})
	if err != nil {
		t.Fatal(err)
	}

	page1, err := svc.ListUserNotifications(context.Background(), appnotification.ListUserNotificationsInput{UserID: userID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page1.Items) != 2 || page1.Items[0].Notification.ID != n3.ID || page1.Items[1].Notification.ID != n2.ID {
		t.Fatalf("page1 = %+v, want [n3, n2]", page1.Items)
	}
	if !page1.HasMore || page1.NextCursor == nil {
		t.Fatal("expected a second page")
	}

	page2, err := svc.ListUserNotifications(context.Background(), appnotification.ListUserNotificationsInput{UserID: userID, Limit: 2, Cursor: page1.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Items) != 1 || page2.Items[0].Notification.ID != n1.ID {
		t.Fatalf("page2 = %+v, want [n1]", page2.Items)
	}
	if page2.HasMore {
		t.Fatal("expected no more pages")
	}
}

type recordingEmailSender struct{ sent int }

func (r *recordingEmailSender) SendTemplateEmail(context.Context, string, string, *string, map[string]string, string) error {
	r.sent++
	return nil
}

func domaincampaignFixture(now time.Time, source *uuid.UUID) domaincampaign.Campaign {
	return domaincampaign.Campaign{
		ID: uuid.New(), Code: "GENERIC", Name: "N", EventType: domaincampaign.CampaignEventTypePackageExpiry5Days,
		SourcePackageID: source, Title: "Campaign", IsActive: true, CreatedByUserID: uuid.New(),
		CurrencyCode: "TRY", StartsAt: now.Add(-time.Hour), Version: 1, CreatedAt: now, UpdatedAt: now,
	}
}
