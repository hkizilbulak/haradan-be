package packaging_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	apppackaging "github.com/hkizilbulak/haradan-be/internal/application/packaging"
	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	pgadvert "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/advert"
	pgpackaging "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/packaging"
	pguser "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/user"
)

// Former deterministic seed IDs from migration 00009 (removed by 00011 when untouched).
var (
	legacySeedStarter  = uuid.MustParse("a0000000-0000-4000-8000-000000000001")
	legacySeedMiddle   = uuid.MustParse("a0000000-0000-4000-8000-000000000002")
	legacySeedAdvanced = uuid.MustParse("a0000000-0000-4000-8000-000000000003")
)

var (
	codeStarter  = domainpackaging.PackageCode("STARTER")
	codeMiddle   = domainpackaging.PackageCode("MIDDLE")
	codeAdvanced = domainpackaging.PackageCode("ADVANCED")
)

func requirePackagingIntegration(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if strings.TrimSpace(os.Getenv("RUN_PACKAGING_INTEGRATION_TESTS")) != "1" {
		t.Skip("RUN_PACKAGING_INTEGRATION_TESTS!=1; skipping packaging integration tests")
	}
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping packaging integration tests")
	}
	if strings.TrimSpace(os.Getenv("DATABASE_URL")) != "" && dsn == strings.TrimSpace(os.Getenv("DATABASE_URL")) {
		t.Fatalf("TEST_DATABASE_URL must not equal DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

type itestClock struct{ t time.Time }

func (c itestClock) Now() time.Time { return c.t }

type itestFixture struct {
	pool     *pgxpool.Pool
	svc      *apppackaging.Service
	clock    *itestClock
	adminID  uuid.UUID
	ownerID  uuid.UUID
	otherID  uuid.UUID
	advert   domainadvert.Advert
	starter  domainpackaging.Package
	middle   domainpackaging.Package
	advanced domainpackaging.Package
}

func newItestFixture(t *testing.T, pool *pgxpool.Pool) *itestFixture {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	users := pguser.NewRepository(pool)
	adverts := pgadvert.NewRepository(pool)

	mkUser := func(role domainuser.Role, emailPrefix string) uuid.UUID {
		email := emailPrefix + uuid.NewString() + "@example.com"
		u := domainuser.User{
			ID: uuid.New(), Email: email, EmailNormalized: strings.ToLower(email),
			PasswordHash: "hash", Role: role, Status: domainuser.StatusActive,
			FirstName: "T", LastName: "U", SecurityStamp: uuid.New(),
			CreatedAt: now, UpdatedAt: now,
		}
		if err := users.Create(ctx, u); err != nil {
			t.Fatalf("create user: %v", err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM hrd_users WHERE id = $1`, u.ID)
		})
		return u.ID
	}

	adminID := mkUser(domainuser.RoleAdmin, "pkg-admin-")
	ownerID := mkUser(domainuser.RoleUser, "pkg-owner-")
	otherID := mkUser(domainuser.RoleUser, "pkg-other-")

	categoryID := uuid.New()
	provinceID := uuid.New()
	districtID := uuid.New()
	if _, err := pool.Exec(ctx, `
INSERT INTO hrd_categories (id, parent_id, slug, name, description, is_active, sort_order, version, created_at, updated_at)
VALUES ($1, NULL, $2, 'Pkg Leaf', NULL, true, 1, 1, $3, $3)`,
		categoryID, "pkg-leaf-"+categoryID.String()[:8], now,
	); err != nil {
		t.Fatalf("insert category: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM hrd_categories WHERE id = $1`, categoryID)
	})
	if _, err := pool.Exec(ctx, `
INSERT INTO hrd_provinces (id, name, name_normalized, is_active, sort_order, created_at, updated_at)
VALUES ($1, $2, $3, true, 1, $4, $4)`,
		provinceID, "PkgProv "+provinceID.String()[:8], "pkgprov-"+provinceID.String()[:8], now,
	); err != nil {
		t.Fatalf("insert province: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM hrd_provinces WHERE id = $1`, provinceID)
	})
	if _, err := pool.Exec(ctx, `
INSERT INTO hrd_districts (id, province_id, name, name_normalized, is_active, sort_order, created_at, updated_at)
VALUES ($1, $2, $3, $4, true, 1, $5, $5)`,
		districtID, provinceID, "PkgDist "+districtID.String()[:8], "pkgdist-"+districtID.String()[:8], now,
	); err != nil {
		t.Fatalf("insert district: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM hrd_districts WHERE id = $1`, districtID)
	})

	title := "Paket test ilanı"
	desc := "Paket test açıklaması"
	advert := domainadvert.Advert{
		ID: uuid.New(), OwnerUserID: ownerID, Status: domainadvert.StatusPublished,
		CategoryID: &categoryID, DistrictID: &districtID, Title: &title, Description: &desc,
		Properties: domainadvert.EmptyProperties(), Version: 1, MediaVersion: 1,
		PublishedAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	if err := adverts.Create(ctx, advert); err != nil {
		t.Fatalf("create advert: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM hrd_advert_feature_activations WHERE advert_id = $1`, advert.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM hrd_advert_package_assignments WHERE advert_id = $1`, advert.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM hrd_advert_status_history WHERE advert_id = $1`, advert.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM hrd_adverts WHERE id = $1`, advert.ID)
	})

	clock := &itestClock{t: now}
	svc, err := apppackaging.NewPostgresService(pool, adverts, users, clock)
	if err != nil {
		t.Fatal(err)
	}
	f := &itestFixture{pool: pool, svc: svc, clock: clock, adminID: adminID, ownerID: ownerID, otherID: otherID, advert: advert}
	f.ensureCatalogPackages(t)
	return f
}

// ensureCatalogPackages creates STARTER/MIDDLE/ADVANCED via CreatePackage when missing.
func (f *itestFixture) ensureCatalogPackages(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	dur := 30

	type def struct {
		code               domainpackaging.PackageCode
		name               string
		sortOrder          int
		allowsUrgent       bool
		showcaseEligible   bool
		searchPriority     int
		broadcastOnPublish bool
		duration           *int
		dest               *domainpackaging.Package
	}
	defs := []def{
		{code: codeStarter, name: "Starter", sortOrder: 10, dest: &f.starter},
		{
			code: codeMiddle, name: "Middle", sortOrder: 20,
			showcaseEligible: true, dest: &f.middle,
		},
		{
			code: codeAdvanced, name: "Advanced", sortOrder: 30,
			allowsUrgent: true, showcaseEligible: true, searchPriority: 100,
			broadcastOnPublish: true, duration: &dur, dest: &f.advanced,
		},
	}

	for _, d := range defs {
		existing, err := f.svc.GetPackageByCode(ctx, f.adminID, d.code)
		if err == nil {
			*d.dest = f.alignPackageFlags(t, existing, d.allowsUrgent, d.showcaseEligible, d.searchPriority, d.broadcastOnPublish, d.sortOrder)
			continue
		}
		if !asNotFound(err) {
			t.Fatalf("GetPackageByCode(%s): %v", d.code, err)
		}
		created, err := f.svc.CreatePackage(ctx, apppackaging.CreatePackageInput{
			ActorUserID: f.adminID, Code: d.code, DisplayName: d.name,
			CurrencyCode: "TRY", DefaultDurationDays: d.duration,
			AllowsUrgent: d.allowsUrgent, ShowcaseEligible: d.showcaseEligible,
			SearchPriority: d.searchPriority, SearchPrioritySet: true, BroadcastOnPublish: d.broadcastOnPublish,
			IsActive: true, SortOrder: intPtr(d.sortOrder),
		})
		if err != nil {
			t.Fatalf("CreatePackage(%s): %v", d.code, err)
		}
		*d.dest = created
		pkgID := created.ID
		t.Cleanup(func() {
			_, _ = f.pool.Exec(context.Background(), `
DELETE FROM hrd_packages WHERE id = $1
  AND NOT EXISTS (SELECT 1 FROM hrd_advert_package_assignments a WHERE a.package_id = $1)
  AND NOT EXISTS (
    SELECT 1 FROM hrd_campaigns c
    WHERE c.source_package_id = $1 OR c.target_package_id = $1
  )`, pkgID)
		})
	}
}

func (f *itestFixture) alignPackageFlags(
	t *testing.T,
	pkg domainpackaging.Package,
	allowsUrgent, showcaseEligible bool,
	searchPriority int,
	broadcastOnPublish bool,
	sortOrder int,
) domainpackaging.Package {
	t.Helper()
	if pkg.AllowsUrgent == allowsUrgent &&
		pkg.ShowcaseEligible == showcaseEligible &&
		pkg.SearchPriority == searchPriority &&
		pkg.BroadcastOnPublish == broadcastOnPublish &&
		pkg.SortOrder == sortOrder &&
		pkg.IsActive {
		return pkg
	}
	ctx := context.Background()
	name := pkg.DisplayName
	active := true
	updated, err := f.svc.UpdatePackage(ctx, apppackaging.UpdatePackageInput{
		ActorUserID: f.adminID, Code: pkg.Code, ExpectedVersion: pkg.Version,
		DisplayName: &name, AllowsUrgent: &allowsUrgent, ShowcaseEligible: &showcaseEligible,
		SearchPriority: &searchPriority, BroadcastOnPublish: &broadcastOnPublish,
		IsActive: &active, SortOrder: &sortOrder,
	})
	if err != nil {
		t.Fatalf("align flags for %s: %v", pkg.Code, err)
	}
	return updated
}

func TestEmptyPackageCatalogAfterMigrations(t *testing.T) {
	pool := requirePackagingIntegration(t)
	ctx := context.Background()
	repo := pgpackaging.NewRepository(pool)

	var untouched int
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM hrd_packages
WHERE id = ANY($1::uuid[])
  AND version = 1
  AND created_at = TIMESTAMPTZ '2020-01-01 00:00:00+00'
  AND updated_at = TIMESTAMPTZ '2020-01-01 00:00:00+00'
  AND description IS NULL
  AND badge_text IS NULL
  AND benefits = '[]'::jsonb
  AND display_price_amount_minor IS NULL
  AND currency_code = 'TRY'
  AND default_duration_days IS NULL
  AND broadcast_on_publish = false`,
		[]uuid.UUID{legacySeedStarter, legacySeedMiddle, legacySeedAdvanced},
	).Scan(&untouched); err != nil {
		t.Fatal(err)
	}
	if untouched != 0 {
		t.Fatalf("migration 00011 should remove untouched seed packages; found %d", untouched)
	}

	items, err := repo.ListPackages(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	legacyIDs := map[uuid.UUID]struct{}{
		legacySeedStarter: {}, legacySeedMiddle: {}, legacySeedAdvanced: {},
	}
	for _, p := range items {
		if _, ok := legacyIDs[p.ID]; ok {
			// Leftover upgrade rows (referenced/edited seeds) are allowed.
			continue
		}
		// Operator-created or prior-test packages may remain on a shared DB.
		t.Logf("non-seed catalog row present: code=%s id=%s (allowed on shared test DB)", p.Code, p.ID)
	}
	if len(items) == 0 {
		t.Log("catalog empty after migrations (ideal fresh DB)")
	}
}

func TestCreatePackageListOrderIntegration(t *testing.T) {
	pool := requirePackagingIntegration(t)
	f := newItestFixture(t, pool)
	ctx := context.Background()

	items, err := f.svc.ListPackages(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	byCode := map[domainpackaging.PackageCode]domainpackaging.Package{}
	for _, p := range items {
		byCode[p.Code] = p
	}
	for _, code := range []domainpackaging.PackageCode{codeStarter, codeMiddle, codeAdvanced} {
		if _, ok := byCode[code]; !ok {
			t.Fatalf("missing created package %s", code)
		}
	}
	if byCode[codeStarter].SortOrder > byCode[codeMiddle].SortOrder ||
		byCode[codeMiddle].SortOrder > byCode[codeAdvanced].SortOrder {
		t.Fatalf("expected STARTER < MIDDLE < ADVANCED sort_order, got %+v", []int{
			byCode[codeStarter].SortOrder, byCode[codeMiddle].SortOrder, byCode[codeAdvanced].SortOrder,
		})
	}
	if !byCode[codeAdvanced].AllowsUrgent || !byCode[codeAdvanced].BroadcastOnPublish ||
		!byCode[codeAdvanced].ShowcaseEligible || byCode[codeAdvanced].SearchPriority != 100 {
		t.Fatalf("ADVANCED capabilities: %+v", byCode[codeAdvanced])
	}
	if !byCode[codeMiddle].ShowcaseEligible || byCode[codeMiddle].BroadcastOnPublish {
		t.Fatalf("MIDDLE capabilities: %+v", byCode[codeMiddle])
	}
	if byCode[codeStarter].AllowsUrgent || byCode[codeStarter].ShowcaseEligible || byCode[codeStarter].BroadcastOnPublish {
		t.Fatalf("STARTER capabilities: %+v", byCode[codeStarter])
	}
	for i := 1; i < len(items); i++ {
		if items[i-1].SortOrder > items[i].SortOrder {
			t.Fatalf("sort_order not ascending: %+v", items)
		}
	}
}

func TestAssignmentCreateHistoryAndUniqueActiveIntegration(t *testing.T) {
	pool := requirePackagingIntegration(t)
	f := newItestFixture(t, pool)
	ctx := context.Background()

	first, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageCode: codeAdvanced,
	})
	if err != nil {
		t.Fatal(err)
	}
	hist, err := f.svc.ListAdvertPackageHistory(ctx, apppackaging.ListAdvertPackageHistoryInput{ActorUserID: f.adminID, AdvertID: f.advert.ID, Limit: intPtr(10)})
	if err != nil || len(hist.Items) != 1 {
		t.Fatalf("history=%+v err=%v", hist, err)
	}

	// Direct second ACTIVE insert must conflict.
	repo := pgpackaging.NewRepository(pool)
	dup := first.Assignment
	dup.ID = uuid.New()
	err = repo.CreateAssignment(ctx, dup)
	if err == nil {
		t.Fatal("expected unique active conflict")
	}
	if !asConflict(err) {
		t.Fatalf("want conflict, got %v", err)
	}
	f.clock.t = f.clock.t.Add(time.Second)
	second, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageCode: codeMiddle,
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Assignment.ID == first.Assignment.ID {
		t.Fatal("expected supersede + new assignment")
	}
	hist, err = f.svc.ListAdvertPackageHistory(ctx, apppackaging.ListAdvertPackageHistoryInput{ActorUserID: f.adminID, AdvertID: f.advert.ID, Limit: intPtr(10)})
	if err != nil || len(hist.Items) != 2 {
		t.Fatalf("history len=%d err=%v", len(hist.Items), err)
	}
	if hist.Items[0].Assignment.Status != domainpackaging.AssignmentStatusActive ||
		hist.Items[0].Assignment.ID != second.Assignment.ID ||
		hist.Items[1].Assignment.Status != domainpackaging.AssignmentStatusSuperseded {
		t.Fatalf("history statuses unexpected: %+v", hist.Items)
	}
}

func TestEffectiveFutureAssignmentIntegration(t *testing.T) {
	pool := requirePackagingIntegration(t)
	f := newItestFixture(t, pool)
	ctx := context.Background()
	start := f.clock.Now().Add(24 * time.Hour)
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageCode: codeStarter,
		StartsAt: &start,
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err := f.svc.GetAdvertPackage(ctx, f.advert.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view != nil {
		t.Fatal("future ACTIVE must not be effective yet")
	}
}

func TestConcurrentAssignmentOneActiveIntegration(t *testing.T) {
	pool := requirePackagingIntegration(t)
	f := newItestFixture(t, pool)
	ctx := context.Background()

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	pkgs := []domainpackaging.PackageCode{codeStarter, codeMiddle}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(pkgCode domainpackaging.PackageCode) {
			defer wg.Done()
			_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
				ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageCode: pkgCode,
			})
			errs <- err
		}(pkgs[i])
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && !asConflict(err) {
			t.Fatalf("unexpected err: %v", err)
		}
	}
	var active int
	if err := f.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM hrd_advert_package_assignments
WHERE advert_id = $1 AND status = 'ACTIVE'`, f.advert.ID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 1 {
		t.Fatalf("active=%d", active)
	}
}

func TestUrgentActivateDeactivateVersionIntegration(t *testing.T) {
	pool := requirePackagingIntegration(t)
	f := newItestFixture(t, pool)
	ctx := context.Background()
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageCode: codeAdvanced,
	})
	if err != nil {
		t.Fatal(err)
	}
	a1, err := f.svc.ActivateUrgent(ctx, f.ownerID, f.advert.ID)
	if err != nil {
		t.Fatal(err)
	}
	a2, err := f.svc.ActivateUrgent(ctx, f.ownerID, f.advert.ID)
	if err != nil || a1.ID != a2.ID {
		t.Fatalf("idempotent activate failed: %v", err)
	}
	if err := f.svc.DeactivateUrgent(ctx, f.ownerID, f.advert.ID); err != nil {
		t.Fatal(err)
	}
	a3, err := f.svc.ActivateUrgent(ctx, f.adminID, f.advert.ID)
	if err != nil {
		t.Fatal(err)
	}
	if a3.ActivationVersion != a1.ActivationVersion+1 {
		t.Fatalf("version=%d want %d", a3.ActivationVersion, a1.ActivationVersion+1)
	}
	_, err = f.svc.ActivateUrgent(ctx, f.otherID, f.advert.ID)
	if !asForbidden(err) {
		t.Fatalf("other user want forbidden, got %v", err)
	}
}

func TestConcurrentUrgentOneActiveIntegration(t *testing.T) {
	pool := requirePackagingIntegration(t)
	f := newItestFixture(t, pool)
	ctx := context.Background()
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageCode: codeAdvanced,
	})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan domainpackaging.AdvertFeatureActivation, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			act, err := f.svc.ActivateUrgent(ctx, f.ownerID, f.advert.ID)
			if err != nil {
				errs <- err
				return
			}
			results <- act
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("activate err: %v", err)
		}
	}
	var ids []uuid.UUID
	for act := range results {
		ids = append(ids, act.ID)
	}
	if len(ids) != 2 || ids[0] != ids[1] {
		// Concurrent may return same idempotent row twice, or one create + one idempotent read.
		var active int
		if err := f.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM hrd_advert_feature_activations
WHERE advert_id = $1 AND feature_code = 'URGENT' AND status = 'ACTIVE'`, f.advert.ID).Scan(&active); err != nil {
			t.Fatal(err)
		}
		if active != 1 {
			t.Fatalf("active urgent=%d ids=%v", active, ids)
		}
	}
	var active int
	if err := f.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM hrd_advert_feature_activations
WHERE advert_id = $1 AND feature_code = 'URGENT' AND status = 'ACTIVE'`, f.advert.ID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 1 {
		t.Fatalf("active urgent=%d", active)
	}
}

