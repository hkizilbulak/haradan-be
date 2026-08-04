// Package campaign implements PostgreSQL persistence for hrd_campaigns.
package campaign

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincampaign "github.com/hkizilbulak/haradan-be/internal/domain/campaign"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

// ListFilter holds optional list predicates and keyset cursor for campaigns.
type ListFilter struct {
	EventType       *domaincampaign.CampaignEventType
	IsActive        *bool
	SourcePackageID *uuid.UUID
	TargetPackageID *uuid.UUID
	AfterCreatedAt  *time.Time
	AfterID         *uuid.UUID
	Limit           int
}

const (
	campaignNotFoundMessage = "Kampanya bulunamadı."
	staleVersionMessage     = "Kampanya başka bir işlem tarafından güncellendi."
	campaignConflictMessage = "Kampanya kodu zaten kullanılıyor."
)

const campaignColumns = `id, code, name, event_type, source_package_id, target_package_id,
title, description, email_subject, email_heading, email_body, cta_label, cta_url, badge_text,
image_asset_id, display_original_price_amount_minor, display_campaign_price_amount_minor,
currency_code, starts_at, ends_at, is_active, created_by_user_id, version, created_at, updated_at`

// Querier is implemented by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository persists campaigns.
type Repository struct {
	pool *pgxpool.Pool
	db   Querier
}

// NewRepository constructs a campaign repository bound to a pool.
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
		return nil, apperr.Internal(errors.New("campaign repository has no pool"))
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("begin campaign tx: %w", pg.SanitizeErr(err)))
	}
	return tx, nil
}

