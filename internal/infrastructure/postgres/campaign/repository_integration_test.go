package campaign_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	appcampaign "github.com/hkizilbulak/haradan-be/internal/application/campaign"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincampaign "github.com/hkizilbulak/haradan-be/internal/domain/campaign"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	pguser "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/user"
)

func requirePackageAdminIntegration(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if strings.TrimSpace(os.Getenv("RUN_PACKAGE_ADMIN_HTTP_INTEGRATION_TESTS")) != "1" {
		t.Skip("RUN_PACKAGE_ADMIN_HTTP_INTEGRATION_TESTS!=1; skipping")
	}
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping")
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

type assetNotFound struct{}

func (assetNotFound) FindAssetByID(context.Context, uuid.UUID) (domainmedia.Asset, error) {
	return domainmedia.Asset{}, apperr.NotFound("Medya varlığı bulunamadı.")
}

func TestCampaignCRUDOptimisticIntegration(t *testing.T) {
	pool := requirePackageAdminIntegration(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	users := pguser.NewRepository(pool)
	email := "camp-" + uuid.NewString() + "@example.com"
	admin := domainuser.User{
		ID: uuid.New(), Email: email, EmailNormalized: strings.ToLower(email),
		PasswordHash: "hash", Role: domainuser.RoleAdmin, Status: domainuser.StatusActive,
		FirstName: "A", LastName: "D", SecurityStamp: uuid.New(),
		CreatedAt: now, UpdatedAt: now,
	}
	if err := users.Create(ctx, admin); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM hrd_campaigns WHERE created_by_user_id = $1`, admin.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM hrd_users WHERE id = $1`, admin.ID)
	})

	targetCode := "ADVANCED"
	var existingID uuid.UUID
	lookupErr := pool.QueryRow(ctx, `SELECT id FROM hrd_packages WHERE code = $1`, targetCode).Scan(&existingID)
	if lookupErr != nil {
		pkgID := uuid.New()
		if _, err := pool.Exec(ctx, `
INSERT INTO hrd_packages (
    id, code, display_name, description, badge_text, benefits,
    display_price_amount_minor, currency_code, default_duration_days,
    allows_urgent, showcase_eligible, search_priority, broadcast_on_publish,
    is_active, sort_order, version, created_at, updated_at
) VALUES (
    $1, $2, 'Advanced', NULL, NULL, '[]'::jsonb,
    NULL, 'TRY', NULL,
    true, true, 100, true,
    true, 30, 1, $3, $3
)`, pkgID, targetCode, now); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `
DELETE FROM hrd_packages WHERE id = $1 AND NOT EXISTS (
  SELECT 1 FROM hrd_advert_package_assignments a WHERE a.package_id = $1
) AND NOT EXISTS (
  SELECT 1 FROM hrd_campaigns c WHERE c.source_package_id = $1 OR c.target_package_id = $1
)`, pkgID)
		})
	}

	svc, err := appcampaign.NewPostgresService(
		pool,
		appcampaign.NewPostgresPackageLookup(pool),
		assetNotFound{},
		users,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	target := targetCode
	created, err := svc.CreateCampaign(ctx, appcampaign.CreateCampaignInput{
		ActorUserID:       admin.ID,
		Code:              "c-" + uuid.NewString()[:8],
		Name:              "Test Campaign",
		EventType:         domaincampaign.CampaignEventTypePackageRenewal,
		Title:             "Renew now",
		StartsAt:          now,
		IsActive:          true,
		TargetPackageCode: &target,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetCampaign(ctx, admin.ID, created.ID)
	if err != nil || got.ID != created.ID {
		t.Fatalf("get: %#v %v", got, err)
	}
	name := "Updated Campaign"
	updated, err := svc.UpdateCampaign(ctx, appcampaign.UpdateCampaignInput{
		ActorUserID: admin.ID, CampaignID: created.ID, ExpectedVersion: created.Version, Name: &name,
	})
	if err != nil || updated.Name != name || updated.Version != created.Version+1 {
		t.Fatalf("update: %#v %v", updated, err)
	}
	_, err = svc.UpdateCampaign(ctx, appcampaign.UpdateCampaignInput{
		ActorUserID: admin.ID, CampaignID: created.ID, ExpectedVersion: created.Version, Name: &name,
	})
	var ae *apperr.Error
	if err == nil || !errors.As(err, &ae) || ae.Code != apperr.CodeStaleVersion {
		t.Fatalf("expected STALE_VERSION got %v", err)
	}
	page, err := svc.ListCampaigns(ctx, admin.ID, appcampaign.ListCampaignsInput{})
	if err != nil || len(page.Items) == 0 {
		t.Fatalf("list: %#v %v", page, err)
	}
}
