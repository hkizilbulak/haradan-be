package banner_test

import (
	"testing"

	"github.com/google/uuid"

	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

func TestBannerReadyVariantUsesBANNERProfileKey(t *testing.T) {
	assetID := uuid.New()
	bannerKey := domainmedia.VariantObjectKey(assetID, domainmedia.ProfileBanner)
	homepageKey := domainmedia.VariantObjectKey(assetID, domainmedia.ProfileHomepage)

	if !domainmedia.IsOwnedVariantObjectKey(assetID, domainmedia.ProfileBanner, bannerKey) {
		t.Fatal("BANNER variant key must be accepted")
	}
	if domainmedia.IsOwnedVariantObjectKey(assetID, domainmedia.ProfileBanner, homepageKey) {
		t.Fatal("placement HOMEPAGE key must not satisfy BANNER delivery")
	}
	if domainmedia.IsOwnedVariantObjectKey(assetID, domainmedia.ProfileBanner, domainmedia.RawObjectKey(assetID)) {
		t.Fatal("raw key must be denied")
	}
	url := domainmedia.PublicDeliveryURL(assetID, domainmedia.ProfileBanner)
	want := "/v1/media/" + assetID.String() + "/BANNER"
	if url != want {
		t.Fatalf("PublicDeliveryURL=%q want %q", url, want)
	}
}
