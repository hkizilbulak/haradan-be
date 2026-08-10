package media_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

func TestAdvertEditableForMedia(t *testing.T) {
	editable := []string{"DRAFT", "CHANGES_REQUESTED"}
	closed := []string{
		"PENDING_REVIEW", "PUBLISHED", "REJECTED", "SUSPENDED", "SOLD", "ARCHIVED", "", "draft",
	}
	for _, status := range editable {
		if !domainmedia.AdvertEditableForMedia(status) {
			t.Fatalf("%s must allow media edits", status)
		}
	}
	for _, status := range closed {
		if domainmedia.AdvertEditableForMedia(status) {
			t.Fatalf("%q must not allow media edits", status)
		}
	}
}

func TestAttachableAssetLifecycles(t *testing.T) {
	attachable := domainmedia.AttachableAssetLifecycles()
	want := []domainmedia.AssetLifecycle{
		domainmedia.AssetUploadPending,
		domainmedia.AssetUploaded,
		domainmedia.AssetValidating,
		domainmedia.AssetMasterReady,
	}
	if len(attachable) != len(want) {
		t.Fatalf("attachable=%v", attachable)
	}
	for i, lifecycle := range want {
		if attachable[i] != lifecycle {
			t.Fatalf("attachable=%v", attachable)
		}
		if !domainmedia.IsAttachableAssetLifecycle(lifecycle) {
			t.Fatalf("%s must be attachable", lifecycle)
		}
	}

	rejected := []domainmedia.AssetLifecycle{
		domainmedia.AssetValidationFailed,
		domainmedia.AssetCleanupCandidate,
		domainmedia.AssetDeleting,
		domainmedia.AssetPhysicallyDeleted,
		domainmedia.AssetLifecycle("UNKNOWN"),
	}
	for _, lifecycle := range rejected {
		if domainmedia.IsAttachableAssetLifecycle(lifecycle) {
			t.Fatalf("%s must not be attachable", lifecycle)
		}
	}
}

func TestLifecycleAndJobEnumsAreClosed(t *testing.T) {
	if domainmedia.AssetLifecycle("REMOVED").Valid() {
		t.Fatal("REMOVED is a relation operation, not an asset lifecycle")
	}
	if !domainmedia.AssetMasterReady.Valid() || !domainmedia.VariantReady.Valid() {
		t.Fatal("known lifecycles must be valid")
	}
	if domainmedia.VariantLifecycle("MASTER_READY").Valid() {
		t.Fatal("asset lifecycles must not leak into variant lifecycles")
	}
	if !domainmedia.JobValidateAndNormalize.Valid() || !domainmedia.JobQueued.Valid() {
		t.Fatal("known job values must be valid")
	}
	if !domainmedia.JobNotificationFanoutAdvancedAdvert.Valid() ||
		!domainmedia.JobNotificationFanoutPackageAdvert.Valid() ||
		!domainmedia.JobEmailSendPackageExpiryReminder.Valid() {
		t.Fatal("notification job types must be valid")
	}
	if domainmedia.JobNotificationFanoutPackageAdvert.IsNotificationJob() != true {
		t.Fatal("package fanout job must be notification job")
	}
	if domainmedia.JobNotificationFanoutAdvancedAdvert.IsNotificationJob() != true {
		t.Fatal("historical advanced fanout job must remain a notification job")
	}
	if domainmedia.JobValidateAndNormalize.IsNotificationJob() {
		t.Fatal("media validate must not be notification job")
	}
	if domainmedia.JobType("TJK_SYNC_BATCH").Valid() {
		t.Fatal("TJK job type does not belong to the media domain")
	}
}

