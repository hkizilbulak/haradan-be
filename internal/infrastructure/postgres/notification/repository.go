// Package notification implements PostgreSQL persistence for hrd_notification_templates.
package notification

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

const (
	templateNotFoundMessage = "Bildirim şablonu bulunamadı."
	staleVersionMessage     = "Bildirim şablonu başka bir işlem tarafından güncellendi."
)

const templateColumns = `id, event_type, name, in_app_title_template, in_app_body_template,
resend_template_id, email_subject_fallback, is_active, version, updated_by_user_id,
created_at, updated_at`

// Querier is implemented by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository persists notification templates.
type Repository struct {
	pool *pgxpool.Pool
	db   Querier
}

// NewRepository constructs a notification template repository bound to a pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, db: pool}
}

// WithTx returns a repository scoped to a transaction querier.
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{pool: r.pool, db: tx}
}

// BeginTx starts a read-write transaction.
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	if r.pool == nil {
		return nil, apperr.Internal(errors.New("notification repository has no pool"))
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("begin notification tx: %w", pg.SanitizeErr(err)))
	}
	return tx, nil
}

// ListTemplates returns templates ordered by event_type ASC.
func (r *Repository) ListTemplates(ctx context.Context) ([]domainnotification.NotificationTemplate, error) {
	q := `SELECT ` + templateColumns + ` FROM hrd_notification_templates ORDER BY event_type ASC`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list notification templates: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()

	out := make([]domainnotification.NotificationTemplate, 0)
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan notification template: %w", pg.SanitizeErr(err)))
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate notification templates: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// GetByEventType loads a template by event type.
func (r *Repository) GetByEventType(
	ctx context.Context,
	eventType domainnotification.TemplateEventType,
) (domainnotification.NotificationTemplate, error) {
	q := `SELECT ` + templateColumns + ` FROM hrd_notification_templates WHERE event_type = $1`
	t, err := scanTemplate(r.db.QueryRow(ctx, q, string(eventType)))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainnotification.NotificationTemplate{}, apperr.NotFound(templateNotFoundMessage)
	}
	if err != nil {
		return domainnotification.NotificationTemplate{}, apperr.Internal(fmt.Errorf("get notification template: %w", pg.SanitizeErr(err)))
	}
	return t, nil
}

// LockByEventType locks a template row FOR UPDATE.
func (r *Repository) LockByEventType(
	ctx context.Context,
	eventType domainnotification.TemplateEventType,
) (domainnotification.NotificationTemplate, error) {
	q := `SELECT ` + templateColumns + ` FROM hrd_notification_templates WHERE event_type = $1 FOR UPDATE`
	t, err := scanTemplate(r.db.QueryRow(ctx, q, string(eventType)))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainnotification.NotificationTemplate{}, apperr.NotFound(templateNotFoundMessage)
	}
	if err != nil {
		return domainnotification.NotificationTemplate{}, apperr.Internal(fmt.Errorf("lock notification template: %w", pg.SanitizeErr(err)))
	}
	return t, nil
}

// UpdateOptimistic updates a template when event_type and version still match.
func (r *Repository) UpdateOptimistic(
	ctx context.Context,
	t domainnotification.NotificationTemplate,
	expectedVersion int,
) (domainnotification.NotificationTemplate, error) {
	const q = `
UPDATE hrd_notification_templates
SET name = $3,
    in_app_title_template = $4,
    in_app_body_template = $5,
    resend_template_id = $6,
    email_subject_fallback = $7,
    is_active = $8,
    updated_by_user_id = $9,
    version = version + 1,
    updated_at = $10
WHERE event_type = $1 AND version = $2
RETURNING ` + templateColumns

	updated, err := scanTemplate(r.db.QueryRow(ctx, q,
		string(t.EventType), expectedVersion,
		t.Name, t.InAppTitleTemplate, t.InAppBodyTemplate,
		t.ResendTemplateID, t.EmailSubjectFallback, t.IsActive, t.UpdatedByUserID, t.UpdatedAt,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainnotification.NotificationTemplate{}, apperr.StaleVersion(staleVersionMessage)
	}
	if err != nil {
		if isCheckViolation(err) {
			return domainnotification.NotificationTemplate{}, apperr.Validation("Bildirim şablonu geçersiz.")
		}
		if isForeignKeyViolation(err) {
			return domainnotification.NotificationTemplate{}, apperr.Validation("Bildirim şablonu ilişkisi geçersiz.")
		}
		if isUniqueViolation(err) {
			return domainnotification.NotificationTemplate{}, apperr.Conflict("Bildirim şablonu çakışması.")
		}
		return domainnotification.NotificationTemplate{}, apperr.Internal(fmt.Errorf("update notification template: %w", pg.SanitizeErr(err)))
	}
	return updated, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTemplate(row rowScanner) (domainnotification.NotificationTemplate, error) {
	var (
		t         domainnotification.NotificationTemplate
		eventType string
	)
	err := row.Scan(
		&t.ID, &eventType, &t.Name, &t.InAppTitleTemplate, &t.InAppBodyTemplate,
		&t.ResendTemplateID, &t.EmailSubjectFallback, &t.IsActive, &t.Version, &t.UpdatedByUserID,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return domainnotification.NotificationTemplate{}, err
	}
	t.EventType = domainnotification.TemplateEventType(eventType)
	return t, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

func isCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514"
}
