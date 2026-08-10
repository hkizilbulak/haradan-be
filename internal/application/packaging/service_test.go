package packaging_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	apppackaging "github.com/hkizilbulak/haradan-be/internal/application/packaging"
	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

type mutableClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *mutableClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *mutableClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type fixture struct {
	store  *apppackaging.MemoryStore
	svc    *apppackaging.Service
	clock  *mutableClock
	admin  domainuser.User
	owner  domainuser.User
	other  domainuser.User
	advert domainadvert.Advert
	starter,
	middle,
	advanced domainpackaging.Package
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	store := apppackaging.NewMemoryStore()
	clock := &mutableClock{t: now}
	svc, err := apppackaging.NewMemoryService(store, clock)
	if err != nil {
		t.Fatalf("NewMemoryService: %v", err)
	}

	admin := domainuser.User{ID: uuid.New(), Role: domainuser.RoleAdmin, Status: domainuser.StatusActive}
	owner := domainuser.User{ID: uuid.New(), Role: domainuser.RoleUser, Status: domainuser.StatusActive}
	other := domainuser.User{ID: uuid.New(), Role: domainuser.RoleUser, Status: domainuser.StatusActive}
	store.PutUser(admin)
	store.PutUser(owner)
	store.PutUser(other)

	starterID := uuid.MustParse("a0000000-0000-4000-8000-000000000001")
	middleID := uuid.MustParse("a0000000-0000-4000-8000-000000000002")
	advancedID := uuid.MustParse("a0000000-0000-4000-8000-000000000003")
	starter := domainpackaging.Package{
		ID: starterID, Code: domainpackaging.PackageCode("STARTER"), DisplayName: "Starter",
		CurrencyCode: "TRY", IsActive: true, SortOrder: 10, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	middle := domainpackaging.Package{
		ID: middleID, Code: domainpackaging.PackageCode("MIDDLE"), DisplayName: "Middle",
		CurrencyCode: "TRY", ShowcaseEligible: true, IsActive: true, SortOrder: 20, Version: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	dur := 30
	advanced := domainpackaging.Package{
		ID: advancedID, Code: domainpackaging.PackageCode("ADVANCED"), DisplayName: "Advanced", BroadcastOnPublish: true,
		CurrencyCode: "TRY", DefaultDurationDays: &dur, AllowsUrgent: true, ShowcaseEligible: true,
		SearchPriority: 100, IsActive: true, SortOrder: 30, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	store.PutPackage(starter)
	store.PutPackage(middle)
	store.PutPackage(advanced)

	advert := domainadvert.Advert{
		ID: uuid.New(), OwnerUserID: owner.ID, Status: domainadvert.StatusPublished,
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	store.PutAdvert(advert)

	return &fixture{
		store: store, svc: svc, clock: clock,
		admin: admin, owner: owner, other: other, advert: advert,
		starter: starter, middle: middle, advanced: advanced,
	}
}

func requireKind(t *testing.T, err error, kind apperr.Kind) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind != kind {
		t.Fatalf("want kind %v, got %v", kind, err)
	}
}

func TestListPackagesSortOrder(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	items, err := f.svc.ListPackages(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("len=%d", len(items))
	}
	if items[0].Code != domainpackaging.PackageCode("STARTER") ||
		items[1].Code != domainpackaging.PackageCode("MIDDLE") ||
		items[2].Code != domainpackaging.PackageCode("ADVANCED") {
		t.Fatalf("unexpected order: %+v", items)
	}
}

func TestAssignAdvertPackageAdminSuccessAndHistory(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	view, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("ADVANCED"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Assignment.Status != domainpackaging.AssignmentStatusActive {
		t.Fatalf("status=%s", view.Assignment.Status)
	}
	if view.Assignment.Source != domainpackaging.AssignmentSourceAdmin {
		t.Fatalf("source=%s", view.Assignment.Source)
	}
	if view.Assignment.EndsAt == nil {
		t.Fatal("expected default duration ends_at")
	}
	wantEnd := f.clock.Now().AddDate(0, 0, 30)
	if !view.Assignment.EndsAt.Equal(wantEnd) {
		t.Fatalf("ends_at=%v want %v", view.Assignment.EndsAt, wantEnd)
	}

	f.clock.Advance(time.Minute)
	_, err = f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("MIDDLE"),
	})
	if err != nil {
		t.Fatal(err)
	}
	hist, err := f.svc.ListAdvertPackageHistory(ctx, apppackaging.ListAdvertPackageHistoryInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, Limit: intPtr(10),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hist.Items) != 2 {
		t.Fatalf("history len=%d", len(hist.Items))
	}
	if hist.Items[0].Assignment.Status != domainpackaging.AssignmentStatusActive {
		t.Fatal("newest should be active")
	}
	if hist.Items[1].Assignment.Status != domainpackaging.AssignmentStatusSuperseded {
		t.Fatal("previous should be superseded")
	}
}

func TestAssignNonAdminForbidden(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.AssignAdvertPackage(context.Background(), apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.owner.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("STARTER"),
	})
	requireKind(t, err, apperr.KindForbidden)
}

func TestAssignPackageNotFoundAndInactive(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("bad code"),
	})
	requireKind(t, err, apperr.KindValidation)

	_, err = f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("NOPE"),
	})
	requireKind(t, err, apperr.KindNotFound)

	inactive := f.starter
	inactive.IsActive = false
	f.store.PutPackage(inactive)
	_, err = f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("STARTER"),
	})
	requireKind(t, err, apperr.KindConflict)
}

