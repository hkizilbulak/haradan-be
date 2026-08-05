package jobdef_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	domainjobdef "github.com/hkizilbulak/haradan-be/internal/domain/jobdef"
)

func TestValidateCronExpressionAcceptsSixField(t *testing.T) {
	t.Parallel()
	for _, expr := range []string{
		"0 10 0 * * 2,4,6",
		"0 0 9 * * *",
		"0 30 3 * * *",
	} {
		if err := domainjobdef.ValidateCronExpression(expr); err != nil {
			t.Fatalf("%q: %v", expr, err)
		}
	}
}

func TestValidateCronExpressionRejectsInvalid(t *testing.T) {
	t.Parallel()
	for _, expr := range []string{"", "  ", "* * *", "not-a-cron", "0 0 25 * * *"} {
		if err := domainjobdef.ValidateCronExpression(expr); err == nil {
			t.Fatalf("expected error for %q", expr)
		}
	}
}

func TestParseCronScheduleAcceptsSeedExpressions(t *testing.T) {
	t.Parallel()
	for _, expr := range []string{"0 10 0 * * 2,4,6", "0 0 9 * * *", "0 30 3 * * *"} {
		sched, err := domainjobdef.ParseCronSchedule(expr)
		if err != nil || sched == nil {
			t.Fatalf("%q: %v", expr, err)
		}
	}
}

func TestNextRunAtActiveOnly(t *testing.T) {
	t.Parallel()
	loc := domainjobdef.Istanbul()
	now := time.Date(2026, 8, 5, 8, 0, 0, 0, loc)
	active := domainjobdef.JobDefinition{
		IsActive: true, CronExpression: "0 0 9 * * *",
	}
	next := domainjobdef.NextRunAt(active, now)
	if next == nil {
		t.Fatal("expected next run")
	}
	want := time.Date(2026, 8, 5, 9, 0, 0, 0, loc).UTC()
	if !next.Equal(want) {
		t.Fatalf("got %v want %v", next, want)
	}

	inactive := active
	inactive.IsActive = false
	if domainjobdef.NextRunAt(inactive, now) != nil {
		t.Fatal("inactive must yield nil nextRun")
	}
}

func TestScheduledOccurrenceDedupKey(t *testing.T) {
	t.Parallel()
	loc := domainjobdef.Istanbul()
	occ := time.Date(2026, 8, 5, 0, 10, 0, 0, loc)
	got := domainjobdef.ScheduledOccurrenceDedupKey("TJK_SYNC", occ)
	want := "TJK_SYNC:" + occ.Format(time.RFC3339)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if len(got) > 255 {
		t.Fatalf("dedup key too long: %d", len(got))
	}
}

func TestManualRunDedupKey(t *testing.T) {
	t.Parallel()
	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	without := domainjobdef.ManualRunDedupKey("MEDIA_RECONCILE", nil, id)
	if without != "MEDIA_RECONCILE:MANUAL:"+id.String() {
		t.Fatalf("got %q", without)
	}
	ref := time.Date(2026, 8, 5, 15, 0, 0, 0, domainjobdef.Istanbul())
	with := domainjobdef.ManualRunDedupKey("PACKAGE_EXPIRY_SCAN", &ref, id)
	if with != "PACKAGE_EXPIRY_SCAN:MANUAL:2026-08-05" {
		t.Fatalf("got %q", with)
	}
}

func TestSanitizeLastError(t *testing.T) {
	t.Parallel()
	if domainjobdef.SanitizeLastError(nil) != nil {
		t.Fatal("nil stays nil")
	}
	empty := "  "
	if domainjobdef.SanitizeLastError(&empty) != nil {
		t.Fatal("blank becomes nil")
	}
	ok := "timeout connecting to dependency"
	if got := domainjobdef.SanitizeLastError(&ok); got == nil || *got != ok {
		t.Fatalf("got %#v", got)
	}
	secret := "failed postgres://user:password@host/db"
	if got := domainjobdef.SanitizeLastError(&secret); got == nil || *got != "İş hata ayrıntısı gizlendi." {
		t.Fatalf("got %#v", got)
	}
	long := strings.Repeat("x", 600)
	got := domainjobdef.SanitizeLastError(&long)
	if got == nil || !strings.HasSuffix(*got, "...") || len([]rune(*got)) > 503 {
		t.Fatalf("truncation failed: %#v", got)
	}
}

func TestJobTypeAndQueueMapping(t *testing.T) {
	t.Parallel()
	if !domainjobdef.JobTypeTJKSync.Valid() {
		t.Fatal("TJK_SYNC should be valid")
	}
	if domainjobdef.JobType("OTHER").Valid() {
		t.Fatal("OTHER should be invalid")
	}
	q, ok := domainjobdef.QueueJobType(domainjobdef.JobTypePackageExpiryScan)
	if !ok || q != "PACKAGE_EXPIRY_REMINDER_SCAN" {
		t.Fatalf("got %q %v", q, ok)
	}
	q, ok = domainjobdef.QueueJobType(domainjobdef.JobTypeMediaReconcile)
	if !ok || q != "MEDIA_RECONCILE" {
		t.Fatalf("got %q %v", q, ok)
	}
	q, ok = domainjobdef.QueueJobType(domainjobdef.JobTypeTJKSync)
	if !ok || q != "TJK_SYNC_BATCH" {
		t.Fatalf("got %q %v", q, ok)
	}
}