func TestPackageChangeDeactivatesUrgentIntegration(t *testing.T) {
	pool := requirePackagingIntegration(t)
	f := newItestFixture(t, pool)
	ctx := context.Background()
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageCode: codeAdvanced,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ActivateUrgent(ctx, f.ownerID, f.advert.ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageCode: codeMiddle,
	})
	if err != nil {
		t.Fatal(err)
	}
	var active int
	if err := f.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM hrd_advert_feature_activations
WHERE advert_id = $1 AND feature_code = 'URGENT' AND status = 'ACTIVE'`, f.advert.ID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("urgent still active=%d", active)
	}
}

func TestCheckViolationSafeMessageIntegration(t *testing.T) {
	pool := requirePackagingIntegration(t)
	f := newItestFixture(t, pool)
	ctx := context.Background()
	repo := pgpackaging.NewRepository(pool)
	now := f.clock.Now()
	err := repo.CreateAssignment(ctx, domainpackaging.AdvertPackageAssignment{
		ID: uuid.New(), AdvertID: f.advert.ID, PackageID: f.starter.ID,
		Status: "NOT_A_STATUS", StartsAt: now, AssignedByUserID: f.adminID, AssignedAt: now,
		Source: domainpackaging.AssignmentSourceAdmin, Version: 1, CreatedAt: now, UpdatedAt: now,
	})
	if err == nil {
		t.Fatal("expected check violation")
	}
	msg := err.Error()
	for _, bad := range []string{"SQLSTATE", "23514", "hrd_advert_package_assignments_status_check", "postgres://"} {
		if strings.Contains(msg, bad) {
			t.Fatalf("leaked %q in %q", bad, msg)
		}
	}
}

func TestNilStartsAtIdempotentIntegration(t *testing.T) {
	pool := requirePackagingIntegration(t)
	f := newItestFixture(t, pool)
	ctx := context.Background()
	first, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageCode: codeStarter,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Wall clock moves; request still omits StartsAt/EndsAt.
	time.Sleep(5 * time.Millisecond)
	second, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageCode: codeStarter,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Assignment.ID != second.Assignment.ID {
		t.Fatal("nil StartsAt same package must be idempotent on postgres")
	}
}

func asConflict(err error) bool {
	var ae *apperr.Error
	return errors.As(err, &ae) && ae.Kind == apperr.KindConflict
}

func asForbidden(err error) bool {
	var ae *apperr.Error
	return errors.As(err, &ae) && ae.Kind == apperr.KindForbidden
}

func asNotFound(err error) bool {
	var ae *apperr.Error
	return errors.As(err, &ae) && ae.Kind == apperr.KindNotFound
}

func intPtr(v int) *int { return &v }

func TestUpdatePackageAllowsUrgentFalseIntegration(t *testing.T) {
	pool := requirePackagingIntegration(t)
	f := newItestFixture(t, pool)
	ctx := context.Background()
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageCode: codeAdvanced,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ActivateUrgent(ctx, f.ownerID, f.advert.ID); err != nil {
		t.Fatal(err)
	}
	pkg, err := f.svc.GetPackageByCode(ctx, f.adminID, codeAdvanced)
	if err != nil {
		t.Fatal(err)
	}
	falseVal := false
	name := pkg.DisplayName
	updated, err := f.svc.UpdatePackage(ctx, apppackaging.UpdatePackageInput{
		ActorUserID: f.adminID, Code: codeAdvanced,
		ExpectedVersion: pkg.Version, DisplayName: &name, AllowsUrgent: &falseVal,
	})
	if err != nil || updated.AllowsUrgent {
		t.Fatalf("update: %#v %v", updated, err)
	}
	t.Cleanup(func() {
		trueVal := true
		cur, err := f.svc.GetPackageByCode(context.Background(), f.adminID, codeAdvanced)
		if err != nil {
			return
		}
		_, _ = f.svc.UpdatePackage(context.Background(), apppackaging.UpdatePackageInput{
			ActorUserID: f.adminID, Code: codeAdvanced,
			ExpectedVersion: cur.Version, AllowsUrgent: &trueVal,
		})
	})
	var active int
	if err := f.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM hrd_advert_feature_activations
WHERE advert_id = $1 AND feature_code = 'URGENT' AND status = 'ACTIVE'`, f.advert.ID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("urgent still active=%d", active)
	}
}