func TestAssignAdvertNotFoundAndTerminal(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: uuid.New(), PackageCode: domainpackaging.PackageCode("STARTER"),
	})
	requireKind(t, err, apperr.KindNotFound)

	sold := f.advert
	sold.ID = uuid.New()
	sold.Status = domainadvert.StatusSold
	f.store.PutAdvert(sold)
	_, err = f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: sold.ID, PackageCode: domainpackaging.PackageCode("STARTER"),
	})
	requireKind(t, err, apperr.KindConflict)
}

func TestAssignInvalidDateRangeAndNullDuration(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	start := f.clock.Now()
	end := start.Add(-time.Hour)
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("STARTER"),
		StartsAt: &start, EndsAt: &end,
	})
	requireKind(t, err, apperr.KindValidation)

	view, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("STARTER"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Assignment.EndsAt != nil {
		t.Fatalf("starter duration null should leave ends_at nil, got %v", view.Assignment.EndsAt)
	}
}

func TestAssignIdempotentSamePackageAndDates(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	first, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("STARTER"),
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("STARTER"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Assignment.ID != second.Assignment.ID {
		t.Fatal("expected idempotent same assignment id")
	}
	active := 0
	for _, a := range f.store.Assignments() {
		if a.AdvertID == f.advert.ID && a.Status == domainpackaging.AssignmentStatusActive {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("active=%d", active)
	}
}

func TestAssignDeactivatesUrgentOnPackageChange(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("ADVANCED"),
	})
	if err != nil {
		t.Fatal(err)
	}
	act, err := f.svc.ActivateUrgent(ctx, f.owner.ID, f.advert.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !act.IsActive() {
		t.Fatal("expected active urgent")
	}

	_, err = f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("MIDDLE"),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, feat := range f.store.Features() {
		if feat.AdvertID == f.advert.ID && feat.FeatureCode == domainpackaging.FeatureCodeUrgent {
			if feat.Status != domainpackaging.FeatureActivationStatusDeactivated {
				t.Fatalf("urgent status=%s", feat.Status)
			}
		}
	}

	// ADVANCED → ADVANCED also deactivates so URGENT cannot stay on superseded assignment.
	_, err = f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("ADVANCED"),
	})
	if err != nil {
		t.Fatal(err)
	}
	act2, err := f.svc.ActivateUrgent(ctx, f.owner.ID, f.advert.ID)
	if err != nil {
		t.Fatal(err)
	}
	if act2.ActivationVersion <= act.ActivationVersion {
		t.Fatalf("version did not increase: %d <= %d", act2.ActivationVersion, act.ActivationVersion)
	}
	_, err = f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("ADVANCED"),
		StartsAt: ptrTime(f.clock.Now().Add(time.Minute)),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, feat := range f.store.Features() {
		if feat.ID == act2.ID && feat.Status != domainpackaging.FeatureActivationStatusDeactivated {
			t.Fatal("urgent must deactivate when assignment changes even ADVANCED→ADVANCED")
		}
	}
}

func TestAssignConcurrentOnlyOneActive(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	codes := []domainpackaging.PackageCode{
		domainpackaging.PackageCode("STARTER"),
		domainpackaging.PackageCode("MIDDLE"),
	}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
				ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: codes[i],
			})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	ok := 0
	for err := range errs {
		if err == nil {
			ok++
		}
	}
	if ok == 0 {
		t.Fatal("expected at least one success")
	}
	active := 0
	for _, a := range f.store.Assignments() {
		if a.AdvertID == f.advert.ID && a.Status == domainpackaging.AssignmentStatusActive {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("active assignments=%d", active)
	}
}

