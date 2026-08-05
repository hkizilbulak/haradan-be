package notification_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
	pgnotification "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/notification"
)

func requireNotificationRuntimeIntegration(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if strings.TrimSpace(os.Getenv("RUN_NOTIFICATION_RUNTIME_INTEGRATION_TESTS")) != "1" {
		t.Skip("RUN_NOTIFICATION_RUNTIME_INTEGRATION_TESTS!=1; skipping")
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

func TestMigration00010EmailDeliveryColumns(t *testing.T) {
	pool := requireNotificationRuntimeIntegration(t)
	ctx := context.Background()

	rows, err := pool.Query(ctx, `
SELECT column_name
FROM information_schema.columns
WHERE table_name = 'hrd_user_notification_states'
  AND column_name LIKE 'email_%'
ORDER BY column_name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		got[name] = true
	}
	for _, want := range []string{
		"email_status", "email_idempotency_key", "email_attempt_count",
		"email_last_attempt_at", "email_sent_at", "email_last_error",
	} {
		if !got[want] {
			t.Fatalf("missing column %s", want)
		}
	}

	var idx int
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM pg_indexes
WHERE indexname IN (
  'hrd_user_notification_states_email_idempotency_key_uidx',
  'hrd_user_notification_states_notification_email_user_idx',
  'hrd_users_active_id_idx',
  'hrd_users_active_verified_id_idx'
)`).Scan(&idx); err != nil {
		t.Fatal(err)
	}
	if idx != 4 {
		t.Fatalf("expected 4 email/fanout indexes, got %d", idx)
	}
}

func TestNotificationEventKeyUniqueness(t *testing.T) {
	pool := requireNotificationRuntimeIntegration(t)
	ctx := context.Background()
	repo := pgnotification.NewRepository(pool)
	now := time.Now().UTC()
	id1 := uuid.New()
	eventKey := domainnotification.PackageAdvertPublishedEventKey(uuid.New(), uuid.New())
	n := domainnotification.Notification{
		ID:        id1,
		EventType: domainnotification.TemplateEventTypePackageAdvertPublished,
		EventKey:  eventKey,
		Title:     "t",
		Body:      "b",
		Payload:   []byte(`{}`),
		CreatedAt: now,
	}
	created, err := repo.CreateNotificationEventIdempotent(ctx, n)
	if err != nil || !created {
		t.Fatalf("first insert: created=%v err=%v", created, err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM hrd_notifications WHERE id = $1`, id1)
	})
	n2 := n
	n2.ID = uuid.New()
	created2, err := repo.CreateNotificationEventIdempotent(ctx, n2)
	if err != nil {
		t.Fatal(err)
	}
	if created2 {
		t.Fatal("duplicate event_key must be idempotent no-op")
	}
}
