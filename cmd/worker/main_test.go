package main

import (
	"testing"

	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

// TestWorkerMainCompiles ensures `go test ./cmd/worker` compiles the worker binary package.
func TestWorkerMainCompiles(t *testing.T) {
	t.Parallel()
}

func TestSupportedJobTypesFollowCapabilities(t *testing.T) {
	t.Parallel()

	base := supportedJobTypes(false, false)
	for _, jobType := range []domainmedia.JobType{
		domainmedia.JobNotificationFanoutPackageAdvert,
		domainmedia.JobPackageExpiryReminderScan,
	} {
		if !hasJobType(base, jobType) {
			t.Fatalf("base worker missing %s", jobType)
		}
	}
	for _, jobType := range []domainmedia.JobType{
		domainmedia.JobValidateAndNormalize,
		domainmedia.JobGenerateVariant,
		domainmedia.JobDeleteObjects,
		domainmedia.JobReconcile,
		domainmedia.JobEmailSendAdvertNotificationChunk,
		domainmedia.JobEmailSendPackageExpiryReminder,
	} {
		if hasJobType(base, jobType) {
			t.Fatalf("worker without providers must not claim %s", jobType)
		}
	}

	mediaOnly := supportedJobTypes(true, false)
	if !hasJobType(mediaOnly, domainmedia.JobValidateAndNormalize) ||
		hasJobType(mediaOnly, domainmedia.JobEmailSendAdvertNotificationChunk) {
		t.Fatalf("unexpected media-only capabilities: %v", mediaOnly)
	}
	emailOnly := supportedJobTypes(false, true)
	if hasJobType(emailOnly, domainmedia.JobValidateAndNormalize) ||
		!hasJobType(emailOnly, domainmedia.JobEmailSendAdvertNotificationChunk) {
		t.Fatalf("unexpected email-only capabilities: %v", emailOnly)
	}
}

func hasJobType(types []domainmedia.JobType, want domainmedia.JobType) bool {
	for _, got := range types {
		if got == want {
			return true
		}
	}
	return false
}