func TestUrgentOwnerAdminAndForbidden(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("ADVANCED"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.ActivateUrgent(ctx, f.owner.ID, f.advert.ID); err != nil {
		t.Fatalf("owner: %v", err)
	}
	if err := f.svc.DeactivateUrgent(ctx, f.admin.ID, f.advert.ID); err != nil {
		t.Fatalf("admin deactivate: %v", err)
	}
	if _, err := f.svc.ActivateUrgent(ctx, f.admin.ID, f.advert.ID); err != nil {
		t.Fatalf("admin activate: %v", err)
	}
	_, err = f.svc.ActivateUrgent(ctx, f.other.ID, f.advert.ID)
	requireKind(t, err, apperr.KindForbidden)
}

func TestAssignNilStartsAtSamePackageIdempotentAcrossClock(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	first, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("STARTER"),
	})
	if err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(2 * time.Hour)
	second, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("STARTER"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Assignment.ID != second.Assignment.ID {
		t.Fatal("nil StartsAt/EndsAt same package must stay idempotent across clock")
	}
	if !second.Assignment.StartsAt.Equal(first.Assignment.StartsAt) {
		t.Fatal("starts_at must not be rewritten by later nil StartsAt assign")
	}
}

func TestAssignExplicitChangedDatesCreatesNew(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	start1 := f.clock.Now()
	end1 := start1.Add(24 * time.Hour)
	first, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("STARTER"),
		StartsAt: &start1, EndsAt: &end1,
	})
	if err != nil {
		t.Fatal(err)
	}
	start2 := start1.Add(time.Hour)
	end2 := end1.Add(time.Hour)
	second, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("STARTER"),
		StartsAt: &start2, EndsAt: &end2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Assignment.ID == second.Assignment.ID {
		t.Fatal("explicit different dates must create new assignment")
	}
}

func TestAssignDisabledAdminForbidden(t *testing.T) {
	f := newFixture(t)
	disabled := f.admin
	disabled.ID = uuid.New()
	disabled.Status = domainuser.StatusDisabled
	f.store.PutUser(disabled)
	_, err := f.svc.AssignAdvertPackage(context.Background(), apppackaging.AssignAdvertPackageInput{
		ActorUserID: disabled.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("STARTER"),
	})
	requireKind(t, err, apperr.KindForbidden)
}

func TestUrgentLifecycleDraftPendingPublishedAllowed(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("ADVANCED"),
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, st := range []domainadvert.Status{
		domainadvert.StatusDraft,
		domainadvert.StatusPendingReview,
		domainadvert.StatusPublished,
	} {
		a := f.advert
		a.Status = st
		f.store.PutAdvert(a)
		if _, err := f.svc.ActivateUrgent(ctx, f.owner.ID, f.advert.ID); err != nil {
			t.Fatalf("status %s owner activate: %v", st, err)
		}
		if err := f.svc.DeactivateUrgent(ctx, f.owner.ID, f.advert.ID); err != nil {
			t.Fatalf("status %s deactivate: %v", st, err)
		}
		if _, err := f.svc.ActivateUrgent(ctx, f.admin.ID, f.advert.ID); err != nil {
			t.Fatalf("status %s admin activate: %v", st, err)
		}
		if err := f.svc.DeactivateUrgent(ctx, f.admin.ID, f.advert.ID); err != nil {
			t.Fatalf("status %s admin deactivate: %v", st, err)
		}
	}
}

func TestUrgentLifecycleTerminalRejected(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("ADVANCED"),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range []domainadvert.Status{
		domainadvert.StatusSold,
		domainadvert.StatusArchived,
		domainadvert.StatusSuspended,
		domainadvert.StatusRejected,
		domainadvert.StatusChangesRequested,
	} {
		a := f.advert
		a.Status = st
		f.store.PutAdvert(a)
		_, err := f.svc.ActivateUrgent(ctx, f.owner.ID, f.advert.ID)
		requireKind(t, err, apperr.KindConflict)
	}
	deleted := f.advert
	deleted.Status = domainadvert.StatusPublished
	now := f.clock.Now()
	deleted.DeletedAt = &now
	f.store.PutAdvert(deleted)
	_, err = f.svc.ActivateUrgent(ctx, f.owner.ID, f.advert.ID)
	requireKind(t, err, apperr.KindNotFound)
}

