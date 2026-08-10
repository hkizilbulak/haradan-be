package jobadmin_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	appjobadmin "github.com/hkizilbulak/haradan-be/internal/application/jobadmin"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainjobdef "github.com/hkizilbulak/haradan-be/internal/domain/jobdef"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func seedAdmin(store *appjobadmin.MemoryStore) uuid.UUID {
	id := uuid.New()
	store.SeedUser(domainuser.User{
		ID: id, Role: domainuser.RoleAdmin, Status: domainuser.StatusActive,
	})
	return id
}

func seedDef(store *appjobadmin.MemoryStore, mut func(*domainjobdef.JobDefinition)) domainjobdef.JobDefinition {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	def := domainjobdef.JobDefinition{
		ID:                    uuid.New(),
		JobKey:                "PACKAGE_EXPIRY_SCAN",
		Name:                  "Package expiry",
		JobType:               domainjobdef.JobTypePackageExpiryScan,
		CronExpression:        "0 0 9 * * *",
		IsActive:              true,
		TimeoutSeconds:        1800,
		DefaultPayload:        []byte(`{}`),
		SupportsReferenceDate: true,
		Version:               1,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if mut != nil {
		mut(&def)
	}
	store.SeedDefinition(def)
	return def
}

func newSvc(t *testing.T, store *appjobadmin.MemoryStore, caps appjobadmin.ProviderCapabilities) *appjobadmin.Service {
	t.Helper()
	svc, err := appjobadmin.NewService(appjobadmin.Config{
		Repo: store, Users: store, Caps: caps,
		Clock: fixedClock{t: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)},
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func requireCode(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	ae, ok := apperr.As(err)
	if !ok || ae.Code != code {
		t.Fatalf("got %#v want %s", err, code)
	}
}

func TestListAndGetRequireAdmin(t *testing.T) {
	store := appjobadmin.NewMemoryStore()
	def := seedDef(store, nil)
	userID := uuid.New()
	store.SeedUser(domainuser.User{ID: userID, Role: domainuser.RoleUser, Status: domainuser.StatusActive})
	svc := newSvc(t, store, appjobadmin.ProviderCapabilities{B2Enabled: true, TinifyEnabled: true, TJKEnabled: true})

	_, err := svc.ListJobs(context.Background(), userID)
	requireCode(t, err, apperr.CodeForbidden)

	admin := seedAdmin(store)
	list, err := svc.ListJobs(context.Background(), admin)
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}
	got, err := svc.GetJob(context.Background(), admin, def.ID)
	if err != nil || got.JobKey != def.JobKey {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestListJobsEnrichesLastRunAndNextRun(t *testing.T) {
	store := appjobadmin.NewMemoryStore()
	def := seedDef(store, nil)
	inactive := seedDef(store, func(d *domainjobdef.JobDefinition) {
		d.ID = uuid.New()
		d.JobKey = "MEDIA_RECONCILE"
		d.JobType = domainjobdef.JobTypeMediaReconcile
		d.IsActive = false
		d.SupportsReferenceDate = false
	})
	admin := seedAdmin(store)
	started := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	completed := started.Add(2 * time.Second)
	defID := def.ID
	store.SeedExecution(domainjobdef.JobExecution{
		ID: uuid.New(), JobDefinitionID: &defID, Status: "SUCCEEDED",
		StartedAt: &started, CompletedAt: &completed,
		CreatedAt: started, UpdatedAt: completed,
	})

	svc := newSvc(t, store, appjobadmin.ProviderCapabilities{B2Enabled: true})
	list, err := svc.ListJobs(context.Background(), admin)
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]domainjobdef.JobDefinition{}
	for _, item := range list {
		byKey[item.JobKey] = item
	}
	active := byKey[def.JobKey]
	if active.LastRunAt == nil || !active.LastRunAt.Equal(started) {
		t.Fatalf("lastRunAt=%v", active.LastRunAt)
	}
	if active.LastStatus == nil || *active.LastStatus != "SUCCEEDED" {
		t.Fatalf("lastStatus=%v", active.LastStatus)
	}
	if active.LastDurationMs == nil || *active.LastDurationMs != 2000 {
		t.Fatalf("lastDurationMs=%v", active.LastDurationMs)
	}
	if active.NextRunAt == nil {
		t.Fatal("expected nextRunAt for active job")
	}
	// Clock is 2026-08-05 12:00 UTC = 15:00 Istanbul → next daily 09:00 Istanbul is Aug 6.
	wantNext := time.Date(2026, 8, 6, 9, 0, 0, 0, domainjobdef.Istanbul()).UTC()
	if !active.NextRunAt.Equal(wantNext) {
		t.Fatalf("nextRunAt=%v want %v", active.NextRunAt, wantNext)
	}

	off := byKey[inactive.JobKey]
	if off.NextRunAt != nil || off.LastRunAt != nil {
		t.Fatalf("inactive never-run should have null last/next: %#v", off)
	}
}

func TestUpdateOptimisticConflictAndCronValidation(t *testing.T) {
	store := appjobadmin.NewMemoryStore()
	def := seedDef(store, nil)
	admin := seedAdmin(store)
	svc := newSvc(t, store, appjobadmin.ProviderCapabilities{})

	bad := "not-cron"
	_, err := svc.UpdateJob(context.Background(), appjobadmin.UpdateJobInput{
		ActorUserID: admin, JobID: def.ID, ExpectedVersion: 1, CronExpression: &bad,
	})
	requireCode(t, err, apperr.CodeValidation)

	active := false
	cron := "0 15 9 * * *"
	timeout := 900
	updated, err := svc.UpdateJob(context.Background(), appjobadmin.UpdateJobInput{
		ActorUserID: admin, JobID: def.ID, ExpectedVersion: 1,
		CronExpression: &cron, IsActive: &active, TimeoutSeconds: &timeout,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != 2 || updated.IsActive || updated.TimeoutSeconds != 900 || updated.CronExpression != cron {
		t.Fatalf("%#v", updated)
	}
	if updated.JobKey != "PACKAGE_EXPIRY_SCAN" || updated.JobType != domainjobdef.JobTypePackageExpiryScan {
		t.Fatal("key/type must be immutable")
	}

	_, err = svc.UpdateJob(context.Background(), appjobadmin.UpdateJobInput{
		ActorUserID: admin, JobID: def.ID, ExpectedVersion: 1, IsActive: &active,
	})
	requireCode(t, err, apperr.CodeStaleVersion)
}

func TestActiveToggle(t *testing.T) {
	store := appjobadmin.NewMemoryStore()
	def := seedDef(store, nil)
	admin := seedAdmin(store)
	svc := newSvc(t, store, appjobadmin.ProviderCapabilities{})
	on := true
	off := false
	updated, err := svc.UpdateJob(context.Background(), appjobadmin.UpdateJobInput{
		ActorUserID: admin, JobID: def.ID, ExpectedVersion: 1, IsActive: &off,
	})
	if err != nil || updated.IsActive {
		t.Fatalf("%#v %v", updated, err)
	}
	updated, err = svc.UpdateJob(context.Background(), appjobadmin.UpdateJobInput{
		ActorUserID: admin, JobID: def.ID, ExpectedVersion: 2, IsActive: &on,
	})
	if err != nil || !updated.IsActive {
		t.Fatalf("%#v %v", updated, err)
	}
}

func TestManualRunAndReferenceDateValidation(t *testing.T) {
	store := appjobadmin.NewMemoryStore()
	def := seedDef(store, nil)
	admin := seedAdmin(store)
	svc := newSvc(t, store, appjobadmin.ProviderCapabilities{B2Enabled: true, TinifyEnabled: true, TJKEnabled: true})

	ref := "2026-08-01"
	out, err := svc.RunJob(context.Background(), appjobadmin.RunJobInput{
		ActorUserID: admin, JobID: def.ID, ReferenceDate: &ref,
	})
	if err != nil || out.BackgroundJobID == uuid.Nil {
		t.Fatalf("%#v %v", out, err)
	}
	keys := store.DedupKeys()
	if len(keys) != 1 || keys[0] != "PACKAGE_EXPIRY_SCAN:MANUAL:2026-08-01" {
		t.Fatalf("dedup=%v", keys)
	}

	// same reference date is dedup-safe
	out2, err := svc.RunJob(context.Background(), appjobadmin.RunJobInput{
		ActorUserID: admin, JobID: def.ID, ReferenceDate: &ref,
	})
	if err != nil || !out2.AlreadyExists {
		t.Fatalf("%#v %v", out2, err)
	}

	future := "2026-08-06"
	_, err = svc.RunJob(context.Background(), appjobadmin.RunJobInput{
		ActorUserID: admin, JobID: def.ID, ReferenceDate: &future,
	})
	requireCode(t, err, apperr.CodeValidation)

	media := seedDef(store, func(d *domainjobdef.JobDefinition) {
		d.ID = uuid.New()
		d.JobKey = "MEDIA_RECONCILE"
		d.JobType = domainjobdef.JobTypeMediaReconcile
		d.SupportsReferenceDate = false
	})
	_, err = svc.RunJob(context.Background(), appjobadmin.RunJobInput{
		ActorUserID: admin, JobID: media.ID, ReferenceDate: &ref,
	})
	requireCode(t, err, apperr.CodeValidation)
}

func TestRunJobProviderDisabled(t *testing.T) {
	store := appjobadmin.NewMemoryStore()
	def := seedDef(store, func(d *domainjobdef.JobDefinition) {
		d.JobKey = "TJK_SYNC"
		d.JobType = domainjobdef.JobTypeTJKSync
		d.SupportsReferenceDate = false
	})
	admin := seedAdmin(store)
	svc := newSvc(t, store, appjobadmin.ProviderCapabilities{TJKEnabled: false, B2Enabled: true})
	_, err := svc.RunJob(context.Background(), appjobadmin.RunJobInput{ActorUserID: admin, JobID: def.ID})
	requireCode(t, err, apperr.CodeInvalidState)
}

func TestHistorySanitizationAndPagination(t *testing.T) {
	store := appjobadmin.NewMemoryStore()
	def := seedDef(store, nil)
	admin := seedAdmin(store)
	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		ts := now.Add(-time.Duration(i) * time.Minute)
		if _, err := store.Enqueue(context.Background(), appjobadmin.EnqueueRequest{
			Definition: def, ExecutionType: domainjobdef.ExecutionTypeManual,
			DeduplicationKey: "hist-" + uuid.NewString(), AvailableAt: ts, Now: ts,
		}); err != nil {
			t.Fatal(err)
		}
	}
	svc := newSvc(t, store, appjobadmin.ProviderCapabilities{B2Enabled: true})
	page, err := svc.ListHistory(context.Background(), admin, def.ID, nil, intPtr(2))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || !page.HasMore || page.NextCursor == nil {
		t.Fatalf("%#v", page)
	}
	page2, err := svc.ListHistory(context.Background(), admin, def.ID, page.NextCursor, intPtr(2))
	if err != nil || len(page2.Items) != 1 || page2.HasMore {
		t.Fatalf("%#v %v", page2, err)
	}

	secret := "boom postgres://user:password@db/hrd"
	secretStore := &historySecretRepo{inner: store, secret: secret}
	svc3, err := appjobadmin.NewService(appjobadmin.Config{
		Repo: secretStore, Users: store,
		Clock: fixedClock{t: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)},
	})
	if err != nil {
		t.Fatal(err)
	}
	sanitized, err := svc3.ListHistory(context.Background(), admin, def.ID, nil, intPtr(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(sanitized.Items) == 0 || sanitized.Items[0].LastError == nil {
		t.Fatal("expected last error")
	}
	if strings.Contains(strings.ToLower(*sanitized.Items[0].LastError), "postgres") {
		t.Fatalf("secret leaked: %s", *sanitized.Items[0].LastError)
	}
}

type historySecretRepo struct {
	inner  *appjobadmin.MemoryStore
	secret string
}

func (r *historySecretRepo) ListDefinitions(ctx context.Context) ([]domainjobdef.JobDefinition, error) {
	return r.inner.ListDefinitions(ctx)
}
func (r *historySecretRepo) GetDefinition(ctx context.Context, id uuid.UUID) (domainjobdef.JobDefinition, error) {
	return r.inner.GetDefinition(ctx, id)
}
func (r *historySecretRepo) UpdateDefinitionOptimistic(ctx context.Context, def domainjobdef.JobDefinition, v int) (domainjobdef.JobDefinition, error) {
	return r.inner.UpdateDefinitionOptimistic(ctx, def, v)
}
func (r *historySecretRepo) Enqueue(ctx context.Context, req appjobadmin.EnqueueRequest) (appjobadmin.EnqueueResult, error) {
	return r.inner.Enqueue(ctx, req)
}
func (r *historySecretRepo) ListHistory(ctx context.Context, definitionID uuid.UUID, f appjobadmin.HistoryFilter) ([]domainjobdef.JobExecution, error) {
	rows, err := r.inner.ListHistory(ctx, definitionID, f)
	if err != nil {
		return nil, err
	}
	sec := r.secret
	if len(rows) > 0 {
		rows[0].LastError = &sec
	}
	return rows, nil
}
func (r *historySecretRepo) ListLastRuns(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domainjobdef.LastRunSummary, error) {
	return r.inner.ListLastRuns(ctx, ids)
}

func intPtr(v int) *int { return &v }