// TestObjectKeysAreSystemGenerated pins the rule that keys derive from the asset
// id, never from a user-supplied file name.
func TestObjectKeysAreSystemGenerated(t *testing.T) {
	assetID := uuid.MustParse("3f2b1c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d")
	cases := map[string]string{
		domainmedia.RawObjectKey(assetID):                                  "assets/" + assetID.String() + "/raw",
		domainmedia.MasterObjectKey(assetID):                               "assets/" + assetID.String() + "/master",
		domainmedia.VariantObjectKey(assetID, domainmedia.ProfileDetail):   "assets/" + assetID.String() + "/variants/DETAIL",
		domainmedia.VariantObjectKey(assetID, domainmedia.ProfileHomepage): "assets/" + assetID.String() + "/variants/HOMEPAGE",
		domainmedia.VariantObjectKey(assetID, domainmedia.ProfileSearch):   "assets/" + assetID.String() + "/variants/SEARCH",
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
	if domainmedia.RawObjectKey(assetID) == domainmedia.MasterObjectKey(assetID) {
		t.Fatal("raw upload and canonical master must be different objects")
	}
}

func TestRequiredTransformProfiles(t *testing.T) {
	profiles := domainmedia.RequiredTransformProfiles()
	want := []string{"DETAIL", "HOMEPAGE", "SEARCH"}
	if len(profiles) != len(want) {
		t.Fatalf("profiles=%v", profiles)
	}
	for i, name := range want {
		if profiles[i] != name {
			t.Fatalf("profiles=%v", profiles)
		}
		if !domainmedia.IsKnownTransformProfile(name) {
			t.Fatalf("%s must be a known profile", name)
		}
	}
	if domainmedia.IsKnownTransformProfile("THUMBNAIL") {
		t.Fatal("unknown profiles must be rejected")
	}
	if !domainmedia.IsKnownDeliveryProfile(domainmedia.ProfileBanner) {
		t.Fatal("BANNER must be a delivery profile")
	}
	dims, ok := domainmedia.DefaultProfileDimensions(domainmedia.ProfileHomepage)
	if !ok || dims.Width != 340 || dims.Height != 268 {
		t.Fatalf("HOMEPAGE dims=%+v", dims)
	}
	dims, ok = domainmedia.DefaultProfileDimensions(domainmedia.ProfileDetail)
	if !ok || dims.Width != 368 || dims.Height != 290 {
		t.Fatalf("DETAIL dims=%+v", dims)
	}
	dims, ok = domainmedia.DefaultProfileDimensions(domainmedia.ProfileSearch)
	if !ok || dims.Width != 100 || dims.Height != 79 {
		t.Fatalf("SEARCH dims=%+v", dims)
	}
	if domainmedia.MaxUploadBytes != 67108864 {
		t.Fatalf("MaxUploadBytes=%d", domainmedia.MaxUploadBytes)
	}
	assetID := uuid.MustParse("3f2b1c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d")
	got := domainmedia.PublicDeliveryURL(assetID, domainmedia.ProfileDetail)
	wantURL := "/v1/media/" + assetID.String() + "/DETAIL"
	if got != wantURL {
		t.Fatalf("PublicDeliveryURL=%q want %q", got, wantURL)
	}
}

// TestJobDedupKeysAreDeterministic pins the keys the partial unique index relies
// on to stop duplicate work.
func TestJobDedupKeysAreDeterministic(t *testing.T) {
	assetID := uuid.New()
	other := uuid.New()

	validate := domainmedia.ValidateJobDedupKey(assetID)
	if validate != domainmedia.ValidateJobDedupKey(assetID) {
		t.Fatal("validate dedup key must be stable")
	}
	if validate == domainmedia.ValidateJobDedupKey(other) {
		t.Fatal("validate dedup key must differ per asset")
	}
	if !strings.HasPrefix(validate, string(domainmedia.JobValidateAndNormalize)) {
		t.Fatalf("validate=%q", validate)
	}

	detail := domainmedia.VariantJobDedupKey(assetID, domainmedia.ProfileDetail)
	search := domainmedia.VariantJobDedupKey(assetID, domainmedia.ProfileSearch)
	if detail == search {
		t.Fatal("each profile needs its own dedup key")
	}
	if detail != domainmedia.VariantJobDedupKey(assetID, domainmedia.ProfileDetail) {
		t.Fatal("variant dedup key must be stable")
	}
	// varchar(255) is the column width for deduplication_key.
	if len(detail) > 255 {
		t.Fatalf("dedup key too long: %d", len(detail))
	}
}

func TestAdvertRefIsDeleted(t *testing.T) {
	if (domainmedia.AdvertRef{}).IsDeleted() {
		t.Fatal("a ref without deleted_at is not deleted")
	}
	stamp := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	if !(domainmedia.AdvertRef{DeletedAt: &stamp}).IsDeleted() {
		t.Fatal("a ref with deleted_at is deleted")
	}
}