func TestUrgentRequiresAdvancedAndAllowsUrgent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("MIDDLE"),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.ActivateUrgent(ctx, f.owner.ID, f.advert.ID)
	requireKind(t, err, apperr.KindConflict)

	advNoUrgent := f.advanced
	advNoUrgent.AllowsUrgent = false
	f.store.PutPackage(advNoUrgent)
	f.advanced = advNoUrgent
	_, err = f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("ADVANCED"),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.ActivateUrgent(ctx, f.owner.ID, f.advert.ID)
	requireKind(t, err, apperr.KindConflict)
}

func TestUrgentFutureAssignmentRejected(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	start := f.clock.Now().Add(24 * time.Hour)
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("ADVANCED"),
		StartsAt: &start,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.ActivateUrgent(ctx, f.owner.ID, f.advert.ID)
	requireKind(t, err, apperr.KindConflict)
}

func TestUrgentDisabledAdminForbidden(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("ADVANCED"),
	})
	if err != nil {
		t.Fatal(err)
	}
	disabled := f.admin
	disabled.ID = uuid.New()
	disabled.Status = domainuser.StatusDisabled
	f.store.PutUser(disabled)
	_, err = f.svc.ActivateUrgent(ctx, disabled.ID, f.advert.ID)
	requireKind(t, err, apperr.KindForbidden)
}

func TestGetAdvertPackageIgnoresExpiredActiveWindow(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	start := f.clock.Now().Add(-48 * time.Hour)
	end := f.clock.Now().Add(-time.Hour)
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("STARTER"),
		StartsAt: &start, EndsAt: &end,
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err := f.svc.GetAdvertPackage(ctx, f.advert.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view != nil {
		t.Fatal("expired ACTIVE window must not be returned as current package")
	}
}

func TestUrgentIdempotentAndVersionBump(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("ADVANCED"),
	})
	if err != nil {
		t.Fatal(err)
	}
	a1, err := f.svc.ActivateUrgent(ctx, f.owner.ID, f.advert.ID)
	if err != nil {
		t.Fatal(err)
	}
	a2, err := f.svc.ActivateUrgent(ctx, f.owner.ID, f.advert.ID)
	if err != nil {
		t.Fatal(err)
	}
	if a1.ID != a2.ID || a1.ActivationVersion != a2.ActivationVersion {
		t.Fatal("duplicate activate should be idempotent")
	}
	if err := f.svc.DeactivateUrgent(ctx, f.owner.ID, f.advert.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.DeactivateUrgent(ctx, f.owner.ID, f.advert.ID); err != nil {
		t.Fatal(err)
	}
	a3, err := f.svc.ActivateUrgent(ctx, f.owner.ID, f.advert.ID)
	if err != nil {
		t.Fatal(err)
	}
	if a3.ActivationVersion != a1.ActivationVersion+1 {
		t.Fatalf("version=%d", a3.ActivationVersion)
	}
}

