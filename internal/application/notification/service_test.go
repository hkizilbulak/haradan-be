package notification_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	appnotification "github.com/hkizilbulak/haradan-be/internal/application/notification"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fixture struct {
	store *appnotification.MemoryStore
	svc   *appnotification.Service
	admin domainuser.User
	user  domainuser.User
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	store := appnotification.NewMemoryStore()
	svc, err := appnotification.NewMemoryService(store, fixedClock{t: now})
	if err != nil {
		t.Fatalf("NewMemoryService: %v", err)
	}

	admin := domainuser.User{ID: uuid.New(), Role: domainuser.RoleAdmin, Status: domainuser.StatusActive}
	user := domainuser.User{ID: uuid.New(), Role: domainuser.RoleUser, Status: domainuser.StatusActive}
	store.PutUser(admin)
	store.PutUser(user)

	seed := []domainnotification.NotificationTemplate{
		{
			ID:                 uuid.MustParse("b0000000-0000-4000-8000-000000000001"),
			EventType:          domainnotification.TemplateEventTypePackageAdvertPublished,
			Name:               "Package advert published placeholder",
			InAppTitleTemplate: "Yeni ilan", InAppBodyTemplate: "Yeni bir ilan yayınlandı.",
			Version: 1, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID:                 uuid.MustParse("b0000000-0000-4000-8000-000000000002"),
			EventType:          domainnotification.TemplateEventTypeUrgentAdvertActivated,
			Name:               "Urgent advert activated placeholder",
			InAppTitleTemplate: "Acil ilan", InAppBodyTemplate: "Bir ilan acil olarak işaretlendi.",
			Version: 1, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID:                 uuid.MustParse("b0000000-0000-4000-8000-000000000003"),
			EventType:          domainnotification.TemplateEventTypePackageExpiry5Days,
			Name:               "Package expiry 5 days placeholder",
			InAppTitleTemplate: "Paket süresi", InAppBodyTemplate: "Paket süreniz yakında dolacak.",
			Version: 1, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID:                 uuid.MustParse("b0000000-0000-4000-8000-000000000004"),
			EventType:          domainnotification.TemplateEventTypePackageExpiry1Day,
			Name:               "Package expiry 1 day placeholder",
			InAppTitleTemplate: "Paket süresi", InAppBodyTemplate: "Paket süreniz yakında dolacak.",
			Version: 1, CreatedAt: now, UpdatedAt: now,
		},
	}
	for _, tmpl := range seed {
		store.PutTemplate(tmpl)
	}

	return &fixture{store: store, svc: svc, admin: admin, user: user}
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

func TestListAndGetTemplates(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	items, err := f.svc.ListTemplates(ctx, f.admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("len=%d", len(items))
	}
	if items[0].EventType != domainnotification.TemplateEventTypePackageAdvertPublished {
		t.Fatalf("order: %s", items[0].EventType)
	}

	got, err := f.svc.GetTemplateByEventType(ctx, f.admin.ID, domainnotification.TemplateEventTypeUrgentAdvertActivated)
	if err != nil || got.Name == "" {
		t.Fatalf("%+v err=%v", got, err)
	}

	_, err = f.svc.ListTemplates(ctx, f.user.ID)
	requireKind(t, err, apperr.KindForbidden)
}

func TestUpdateTemplateSuccessStaleAndBlank(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	et := domainnotification.TemplateEventTypePackageExpiry5Days

	resend := "tmpl_abc"
	active := true
	updated, err := f.svc.UpdateTemplate(ctx, appnotification.UpdateTemplateInput{
		ActorUserID: f.admin.ID, EventType: et, ExpectedVersion: 1,
		Name: strPtr("Expiry 5d"), InAppTitleTemplate: strPtr("Süre doluyor"), InAppBodyTemplate: strPtr("5 gün kaldı."),
		ResendTemplateIDSet: true, ResendTemplateID: &resend, IsActive: &active,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != 2 || !updated.IsActive || updated.ResendTemplateID == nil {
		t.Fatalf("%+v", updated)
	}
	if updated.UpdatedByUserID == nil || *updated.UpdatedByUserID != f.admin.ID {
		t.Fatal("updated_by")
	}

	inactive := false
	_, err = f.svc.UpdateTemplate(ctx, appnotification.UpdateTemplateInput{
		ActorUserID: f.admin.ID, EventType: et, ExpectedVersion: 1,
		Name: strPtr("x"), InAppTitleTemplate: strPtr("y"), InAppBodyTemplate: strPtr("z"), IsActive: &inactive,
	})
	requireCode(t, err, apperr.CodeStaleVersion)

	_, err = f.svc.UpdateTemplate(ctx, appnotification.UpdateTemplateInput{
		ActorUserID: f.admin.ID, EventType: et, ExpectedVersion: 2,
		Name: strPtr(" "), InAppTitleTemplate: strPtr("y"), InAppBodyTemplate: strPtr("z"), IsActive: &inactive,
	})
	requireKind(t, err, apperr.KindValidation)

	_, err = f.svc.UpdateTemplate(ctx, appnotification.UpdateTemplateInput{
		ActorUserID: f.admin.ID, EventType: et, ExpectedVersion: 2,
		Name: strPtr("ok"), InAppTitleTemplate: strPtr(""), InAppBodyTemplate: strPtr("z"), IsActive: &inactive,
	})
	requireKind(t, err, apperr.KindValidation)

	_, err = f.svc.UpdateTemplate(ctx, appnotification.UpdateTemplateInput{
		ActorUserID: f.user.ID, EventType: et, ExpectedVersion: 2,
		Name: strPtr("ok"), InAppTitleTemplate: strPtr("t"), InAppBodyTemplate: strPtr("b"), IsActive: &inactive,
	})
	requireKind(t, err, apperr.KindForbidden)

	// null clear resend / email fallback
	_, err = f.svc.UpdateTemplate(ctx, appnotification.UpdateTemplateInput{
		ActorUserID: f.admin.ID, EventType: et, ExpectedVersion: 2,
		ResendTemplateIDSet: true, ResendTemplateID: nil,
		EmailSubjectFallbackSet: true, EmailSubjectFallback: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	cleared, err := f.svc.GetTemplateByEventType(ctx, f.admin.ID, et)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.ResendTemplateID != nil || cleared.EmailSubjectFallback != nil {
		t.Fatalf("expected cleared nullable fields: %+v", cleared)
	}
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