func TestCancelAdvertPackageIntegration(t *testing.T) {
	pool := requirePackagingIntegration(t)
	f := newItestFixture(t, pool)
	ctx := context.Background()
	if err := f.svc.CancelAdvertPackage(ctx, apppackaging.CancelAdvertPackageInput{
		ActorUserID: f.adminID, AdvertID: f.advert.ID,
	}); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageCode: codeAdvanced,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ActivateUrgent(ctx, f.ownerID, f.advert.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.CancelAdvertPackage(ctx, apppackaging.CancelAdvertPackageInput{
		ActorUserID: f.adminID, AdvertID: f.advert.ID,
	}); err != nil {
		t.Fatal(err)
	}
	var activeAssign, activeUrgent int
	_ = f.pool.QueryRow(ctx, `SELECT COUNT(*) FROM hrd_advert_package_assignments WHERE advert_id=$1 AND status='ACTIVE'`, f.advert.ID).Scan(&activeAssign)
	_ = f.pool.QueryRow(ctx, `SELECT COUNT(*) FROM hrd_advert_feature_activations WHERE advert_id=$1 AND status='ACTIVE'`, f.advert.ID).Scan(&activeUrgent)
	if activeAssign != 0 || activeUrgent != 0 {
		t.Fatalf("assign=%d urgent=%d", activeAssign, activeUrgent)
	}
}