// Create inserts a campaign row.
func (r *Repository) Create(ctx context.Context, c domaincampaign.Campaign) error {
	const q = `
INSERT INTO hrd_campaigns (
  id, code, name, event_type, source_package_id, target_package_id,
  title, description, email_subject, email_heading, email_body, cta_label, cta_url, badge_text,
  image_asset_id, display_original_price_amount_minor, display_campaign_price_amount_minor,
  currency_code, starts_at, ends_at, is_active, created_by_user_id, version, created_at, updated_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25
)`
	_, err := r.db.Exec(ctx, q,
		c.ID, c.Code, c.Name, string(c.EventType), c.SourcePackageID, c.TargetPackageID,
		c.Title, c.Description, c.EmailSubject, c.EmailHeading, c.EmailBody, c.CTALabel, c.CTAURL, c.BadgeText,
		c.ImageAssetID, c.DisplayOriginalPriceAmountMinor, c.DisplayCampaignPriceAmountMinor,
		c.CurrencyCode, c.StartsAt, c.EndsAt, c.IsActive, c.CreatedByUserID, c.Version, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict(campaignConflictMessage)
		}
		if isCheckViolation(err) {
			return apperr.Validation("Kampanya geçersiz.")
		}
		if isForeignKeyViolation(err) {
			return apperr.Validation("Kampanya ilişkisi geçersiz.")
		}
		return apperr.Internal(fmt.Errorf("create campaign: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// GetByID loads a campaign by id.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (domaincampaign.Campaign, error) {
	q := `SELECT ` + campaignColumns + ` FROM hrd_campaigns WHERE id = $1`
	c, err := scanCampaign(r.db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domaincampaign.Campaign{}, apperr.NotFound(campaignNotFoundMessage)
	}
	if err != nil {
		return domaincampaign.Campaign{}, apperr.Internal(fmt.Errorf("get campaign: %w", pg.SanitizeErr(err)))
	}
	return c, nil
}

// List returns campaigns ordered by created_at DESC, id DESC with optional filters.
func (r *Repository) List(ctx context.Context, f ListFilter) ([]domaincampaign.Campaign, error) {
	var (
		b    strings.Builder
		args []any
	)
	b.WriteString(`SELECT ` + campaignColumns + ` FROM hrd_campaigns WHERE 1=1`)
	if f.EventType != nil {
		args = append(args, string(*f.EventType))
		fmt.Fprintf(&b, ` AND event_type = $%d`, len(args))
	}
	if f.IsActive != nil {
		args = append(args, *f.IsActive)
		fmt.Fprintf(&b, ` AND is_active = $%d`, len(args))
	}
	if f.SourcePackageID != nil {
		args = append(args, *f.SourcePackageID)
		fmt.Fprintf(&b, ` AND source_package_id = $%d`, len(args))
	}
	if f.TargetPackageID != nil {
		args = append(args, *f.TargetPackageID)
		fmt.Fprintf(&b, ` AND target_package_id = $%d`, len(args))
	}
	if f.AfterCreatedAt != nil && f.AfterID != nil {
		args = append(args, *f.AfterCreatedAt, *f.AfterID)
		fmt.Fprintf(&b, ` AND (created_at, id) < ($%d, $%d)`, len(args)-1, len(args))
	}
	b.WriteString(` ORDER BY created_at DESC, id DESC`)
	if f.Limit > 0 {
		args = append(args, f.Limit)
		fmt.Fprintf(&b, ` LIMIT $%d`, len(args))
	}

	rows, err := r.db.Query(ctx, b.String(), args...)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list campaigns: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()

	out := make([]domaincampaign.Campaign, 0)
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan campaign: %w", pg.SanitizeErr(err)))
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate campaigns: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// LockByID locks a campaign row FOR UPDATE.
func (r *Repository) LockByID(ctx context.Context, id uuid.UUID) (domaincampaign.Campaign, error) {
	q := `SELECT ` + campaignColumns + ` FROM hrd_campaigns WHERE id = $1 FOR UPDATE`
	c, err := scanCampaign(r.db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domaincampaign.Campaign{}, apperr.NotFound(campaignNotFoundMessage)
	}
	if err != nil {
		return domaincampaign.Campaign{}, apperr.Internal(fmt.Errorf("lock campaign: %w", pg.SanitizeErr(err)))
	}
	return c, nil
}

// UpdateOptimistic updates a campaign when id and version still match.
func (r *Repository) UpdateOptimistic(
	ctx context.Context,
	c domaincampaign.Campaign,
	expectedVersion int,
) (domaincampaign.Campaign, error) {
	const q = `
UPDATE hrd_campaigns
SET code = $3,
    name = $4,
    event_type = $5,
    source_package_id = $6,
    target_package_id = $7,
    title = $8,
    description = $9,
    email_subject = $10,
    email_heading = $11,
    email_body = $12,
    cta_label = $13,
    cta_url = $14,
    badge_text = $15,
    image_asset_id = $16,
    display_original_price_amount_minor = $17,
    display_campaign_price_amount_minor = $18,
    currency_code = $19,
    starts_at = $20,
    ends_at = $21,
    is_active = $22,
    version = version + 1,
    updated_at = $23
WHERE id = $1 AND version = $2
RETURNING ` + campaignColumns

	updated, err := scanCampaign(r.db.QueryRow(ctx, q,
		c.ID, expectedVersion,
		c.Code, c.Name, string(c.EventType), c.SourcePackageID, c.TargetPackageID,
		c.Title, c.Description, c.EmailSubject, c.EmailHeading, c.EmailBody, c.CTALabel, c.CTAURL, c.BadgeText,
		c.ImageAssetID, c.DisplayOriginalPriceAmountMinor, c.DisplayCampaignPriceAmountMinor,
		c.CurrencyCode, c.StartsAt, c.EndsAt, c.IsActive, c.UpdatedAt,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domaincampaign.Campaign{}, apperr.StaleVersion(staleVersionMessage)
	}
	if err != nil {
		if isUniqueViolation(err) {
			return domaincampaign.Campaign{}, apperr.Conflict(campaignConflictMessage)
		}
		if isCheckViolation(err) {
			return domaincampaign.Campaign{}, apperr.Validation("Kampanya geçersiz.")
		}
		if isForeignKeyViolation(err) {
			return domaincampaign.Campaign{}, apperr.Validation("Kampanya ilişkisi geçersiz.")
		}
		return domaincampaign.Campaign{}, apperr.Internal(fmt.Errorf("update campaign: %w", pg.SanitizeErr(err)))
	}
	return updated, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCampaign(row rowScanner) (domaincampaign.Campaign, error) {
	var (
		c         domaincampaign.Campaign
		eventType string
	)
	err := row.Scan(
		&c.ID, &c.Code, &c.Name, &eventType, &c.SourcePackageID, &c.TargetPackageID,
		&c.Title, &c.Description, &c.EmailSubject, &c.EmailHeading, &c.EmailBody, &c.CTALabel, &c.CTAURL, &c.BadgeText,
		&c.ImageAssetID, &c.DisplayOriginalPriceAmountMinor, &c.DisplayCampaignPriceAmountMinor,
		&c.CurrencyCode, &c.StartsAt, &c.EndsAt, &c.IsActive, &c.CreatedByUserID, &c.Version, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return domaincampaign.Campaign{}, err
	}
	c.EventType = domaincampaign.CampaignEventType(eventType)
	return c, nil
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