func TestGetAdvertPackageNilWhenMissing(t *testing.T) {
	f := newFixture(t)
	view, err := f.svc.GetAdvertPackage(context.Background(), f.advert.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view != nil {
		t.Fatal("expected nil")
	}
}

func TestHistoryForbiddenForNonAdmin(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.ListAdvertPackageHistory(context.Background(), apppackaging.ListAdvertPackageHistoryInput{
		ActorUserID: f.owner.ID, AdvertID: f.advert.ID, Limit: intPtr(10),
	})
	requireKind(t, err, apperr.KindForbidden)
}

func TestErrorsDoNotLeakSQL(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.AssignAdvertPackage(context.Background(), apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: uuid.New(), PackageCode: domainpackaging.PackageCode("STARTER"),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, bad := range []string{"postgres://", "hrd_", "SQLSTATE", "23505"} {
		if contains(msg, bad) {
			t.Fatalf("leaked %q in %q", bad, msg)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}

func ptrTime(t time.Time) *time.Time { return &t }
func intPtr(v int) *int              { return &v }

func TestUpdatePackageOptimisticAndAllowsUrgentFalse(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("ADVANCED"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ActivateUrgent(ctx, f.owner.ID, f.advert.ID); err != nil {
		t.Fatal(err)
	}
	name := "Advanced Plus"
	falseVal := false
	updated, err := f.svc.UpdatePackage(ctx, apppackaging.UpdatePackageInput{
		ActorUserID: f.admin.ID, Code: domainpackaging.PackageCode("ADVANCED"),
		ExpectedVersion: 1, DisplayName: &name, AllowsUrgent: &falseVal,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != 2 || updated.DisplayName != name || updated.AllowsUrgent {
		t.Fatalf("unexpected update %#v", updated)
	}
	active := false
	for _, feat := range f.store.Features() {
		if feat.FeatureCode == domainpackaging.FeatureCodeUrgent &&
			feat.Status == domainpackaging.FeatureActivationStatusActive {
			active = true
		}
	}
	if active {
		t.Fatal("allowsUrgent false must deactivate active URGENT")
	}
	_, err = f.svc.UpdatePackage(ctx, apppackaging.UpdatePackageInput{
		ActorUserID: f.admin.ID, Code: domainpackaging.PackageCode("ADVANCED"),
		ExpectedVersion: 1, DisplayName: &name,
	})
	requireKind(t, err, apperr.KindConflict)
}

func TestCancelAdvertPackageIdempotentAndDeactivatesUrgent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.CancelAdvertPackage(ctx, apppackaging.CancelAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID,
	}); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID, PackageCode: domainpackaging.PackageCode("ADVANCED"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ActivateUrgent(ctx, f.owner.ID, f.advert.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.CancelAdvertPackage(ctx, apppackaging.CancelAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID,
	}); err != nil {
		t.Fatal(err)
	}
	view, err := f.svc.GetAdvertPackage(ctx, f.advert.ID)
	if err != nil || view != nil {
		t.Fatalf("expected no active package, view=%v err=%v", view, err)
	}
	for _, feat := range f.store.Features() {
		if feat.Status == domainpackaging.FeatureActivationStatusActive {
			t.Fatal("urgent must be deactivated on cancel")
		}
	}
	if err := f.svc.CancelAdvertPackage(ctx, apppackaging.CancelAdvertPackageInput{
		ActorUserID: f.admin.ID, AdvertID: f.advert.ID,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestUpdatePackageNonAdminForbidden(t *testing.T) {
	f := newFixture(t)
	name := "x"
	_, err := f.svc.UpdatePackage(context.Background(), apppackaging.UpdatePackageInput{
		ActorUserID: f.owner.ID, Code: domainpackaging.PackageCode("STARTER"),
		ExpectedVersion: 1, DisplayName: &name,
	})
	requireKind(t, err, apperr.KindForbidden)
}

func TestUpdatePackageNullableOmitNullValue(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	desc := "desc"
	badge := "badge"
	price := int64(1000)
	dur := 14
	updated, err := f.svc.UpdatePackage(ctx, apppackaging.UpdatePackageInput{
		ActorUserID: f.admin.ID, Code: domainpackaging.PackageCode("STARTER"), ExpectedVersion: 1,
		DescriptionSet: true, Description: &desc,
		BadgeTextSet: true, BadgeText: &badge,
		DisplayPriceSet: true, DisplayPriceAmountMinor: &price,
		DefaultDurationSet: true, DefaultDurationDays: &dur,
	})
	if err != nil {
		t.Fatal(err)
	}
	// omit keeps
	kept, err := f.svc.UpdatePackage(ctx, apppackaging.UpdatePackageInput{
		ActorUserID: f.admin.ID, Code: domainpackaging.PackageCode("STARTER"), ExpectedVersion: updated.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	if kept.Description == nil || *kept.Description != desc || kept.BadgeText == nil || *kept.BadgeText != badge ||
		kept.DisplayPriceAmountMinor == nil || *kept.DisplayPriceAmountMinor != price ||
		kept.DefaultDurationDays == nil || *kept.DefaultDurationDays != dur {
		t.Fatalf("omit changed values: %+v", kept)
	}
	// null clears
	cleared, err := f.svc.UpdatePackage(ctx, apppackaging.UpdatePackageInput{
		ActorUserID: f.admin.ID, Code: domainpackaging.PackageCode("STARTER"), ExpectedVersion: kept.Version,
		DescriptionSet: true, Description: nil,
		BadgeTextSet: true, BadgeText: nil,
		DisplayPriceSet: true, DisplayPriceAmountMinor: nil,
		DefaultDurationSet: true, DefaultDurationDays: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Description != nil || cleared.BadgeText != nil ||
		cleared.DisplayPriceAmountMinor != nil || cleared.DefaultDurationDays != nil {
		t.Fatalf("null did not clear: %+v", cleared)
	}
}
