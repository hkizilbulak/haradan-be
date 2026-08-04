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

var (
	seedStarter  = uuid.MustParse("a0000000-0000-4000-8000-000000000001")
	seedMiddle   = uuid.MustParse("a0000000-0000-4000-8000-000000000002")
	seedAdvanced = uuid.MustParse("a0000000-0000-4000-8000-000000000003")
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
	pool    *pgxpool.Pool
	svc     *apppackaging.Service
	clock   itestClock
	adminID uuid.UUID
	ownerID uuid.UUID
	otherID uuid.UUID
	advert  domainadvert.Advert
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

	clock := itestClock{t: now}
	svc, err := apppackaging.NewPostgresService(pool, adverts, users, clock)
	if err != nil {
		t.Fatal(err)
	}
	return &itestFixture{pool: pool, svc: svc, clock: clock, adminID: adminID, ownerID: ownerID, otherID: otherID, advert: advert}
}

func TestPackageSeedListOrderIntegration(t *testing.T) {
	pool := requirePackagingIntegration(t)
	ctx := context.Background()
	repo := pgpackaging.NewRepository(pool)
	items, err := repo.ListPackages(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 3 {
		t.Fatalf("expected seeded packages, got %d", len(items))
	}
	byCode := map[domainpackaging.PackageCode]domainpackaging.Package{}
	for _, p := range items {
		byCode[p.Code] = p
	}
	if byCode[domainpackaging.PackageCodeStarter].ID != seedStarter ||
		byCode[domainpackaging.PackageCodeMiddle].ID != seedMiddle ||
		byCode[domainpackaging.PackageCodeAdvanced].ID != seedAdvanced {
		t.Fatal("seed package ids mismatch")
	}
	// Active list must be ordered by sort_order, code.
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
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageID: seedAdvanced,
	})
	if err != nil {
		t.Fatal(err)
	}
	hist, err := f.svc.ListAdvertPackageHistory(ctx, f.adminID, f.advert.ID, 10, nil, nil)
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
	second, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageID: seedMiddle,
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Assignment.ID == first.Assignment.ID {
		t.Fatal("expected supersede + new assignment")
	}
	hist, err = f.svc.ListAdvertPackageHistory(ctx, f.adminID, f.advert.ID, 10, nil, nil)
	if err != nil || len(hist.Items) != 2 {
		t.Fatalf("history len=%d err=%v", len(hist.Items), err)
	}
	if hist.Items[0].Assignment.Status != domainpackaging.AssignmentStatusActive ||
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
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageID: seedStarter,
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
	pkgs := []uuid.UUID{seedStarter, seedMiddle}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(pkgID uuid.UUID) {
			defer wg.Done()
			_, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
				ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageID: pkgID,
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
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageID: seedAdvanced,
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
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageID: seedAdvanced,
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
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageID: seedAdvanced,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ActivateUrgent(ctx, f.ownerID, f.advert.ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageID: seedMiddle,
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
		ID: uuid.New(), AdvertID: f.advert.ID, PackageID: seedStarter,
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
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageID: seedStarter,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Wall clock moves; request still omits StartsAt/EndsAt.
	time.Sleep(5 * time.Millisecond)
	second, err := f.svc.AssignAdvertPackage(ctx, apppackaging.AssignAdvertPackageInput{
		ActorUserID: f.adminID, AdvertID: f.advert.ID, PackageID: seedStarter,
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
