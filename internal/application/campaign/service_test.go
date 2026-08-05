package campaign_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	appcampaign "github.com/hkizilbulak/haradan-be/internal/application/campaign"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincampaign "github.com/hkizilbulak/haradan-be/internal/domain/campaign"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fixture struct {
	store *appcampaign.MemoryStore
	svc   *appcampaign.Service
	clock fixedClock
	admin domainuser.User
	user  domainuser.User
	starter,
	advanced domainpackaging.Package
	asset domainmedia.Asset
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	store := appcampaign.NewMemoryStore()
	clock := fixedClock{t: now}
	svc, err := appcampaign.NewMemoryService(store, clock)
	if err != nil {
		t.Fatalf("NewMemoryService: %v", err)
	}

	admin := domainuser.User{ID: uuid.New(), Role: domainuser.RoleAdmin, Status: domainuser.StatusActive}
	user := domainuser.User{ID: uuid.New(), Role: domainuser.RoleUser, Status: domainuser.StatusActive}
	store.PutUser(admin)
	store.PutUser(user)

	starter := domainpackaging.Package{
		ID:   uuid.MustParse("a0000000-0000-4000-8000-000000000001"),
		Code: domainpackaging.PackageCode("STARTER"), DisplayName: "Starter",
		CurrencyCode: "TRY", IsActive: true, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	advanced := domainpackaging.Package{
		ID:   uuid.MustParse("a0000000-0000-4000-8000-000000000003"),
		Code: domainpackaging.PackageCode("ADVANCED"), DisplayName: "Advanced",
		CurrencyCode: "TRY", IsActive: true, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	store.PutPackage(starter)
	store.PutPackage(advanced)

	asset := domainmedia.Asset{ID: uuid.New(), OwnerUserID: admin.ID, CreatedAt: now, UpdatedAt: now}
	store.PutAsset(asset)

	return &fixture{
		store: store, svc: svc, clock: clock,
		admin: admin, user: user, starter: starter, advanced: advanced, asset: asset,
	}
}

func requireKind(t *testing.T, err error, kind apperr.Kind) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind != kind {
		t.Fatalf("want kind %v, got %v", kind, err)
	}
}

func requireCode(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	ae, ok := apperr.As(err)
	if !ok || ae.Code != code {
		t.Fatalf("want code %s, got %v", code, err)
	}
}

func baseCreate(f *fixture) appcampaign.CreateCampaignInput {
	orig := int64(10000)
	camp := int64(8000)
	src := string(domainpackaging.PackageCode("STARTER"))
	tgt := string(domainpackaging.PackageCode("ADVANCED"))
	return appcampaign.CreateCampaignInput{
		ActorUserID:                     f.admin.ID,
		Code:                            "RENEW-STARTER-1",
		Name:                            "Yenileme",
		EventType:                       domaincampaign.CampaignEventTypePackageRenewal,
		SourcePackageCode:               &src,
		TargetPackageCode:               &tgt,
		Title:                           "Paketinizi yenileyin",
		DisplayOriginalPriceAmountMinor: &orig,
		DisplayCampaignPriceAmountMinor: &camp,
		CurrencyCode:                    "TRY",
		StartsAt:                        f.clock.Now(),
		IsActive:                        true,
	}
}

func TestCreateCampaignSuccess(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	in := baseCreate(f)
	in.ImageAssetID = &f.asset.ID

	c, err := f.svc.CreateCampaign(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if c.Version != 1 || c.Code != "RENEW-STARTER-1" {
		t.Fatalf("%+v", c)
	}
	if c.SourcePackageID == nil || *c.SourcePackageID != f.starter.ID {
		t.Fatal("source package")
	}
	if c.TargetPackageID == nil || *c.TargetPackageID != f.advanced.ID {
		t.Fatal("target package")
	}
}

func TestCreateCampaignDuplicateCodeConflict(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.CreateCampaign(ctx, baseCreate(f)); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.CreateCampaign(ctx, baseCreate(f))
	requireKind(t, err, apperr.KindConflict)
}

func TestCreateCampaignNonAdminForbidden(t *testing.T) {
	f := newFixture(t)
	in := baseCreate(f)
	in.ActorUserID = f.user.ID
	_, err := f.svc.CreateCampaign(context.Background(), in)
	requireKind(t, err, apperr.KindForbidden)
}

func TestCreateCampaignInvalidMoneyAndTime(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	neg := int64(-1)
	in := baseCreate(f)
	in.DisplayOriginalPriceAmountMinor = &neg
	_, err := f.svc.CreateCampaign(ctx, in)
	requireKind(t, err, apperr.KindValidation)

	in = baseCreate(f)
	high := int64(20000)
	in.DisplayCampaignPriceAmountMinor = &high
	_, err = f.svc.CreateCampaign(ctx, in)
	requireKind(t, err, apperr.KindValidation)

	in = baseCreate(f)
	end := in.StartsAt.Add(-time.Hour)
	in.EndsAt = &end
	_, err = f.svc.CreateCampaign(ctx, in)
	requireKind(t, err, apperr.KindValidation)

	in = baseCreate(f)
	in.CurrencyCode = "try"
	_, err = f.svc.CreateCampaign(ctx, in)
	requireKind(t, err, apperr.KindValidation)
}

func TestUpdateCampaignSuccessAndStale(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created, err := f.svc.CreateCampaign(ctx, baseCreate(f))
	if err != nil {
		t.Fatal(err)
	}

	name := "Güncel ad"
	updated, err := f.svc.UpdateCampaign(ctx, appcampaign.UpdateCampaignInput{
		ActorUserID: f.admin.ID, CampaignID: created.ID, ExpectedVersion: 1, Name: &name,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != name || updated.Version != 2 {
		t.Fatalf("%+v", updated)
	}

	_, err = f.svc.UpdateCampaign(ctx, appcampaign.UpdateCampaignInput{
		ActorUserID: f.admin.ID, CampaignID: created.ID, ExpectedVersion: 1, Name: &name,
	})
	requireCode(t, err, apperr.CodeStaleVersion)
}

func TestListCampaignsFiltersAndCursor(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	mk := func(code string, et domaincampaign.CampaignEventType, active bool, at time.Time) {
		c := domaincampaign.Campaign{
			ID: uuid.New(), Code: code, Name: code, EventType: et, Title: code,
			CurrencyCode: "TRY", StartsAt: at, IsActive: active, CreatedByUserID: f.admin.ID,
			Version: 1, CreatedAt: at, UpdatedAt: at,
		}
		f.store.PutCampaign(c)
	}
	t1 := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	mk("A", domaincampaign.CampaignEventTypePackageRenewal, true, t1)
	mk("B", domaincampaign.CampaignEventTypePackageRenewal, true, t2)
	mk("C", domaincampaign.CampaignEventTypePackageUpgrade, true, t2)
	mk("D", domaincampaign.CampaignEventTypePackageRenewal, false, t3)

	et := domaincampaign.CampaignEventTypePackageRenewal
	active := true
	page1, err := f.svc.ListCampaigns(ctx, f.admin.ID, appcampaign.ListCampaignsInput{
		EventType: &et, IsActive: &active, Limit: intPtr(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page1.Items) != 1 || !page1.HasMore || page1.NextCursor == nil {
		t.Fatalf("%+v", page1)
	}
	if page1.Items[0].Code != "A" {
		t.Fatalf("want A got %s", page1.Items[0].Code)
	}

	page2, err := f.svc.ListCampaigns(ctx, f.admin.ID, appcampaign.ListCampaignsInput{
		EventType: &et, IsActive: &active, Cursor: page1.NextCursor, Limit: intPtr(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Items) != 1 || page2.Items[0].Code != "B" || page2.HasMore {
		t.Fatalf("%+v", page2)
	}

	_, err = f.svc.ListCampaigns(ctx, f.admin.ID, appcampaign.ListCampaignsInput{
		Cursor: strPtr("not-a-cursor"),
	})
	requireKind(t, err, apperr.KindValidation)

	_, err = f.svc.ListCampaigns(ctx, f.user.ID, appcampaign.ListCampaignsInput{})
	requireKind(t, err, apperr.KindForbidden)
}

func TestGetCampaign(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created, err := f.svc.CreateCampaign(ctx, baseCreate(f))
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.GetCampaign(ctx, f.admin.ID, created.ID)
	if err != nil || got.ID != created.ID {
		t.Fatalf("%+v err=%v", got, err)
	}
	_, err = f.svc.GetCampaign(ctx, f.admin.ID, uuid.New())
	requireKind(t, err, apperr.KindNotFound)
}

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }

func TestUpdateCampaignNullableOmitNullValue(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created, err := f.svc.CreateCampaign(ctx, baseCreate(f))
	if err != nil {
		t.Fatal(err)
	}
	desc := "d"
	src := "STARTER"
	updated, err := f.svc.UpdateCampaign(ctx, appcampaign.UpdateCampaignInput{
		ActorUserID: f.admin.ID, CampaignID: created.ID, ExpectedVersion: created.Version,
		DescriptionSet: true, Description: &desc,
		SourcePackageCodeSet: true, SourcePackageCode: &src,
	})
	if err != nil {
		t.Fatal(err)
	}
	kept, err := f.svc.UpdateCampaign(ctx, appcampaign.UpdateCampaignInput{
		ActorUserID: f.admin.ID, CampaignID: created.ID, ExpectedVersion: updated.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	if kept.Description == nil || *kept.Description != desc || kept.SourcePackageID == nil {
		t.Fatalf("omit lost: %+v", kept)
	}
	cleared, err := f.svc.UpdateCampaign(ctx, appcampaign.UpdateCampaignInput{
		ActorUserID: f.admin.ID, CampaignID: created.ID, ExpectedVersion: kept.Version,
		DescriptionSet: true, Description: nil,
		SourcePackageCodeSet: true, SourcePackageCode: nil,
		ClearOriginalPrice: true, ClearCampaignPrice: true,
		EndsAtSet: true, EndsAt: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Description != nil || cleared.SourcePackageID != nil ||
		cleared.DisplayOriginalPriceAmountMinor != nil || cleared.DisplayCampaignPriceAmountMinor != nil {
		t.Fatalf("null clear failed: %+v", cleared)
	}
}
