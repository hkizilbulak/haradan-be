package packaging_test

import (
	"testing"
	"time"

	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
)

func TestPackageCodeAndUrgentHelpers(t *testing.T) {
	if !domainpackaging.PackageCode("ADVANCED").Valid() {
		t.Fatal("ADVANCED remains a valid dynamic code")
	}
	if !domainpackaging.PackageCode("CUSTOM_PRO").Valid() {
		t.Fatal("dynamic codes must be valid")
	}
	if domainpackaging.PackageCode("bad code").Valid() {
		t.Fatal("invalid format must be rejected")
	}
	p := domainpackaging.Package{Code: domainpackaging.PackageCode("ADVANCED"), AllowsUrgent: true}
	if !p.AllowsUrgentFeature() {
		t.Fatal("expected allows urgent feature")
	}
	p.AllowsUrgent = false
	if p.AllowsUrgentFeature() {
		t.Fatal("requires allows_urgent")
	}
	p.BroadcastOnPublish = true
	if !p.EmitsPublishBroadcast() {
		t.Fatal("expected broadcast on publish")
	}
	p.BroadcastOnPublish = false
	if p.EmitsPublishBroadcast() {
		t.Fatal("broadcast requires flag")
	}
}

func TestAssignmentEffectiveWindow(t *testing.T) {
	now := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	end := now.Add(24 * time.Hour)
	a := domainpackaging.AdvertPackageAssignment{
		Status:   domainpackaging.AssignmentStatusActive,
		StartsAt: now.Add(-time.Hour),
		EndsAt:   &end,
	}
	if !a.IsEffectiveAt(now) {
		t.Fatal("should be effective")
	}
	if a.IsEffectiveAt(end) {
		t.Fatal("ends_at exclusive")
	}
	a.Status = domainpackaging.AssignmentStatusSuperseded
	if a.IsEffectiveAt(now) {
		t.Fatal("superseded not effective")
	}
}

func TestValidTimeRangeAndActivationVersion(t *testing.T) {
	start := time.Now().UTC()
	end := start.Add(-time.Second)
	if domainpackaging.ValidTimeRange(start, &end) {
		t.Fatal("invalid range")
	}
	if !domainpackaging.ValidActivationVersion(1) || domainpackaging.ValidActivationVersion(0) {
		t.Fatal("activation version")
	}
	if !domainpackaging.FeatureCode("FEATURED").Valid() {
		t.Fatal("FEATURED must be valid")
	}
	if domainpackaging.FeatureCode("BOOST").Valid() {
		t.Fatal("unknown feature codes rejected")
	}
}

func TestAdvertStatusAllowsUrgent(t *testing.T) {
	if !domainpackaging.AdvertStatusAllowsUrgent("DRAFT") ||
		!domainpackaging.AdvertStatusAllowsUrgent("PENDING_REVIEW") ||
		!domainpackaging.AdvertStatusAllowsUrgent("PUBLISHED") {
		t.Fatal("expected draft/pending/published allowed")
	}
	for _, st := range []string{"SOLD", "ARCHIVED", "SUSPENDED", "REJECTED", "CHANGES_REQUESTED"} {
		if domainpackaging.AdvertStatusAllowsUrgent(st) {
			t.Fatalf("%s must reject urgent", st)
		}
	}
}
