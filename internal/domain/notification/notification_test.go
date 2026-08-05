package notification_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
)

func TestTemplateEventTypeParseAndValid(t *testing.T) {
	for _, raw := range []string{
		"ADVANCED_ADVERT_PUBLISHED",
		"URGENT_ADVERT_ACTIVATED",
		"PACKAGE_EXPIRY_10_DAYS",
		"PACKAGE_EXPIRY_3_DAYS",
	} {
		et, ok := domainnotification.ParseTemplateEventType(raw)
		if !ok || !et.Valid() {
			t.Fatalf("%s should be valid", raw)
		}
	}
	if domainnotification.TemplateEventType("PACKAGE_RENEWAL").Valid() {
		t.Fatal("PACKAGE_RENEWAL is not a template event")
	}
}

func TestEventKeysAndEmailIdempotencyLength(t *testing.T) {
	t.Parallel()
	advertID := uuid.MustParse("3f2b1c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d")
	asgID := uuid.MustParse("4f2b1c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5e")
	userID := uuid.MustParse("5f2b1c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5f")
	nID := uuid.MustParse("6f2b1c4d-5e6f-4a7b-8c9d-0e1f2a3b4c50")

	advKey := domainnotification.AdvancedAdvertPublishedEventKey(advertID, asgID)
	if advKey != "ADVANCED_ADVERT_PUBLISHED:"+advertID.String()+":"+asgID.String() {
		t.Fatalf("advanced key=%q", advKey)
	}
	urgKey := domainnotification.UrgentAdvertActivatedEventKey(advertID, 3)
	if urgKey != "URGENT_ADVERT_ACTIVATED:"+advertID.String()+":3" {
		t.Fatalf("urgent key=%q", urgKey)
	}
	ends := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	expKey := domainnotification.PackageExpiryEventKey(asgID, ends, domainnotification.PackageExpiryDayOffset10D)
	if len(expKey) > 255 {
		t.Fatalf("expiry event key too long: %d", len(expKey))
	}
	idem := domainnotification.AdvertNotificationEmailIdempotencyKey(nID, userID)
	if len(idem) > 256 {
		t.Fatalf("idempotency key too long: %d", len(idem))
	}
}

func TestRenderTemplateRejectsUnknownAndHTML(t *testing.T) {
	t.Parallel()
	allow := domainnotification.AllowlistedTemplateVars(domainnotification.TemplateEventTypeAdvancedAdvertPublished)
	vars := domainnotification.TemplateVars{
		"advertId": "id", "advertTitle": "Title", "packageCode": "ADVANCED", "packageDisplayName": "Advanced",
		"isUrgent": "false", "frontendUrl": "https://example.invalid",
	}
	if _, err := domainnotification.RenderTitle(domainnotification.TemplateEventTypeAdvancedAdvertPublished, "{{.advertTitle}}", vars); err != nil {
		t.Fatal(err)
	}
	badVars := domainnotification.TemplateVars{
		"advertId": "id", "advertTitle": "<b>x</b>", "packageCode": "ADVANCED", "packageDisplayName": "P",
		"isUrgent": "false", "frontendUrl": "https://example.invalid",
	}
	if _, err := domainnotification.RenderTitle(domainnotification.TemplateEventTypeAdvancedAdvertPublished, "{{.advertTitle}}", badVars); err == nil {
		t.Fatal("html should be rejected")
	}
	extra := domainnotification.TemplateVars{
		"advertId": "id", "advertTitle": "T", "packageCode": "ADVANCED", "packageDisplayName": "P",
		"isUrgent": "false", "frontendUrl": "https://example.invalid", "evil": "x",
	}
	if _, err := domainnotification.RenderTemplate("{{.evil}}", allow, extra); err == nil {
		t.Fatal("unknown var should be rejected")
	}
	if _, err := domainnotification.RenderTemplate("{{.missing}}", allow, vars); err == nil {
		t.Fatal("missing template key should error")
	}
}

func TestAllowlistedTemplateVarsProductSet(t *testing.T) {
	t.Parallel()
	advertKeys := []string{"advertId", "advertTitle", "packageCode", "packageDisplayName", "isUrgent", "frontendUrl"}
	for _, et := range []domainnotification.TemplateEventType{
		domainnotification.TemplateEventTypeAdvancedAdvertPublished,
		domainnotification.TemplateEventTypeUrgentAdvertActivated,
	} {
		allow := domainnotification.AllowlistedTemplateVars(et)
		if len(allow) != len(advertKeys) {
			t.Fatalf("%s: got %d keys, want %d", et, len(allow), len(advertKeys))
		}
		for _, k := range advertKeys {
			if _, ok := allow[k]; !ok {
				t.Fatalf("%s: missing key %q", et, k)
			}
		}
	}

	expiryKeys := []string{
		"advertId", "advertTitle", "packageCode", "packageDisplayName", "endsAt", "daysRemaining",
		"campaignTitle", "campaignDescription", "campaignCtaLabel", "campaignCtaUrl", "frontendUrl",
	}
	for _, et := range []domainnotification.TemplateEventType{
		domainnotification.TemplateEventTypePackageExpiry10Days,
		domainnotification.TemplateEventTypePackageExpiry3Days,
	} {
		allow := domainnotification.AllowlistedTemplateVars(et)
		if len(allow) != len(expiryKeys) {
			t.Fatalf("%s: got %d keys, want %d", et, len(allow), len(expiryKeys))
		}
		for _, k := range expiryKeys {
			if _, ok := allow[k]; !ok {
				t.Fatalf("%s: missing key %q", et, k)
			}
		}
	}
}

func TestPackageExpiryIstanbulCalendarDays(t *testing.T) {
	t.Parallel()
	loc, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		t.Fatal(err)
	}
	endsAt := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	if got := domainnotification.CalendarDaysUntil(endsAt, now, loc); got != 10 {
		t.Fatalf("calendar days=%d want 10", got)
	}
}

func TestNonBlankTemplateFields(t *testing.T) {
	if domainnotification.NonBlankName(" ") ||
		domainnotification.NonBlankTitleTemplate("") ||
		domainnotification.NonBlankBodyTemplate("\t") {
		t.Fatal("blank rejected")
	}
}
