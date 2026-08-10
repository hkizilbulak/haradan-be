package campaign_test

import (
	"testing"
	"time"

	domaincampaign "github.com/hkizilbulak/haradan-be/internal/domain/campaign"
)

func TestCampaignEventTypeParseAndValid(t *testing.T) {
	for _, raw := range []string{
		"PACKAGE_EXPIRY_5_DAYS",
		"PACKAGE_EXPIRY_1_DAY",
		"PACKAGE_RENEWAL",
		"PACKAGE_UPGRADE",
	} {
		et, ok := domaincampaign.ParseCampaignEventType(raw)
		if !ok || !et.Valid() {
			t.Fatalf("%s should be valid", raw)
		}
	}
	if domaincampaign.CampaignEventType("PAYMENT").Valid() {
		t.Fatal("PAYMENT must be invalid")
	}
	if domaincampaign.CampaignEventType("PACKAGE_EXPIRY_10_DAYS").Valid() {
		t.Fatal("10_DAYS must be invalid")
	}
	if _, ok := domaincampaign.ParseCampaignEventType("  PACKAGE_RENEWAL "); !ok {
		t.Fatal("trim parse")
	}
}

func TestValidTimeRange(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(-time.Second)
	if domaincampaign.ValidTimeRange(start, &end) {
		t.Fatal("invalid range")
	}
	okEnd := start.Add(time.Hour)
	if !domaincampaign.ValidTimeRange(start, &okEnd) {
		t.Fatal("valid range")
	}
	if !domaincampaign.ValidTimeRange(start, nil) {
		t.Fatal("nil ends ok")
	}
}

func TestCampaignPriceLTEOriginal(t *testing.T) {
	orig := int64(100)
	camp := int64(80)
	if !domaincampaign.CampaignPriceLTEOriginal(&orig, &camp) {
		t.Fatal("80 <= 100")
	}
	camp = 120
	if domaincampaign.CampaignPriceLTEOriginal(&orig, &camp) {
		t.Fatal("120 > 100")
	}
	if !domaincampaign.CampaignPriceLTEOriginal(nil, &camp) {
		t.Fatal("nil original ok")
	}
	if !domaincampaign.CampaignPriceLTEOriginal(&orig, nil) {
		t.Fatal("nil campaign ok")
	}
}

func TestNonBlankNameTitle(t *testing.T) {
	if domaincampaign.NonBlankName("  ") || domaincampaign.NonBlankTitle("") {
		t.Fatal("blank rejected")
	}
	if !domaincampaign.NonBlankName("Kampanya") || !domaincampaign.NonBlankTitle("Başlık") {
		t.Fatal("nonblank accepted")
	}
}
