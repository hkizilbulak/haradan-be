package notification_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	appnotification "github.com/hkizilbulak/haradan-be/internal/application/notification"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
)

func TestProcessExpiryScanUsesReferenceDateForTargetDayOnly(t *testing.T) {
	t.Parallel()
	loc, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		t.Fatal(err)
	}
	// Real clock is 2026-08-10 morning; referenceDate backfills as if today were 2026-08-05.
	now := time.Date(2026, 8, 10, 6, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}
	store := appnotification.NewMemoryRuntimeStore()

	ownerID := uuid.New()
	pkgID := uuid.New()
	store.PutPackage(domainpackaging.Package{
		ID: pkgID, Code: domainpackaging.PackageCode("ADVANCED"), DisplayName: "Advanced", BroadcastOnPublish: true,
	})
	for _, et := range []domainnotification.EventType{
		domainnotification.TemplateEventTypePackageExpiry5Days,
		domainnotification.TemplateEventTypePackageExpiry1Day,
	} {
		store.PutTemplate(domainnotification.NotificationTemplate{
			ID: uuid.New(), EventType: et, Name: string(et),
			InAppTitleTemplate: "{{.advertTitle}}", InAppBodyTemplate: "{{.advertTitle}}",
			IsActive: true, Version: 1,
		})
	}

	// Ends later on 2026-08-10 Istanbul (still after real clock) → 5D target for logical 2026-08-05.
	ends5D := time.Date(2026, 8, 10, 20, 0, 0, 0, loc).UTC()
	advert5 := uuid.New()
	asg5 := uuid.New()
	store.PutAdvert(appnotification.AdvertSnapshot{ID: advert5, OwnerUserID: ownerID, Title: "FiveDay", Status: "PUBLISHED"})
	store.PutAssignment(domainpackaging.AdvertPackageAssignment{
		ID: asg5, AdvertID: advert5, PackageID: pkgID, Status: domainpackaging.AssignmentStatusActive,
		StartsAt: now.Add(-10 * 24 * time.Hour), EndsAt: &ends5D,
	})

	// Already past real clock → must still expire via ExpireDueAssignments (real now).
	pastEnds := now.Add(-time.Hour)
	advertPast := uuid.New()
	asgPast := uuid.New()
	store.PutAdvert(appnotification.AdvertSnapshot{ID: advertPast, OwnerUserID: ownerID, Title: "Past", Status: "PUBLISHED"})
	store.PutAssignment(domainpackaging.AdvertPackageAssignment{
		ID: asgPast, AdvertID: advertPast, PackageID: pkgID, Status: domainpackaging.AssignmentStatusActive,
		StartsAt: now.Add(-48 * time.Hour), EndsAt: &pastEnds,
	})

	// Future relative to real clock and not on referenceDate target days → no reminder.
	futureEnds := time.Date(2026, 9, 1, 12, 0, 0, 0, loc).UTC()
	advertFuture := uuid.New()
	asgFuture := uuid.New()
	store.PutAdvert(appnotification.AdvertSnapshot{ID: advertFuture, OwnerUserID: ownerID, Title: "Future", Status: "PUBLISHED"})
	store.PutAssignment(domainpackaging.AdvertPackageAssignment{
		ID: asgFuture, AdvertID: advertFuture, PackageID: pkgID, Status: domainpackaging.AssignmentStatusActive,
		StartsAt: now, EndsAt: &futureEnds,
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
		Adverts: store.AdvertReader(), Users: store.UserReader(), Clock: clock, Timezone: loc,
	})
	if err != nil {
		t.Fatal(err)
	}

	payload, _ := json.Marshal(map[string]string{"referenceDate": "2026-08-05"})
	if err := svc.ProcessExpiryScan(context.Background(), payload); err != nil {
		t.Fatal(err)
	}

	// Past assignment expired by real clock.
	stillDue, err := store.RuntimeRepo().ListActiveAssignmentsPastEndsAt(context.Background(), now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(stillDue) != 0 {
		t.Fatalf("expected past assignment expired by real clock, still due=%d", len(stillDue))
	}

	notifs := store.Notifications()
	if len(notifs) != 1 {
		t.Fatalf("expected one 5D reminder for referenceDate target, got %d", len(notifs))
	}
	if !strings.Contains(string(notifs[0].Payload), asg5.String()) {
		t.Fatalf("expected reminder for 5D assignment, payload=%s", notifs[0].Payload)
	}
}

func TestProcessExpiryScanContinuationPreservesReferenceDate(t *testing.T) {
	t.Parallel()
	loc, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}
	store := appnotification.NewMemoryRuntimeStore()
	ownerID := uuid.New()
	pkgID := uuid.New()
	store.PutPackage(domainpackaging.Package{
		ID: pkgID, Code: domainpackaging.PackageCode("ADVANCED"), DisplayName: "Advanced", BroadcastOnPublish: true,
	})
	store.PutTemplate(domainnotification.NotificationTemplate{
		ID: uuid.New(), EventType: domainnotification.TemplateEventTypePackageExpiry5Days,
		Name: "5d", InAppTitleTemplate: "{{.advertTitle}}", InAppBodyTemplate: "{{.advertTitle}}",
		IsActive: true, Version: 1,
	})

	ends5D := time.Date(2026, 8, 10, 15, 0, 0, 0, loc).UTC()
	ids := make([]uuid.UUID, 0, 3)
	for i := 0; i < 3; i++ {
		advertID := uuid.New()
		asgID := uuid.New()
		ids = append(ids, asgID)
		store.PutAdvert(appnotification.AdvertSnapshot{ID: advertID, OwnerUserID: ownerID, Title: "A", Status: "PUBLISHED"})
		store.PutAssignment(domainpackaging.AdvertPackageAssignment{
			ID: asgID, AdvertID: advertID, PackageID: pkgID, Status: domainpackaging.AssignmentStatusActive,
			StartsAt: now.Add(-10 * 24 * time.Hour), EndsAt: &ends5D,
		})
	}

	writer, err := appnotification.NewEventWriter(appnotification.EventWriterConfig{
		Repo: store.RuntimeRepo(), Jobs: store.JobEnqueuer(),
		Adverts: store.AdvertReader(), Packages: store.PackageReader(), Clock: clock,
	})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := appnotification.NewExpiryScanService(appnotification.ExpiryScanConfig{
		Writer: writer, Repo: store.RuntimeRepo(), Jobs: store.JobEnqueuer(),
		Adverts: store.AdvertReader(), Users: store.UserReader(), Clock: clock, Timezone: loc, BatchSize: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	payload, _ := json.Marshal(map[string]string{
		"offset":        "5D",
		"referenceDate": "2026-08-05",
	})
	if err := svc.ProcessExpiryScan(context.Background(), payload); err != nil {
		t.Fatal(err)
	}

	cont := findJobPayload(t, store, domainmedia.JobPackageExpiryReminderScan)
	var got map[string]string
	if err := json.Unmarshal(cont, &got); err != nil {
		t.Fatal(err)
	}
	if got["referenceDate"] != "2026-08-05" {
		t.Fatalf("continuation lost referenceDate: %#v", got)
	}
	if got["offset"] != "5D" || got["afterAssignmentId"] == "" {
		t.Fatalf("continuation missing cursor fields: %#v", got)
	}
}
