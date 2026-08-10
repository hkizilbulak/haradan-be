package notification_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	appnotification "github.com/hkizilbulak/haradan-be/internal/application/notification"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
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

func TestNotificationTemplateUpdateIntegration(t *testing.T) {
	pool := requirePackageAdminIntegration(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	users := pguser.NewRepository(pool)
	email := "tpl-" + uuid.NewString() + "@example.com"
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
		_, _ = pool.Exec(context.Background(), `
UPDATE hrd_notification_templates
SET name = 'Package advert published placeholder',
    in_app_title_template = 'Yeni ilan',
    in_app_body_template = 'Yeni bir ilan yayınlandı.',
    version = 1,
    is_active = false,
    updated_by_user_id = NULL,
    updated_at = TIMESTAMPTZ '2020-01-01 00:00:00+00'
WHERE event_type = 'PACKAGE_ADVERT_PUBLISHED'`)
		_, _ = pool.Exec(context.Background(), `DELETE FROM hrd_users WHERE id = $1`, admin.ID)
	})

	svc, err := appnotification.NewPostgresService(pool, users, nil)
	if err != nil {
		t.Fatal(err)
	}
	list, err := svc.ListTemplates(ctx, admin.ID)
	if err != nil || len(list) < 4 {
		t.Fatalf("list templates: %d %v", len(list), err)
	}
	tpl, err := svc.GetTemplateByEventType(ctx, admin.ID, domainnotification.TemplateEventTypePackageAdvertPublished)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := svc.UpdateTemplate(ctx, appnotification.UpdateTemplateInput{
		ActorUserID: admin.ID, EventType: tpl.EventType, ExpectedVersion: tpl.Version,
		Name: strPtr("Updated"), InAppTitleTemplate: strPtr("Title"), InAppBodyTemplate: strPtr("Body"), IsActive: boolPtr(true),
	})
	if err != nil || updated.Version != tpl.Version+1 {
		t.Fatalf("update: %#v %v", updated, err)
	}
	_, err = svc.UpdateTemplate(ctx, appnotification.UpdateTemplateInput{
		ActorUserID: admin.ID, EventType: tpl.EventType, ExpectedVersion: tpl.Version,
		Name: strPtr("Updated"), InAppTitleTemplate: strPtr("Title"), InAppBodyTemplate: strPtr("Body"), IsActive: boolPtr(true),
	})
	var ae *apperr.Error
	if err == nil || !errors.As(err, &ae) || ae.Code != apperr.CodeStaleVersion {
		t.Fatalf("expected STALE_VERSION got %v", err)
	}
}
func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
