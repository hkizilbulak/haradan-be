package notification_test

import (
	"testing"

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
	if _, ok := domainnotification.ParseTemplateEventType("  URGENT_ADVERT_ACTIVATED "); !ok {
		t.Fatal("trim parse")
	}
}

func TestNonBlankTemplateFields(t *testing.T) {
	if domainnotification.NonBlankName(" ") ||
		domainnotification.NonBlankTitleTemplate("") ||
		domainnotification.NonBlankBodyTemplate("\t") {
		t.Fatal("blank rejected")
	}
	if !domainnotification.NonBlankName("T") ||
		!domainnotification.NonBlankTitleTemplate("Başlık") ||
		!domainnotification.NonBlankBodyTemplate("Gövde") {
		t.Fatal("nonblank accepted")
	}
}
