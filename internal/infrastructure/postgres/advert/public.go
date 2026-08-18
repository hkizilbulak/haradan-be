package advert

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

// SearchPublished executes the package-priority public keyset query. The
// lateral joins guarantee one effective assignment, one urgent activation, and
// one cover per advert without multiplying card rows.
func (r *Repository) SearchPublished(ctx context.Context, q domainadvert.PublicSearchQuery) ([]domainadvert.PublicCard, error) {
	const order = `COALESCE(pkg.search_priority, 0) DESC, a.published_at DESC, a.id DESC`
	sql := publicCardSelect("$10") + `
WHERE a.status = 'PUBLISHED' AND a.deleted_at IS NULL
  AND ($1::uuid IS NULL OR a.category_id = $1)
  AND ($2::uuid IS NULL OR d.province_id = $2)
  AND ($3::uuid IS NULL OR a.district_id = $3)
  AND ($4::uuid IS NULL OR a.horse_id = $4)
  AND ($5::boolean IS NULL OR $5 = EXISTS (
    SELECT 1 FROM hrd_advert_media hm
    JOIN hrd_media_assets hma ON hma.id = hm.asset_id AND hma.lifecycle_status = 'MASTER_READY'
    WHERE hm.advert_id = a.id
  ))
  AND ($6::int IS NULL OR (COALESCE(pkg.search_priority, 0), a.published_at, a.id) < ($6, $7, $8))
ORDER BY ` + order + ` LIMIT $9`
	var priority *int
	var publishedAt *time.Time
	var id *uuid.UUID
	if q.After != nil {
		priority = &q.After.Priority
		t := q.After.PublishedAt
		publishedAt = &t
		id = &q.After.ID
	}
	rows, err := r.db.Query(ctx, sql, q.CategoryID, q.ProvinceID, q.DistrictID, q.HorseID, q.HasPhoto, priority, publishedAt, id, q.Limit, q.ActorUserID)
	return scanPublicCards(rows, err, "search published adverts")
}

func (r *Repository) ListHomepageNew(ctx context.Context, q domainadvert.HomepageNewQuery) ([]domainadvert.PublicCard, error) {
	sql := publicCardSelect("$4") + `
WHERE a.status = 'PUBLISHED' AND a.deleted_at IS NULL
  AND ($1::timestamptz IS NULL OR (a.published_at, a.id) < ($1, $2))
ORDER BY a.published_at DESC, a.id DESC LIMIT $3`
	var publishedAt *time.Time
	var id *uuid.UUID
	if q.After != nil {
		t := q.After.PublishedAt
		publishedAt = &t
		id = &q.After.ID
	}
	rows, err := r.db.Query(ctx, sql, publishedAt, id, q.Limit, q.ActorUserID)
	return scanPublicCards(rows, err, "list homepage new adverts")
}

func (r *Repository) ListHomepageShowcase(ctx context.Context, seed string, limit int, actorUserID *uuid.UUID) ([]domainadvert.PublicCard, error) {
	sql := publicCardSelect("$3") + `
WHERE a.status = 'PUBLISHED' AND a.deleted_at IS NULL
  AND pkg.showcase_eligible = true
ORDER BY md5(a.id::text || $1), a.id LIMIT $2`
	rows, err := r.db.Query(ctx, sql, seed, limit, actorUserID)
	return scanPublicCards(rows, err, "list homepage showcase")
}

func (r *Repository) ListHomepageUrgent(ctx context.Context, limit int, actorUserID *uuid.UUID) ([]domainadvert.PublicCard, error) {
	sql := publicCardSelect("$2") + `
WHERE a.status = 'PUBLISHED' AND a.deleted_at IS NULL
  AND urgent.id IS NOT NULL
ORDER BY urgent.activated_at DESC, a.id DESC LIMIT $1`
	rows, err := r.db.Query(ctx, sql, limit, actorUserID)
	return scanPublicCards(rows, err, "list homepage urgent")
}

func (r *Repository) ListHomepageFeatured(ctx context.Context, limit int, actorUserID *uuid.UUID) ([]domainadvert.PublicCard, error) {
	sql := publicCardSelect("$2") + `
WHERE a.status = 'PUBLISHED' AND a.deleted_at IS NULL
  AND featured.id IS NOT NULL
ORDER BY COALESCE(pkg.search_priority, 0) DESC, featured.ends_at NULLS LAST, a.published_at DESC, a.id DESC
LIMIT $1`
	rows, err := r.db.Query(ctx, sql, limit, actorUserID)
	return scanPublicCards(rows, err, "list homepage featured")
}

func (r *Repository) GetPublishedDetail(ctx context.Context, advertID uuid.UUID, actorUserID *uuid.UUID) (domainadvert.PublicDetail, error) {
	sql := publicCardSelect("$2") + `
WHERE a.id = $1 AND a.status = 'PUBLISHED' AND a.deleted_at IS NULL`
	card, err := scanPublicCard(r.db.QueryRow(ctx, sql, advertID, actorUserID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainadvert.PublicDetail{}, apperr.NotFound(advertNotFoundMessage)
	}
	if err != nil {
		return domainadvert.PublicDetail{}, apperr.Internal(fmt.Errorf("get published advert: %w", pg.SanitizeErr(err)))
	}
	const metadata = `
SELECT a.description, c.name, c.slug, d.name, p.name, h.id, h.original_name, h.tjk_number, a.properties
FROM hrd_adverts a
JOIN hrd_categories c ON c.id = a.category_id
JOIN hrd_districts d ON d.id = a.district_id
JOIN hrd_provinces p ON p.id = d.province_id
LEFT JOIN hrd_horses h ON h.id = a.horse_id
WHERE a.id = $1`
	var out domainadvert.PublicDetail
	out.PublicCard = card
	var horseID *uuid.UUID
	var horseName *string
	var horseTJKNumber *string
	var props []byte
	if err := r.db.QueryRow(ctx, metadata, advertID).Scan(&out.Description, &out.CategoryName, &out.CategorySlug, &out.DistrictName, &out.ProvinceName, &horseID, &horseName, &horseTJKNumber, &props); err != nil {
		return domainadvert.PublicDetail{}, apperr.Internal(fmt.Errorf("get published advert metadata: %w", pg.SanitizeErr(err)))
	}
	if horseID != nil && horseName != nil {
		out.Horse = &domainadvert.PublicHorse{ID: *horseID, Name: *horseName, TJKNumber: horseTJKNumber}
	}
	media, err := r.listPublicMedia(ctx, advertID)
	if err != nil {
		return domainadvert.PublicDetail{}, err
	}
	out.Media = media
	properties, err := r.listPublicProperties(ctx, advertID, props)
	if err != nil {
		return domainadvert.PublicDetail{}, err
	}
	out.Properties = properties
	return out, nil
}

// The final actor id is intentionally last in every card query. It allows the
// same public endpoint to enrich favorites when selective auth supplied a
// principal, while anonymous callers receive null.
func publicCardSelect(actorArg string) string {
	return `
SELECT a.id, a.category_id, a.district_id, d.province_id, a.horse_id, a.title,
       a.price_amount_minor, a.price_currency, a.published_at,
       cover.asset_id, cover.display_order, cover.is_cover, cover.object_key,
       pkg.code, pkg.display_name, pkg.badge_text, COALESCE(pkg.search_priority, 0),
       (urgent.id IS NOT NULL), urgent.activated_at,
       (featured.id IS NOT NULL), featured.ends_at,
       CASE WHEN ` + actorArg + `::uuid IS NULL THEN NULL ELSE EXISTS (
           SELECT 1 FROM hrd_favorites f WHERE f.advert_id = a.id AND f.user_id = ` + actorArg + `
       ) END,
       a.view_count
FROM hrd_adverts a
JOIN hrd_districts d ON d.id = a.district_id
LEFT JOIN LATERAL (
  SELECT p.code, p.display_name, p.badge_text, p.search_priority, p.showcase_eligible
  FROM hrd_advert_package_assignments pa
  JOIN hrd_packages p ON p.id = pa.package_id
  WHERE pa.advert_id = a.id AND pa.status = 'ACTIVE'
    AND pa.starts_at <= now() AND (pa.ends_at IS NULL OR pa.ends_at > now())
  LIMIT 1
) pkg ON true
LEFT JOIN LATERAL (
  SELECT fa.id, fa.activated_at
  FROM hrd_advert_feature_activations fa
  WHERE fa.advert_id = a.id AND fa.feature_code = 'URGENT' AND fa.status = 'ACTIVE'
  LIMIT 1
) urgent ON true
LEFT JOIN LATERAL (
  SELECT fa.id, fa.ends_at
  FROM hrd_advert_feature_activations fa
  WHERE fa.advert_id = a.id AND fa.feature_code = 'FEATURED' AND fa.status = 'ACTIVE'
    AND (fa.ends_at IS NULL OR fa.ends_at > now())
  LIMIT 1
) featured ON true
LEFT JOIN LATERAL (
  SELECT am.asset_id, am.display_order, am.is_cover, COALESCE(v.object_key, ma.master_object_key) AS object_key
  FROM hrd_advert_media am
  JOIN hrd_media_assets ma ON ma.id = am.asset_id AND ma.lifecycle_status = 'MASTER_READY'
  LEFT JOIN hrd_media_variants v ON v.asset_id = ma.id AND v.transform_profile = 'SEARCH' AND v.lifecycle_status = 'READY'
  WHERE am.advert_id = a.id
  ORDER BY (v.object_key IS NOT NULL) DESC, am.is_cover DESC, am.display_order ASC, am.asset_id ASC
  LIMIT 1
) cover ON true`
}

func (r *Repository) RecordView(ctx context.Context, advertID uuid.UUID, ipAddress string) error {
	if ipAddress == "" {
		return nil
	}
	const q = `
WITH new_view AS (
    INSERT INTO hrd_advert_views (advert_id, ip_address, created_at)
    VALUES ($1, $2, $3)
    ON CONFLICT (advert_id, ip_address) DO NOTHING
    RETURNING advert_id
)
UPDATE hrd_adverts
SET view_count = view_count + 1
WHERE id IN (SELECT advert_id FROM new_view)`
	_, err := r.db.Exec(ctx, q, advertID, ipAddress, time.Now().UTC())
	if err != nil {
		return apperr.Internal(fmt.Errorf("record advert view: %w", pg.SanitizeErr(err)))
	}
	return nil
}

func scanPublicCards(rows pgx.Rows, queryErr error, op string) ([]domainadvert.PublicCard, error) {
	if queryErr != nil {
		return nil, apperr.Internal(fmt.Errorf("%s: %w", op, pg.SanitizeErr(queryErr)))
	}
	defer rows.Close()
	out := make([]domainadvert.PublicCard, 0)
	for rows.Next() {
		card, err := scanPublicCard(rows)
		if err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan %s: %w", op, pg.SanitizeErr(err)))
		}
		out = append(out, card)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate %s: %w", op, pg.SanitizeErr(err)))
	}
	return out, nil
}

func scanPublicCard(row interface{ Scan(...any) error }) (domainadvert.PublicCard, error) {
	var card domainadvert.PublicCard
	var amount *int64
	var currency *string
	var coverAsset *uuid.UUID
	var coverOrder *int
	var coverIsCover *bool
	var coverKey *string
	if err := row.Scan(&card.ID, &card.CategoryID, &card.DistrictID, &card.ProvinceID, &card.HorseID, &card.Title,
		&amount, &currency, &card.PublishedAt, &coverAsset, &coverOrder, &coverIsCover, &coverKey,
		&card.PackageCode, &card.PackageDisplayName, &card.PackageBadgeText, &card.SearchPriority,
		&card.IsUrgent, &card.UrgentActivatedAt, &card.IsFeatured, &card.FeaturedUntil, &card.IsFavorite, &card.ViewCount); err != nil {
		return card, err
	}
	if amount != nil && currency != nil {
		card.Price = &domainadvert.Money{AmountMinor: *amount, Currency: *currency}
	}
	if coverAsset != nil && coverOrder != nil && coverIsCover != nil {
		// object_key may be null while MASTER_READY (variant still pending);
		// public URL is projected from asset id, so cover must still surface.
		key := ""
		if coverKey != nil {
			key = *coverKey
		}
		card.Cover = &domainadvert.PublicMedia{
			AssetID:      *coverAsset,
			DisplayOrder: *coverOrder,
			IsCover:      *coverIsCover,
			ObjectKey:    key,
		}
	}
	return card, nil
}

func (r *Repository) listPublicMedia(ctx context.Context, advertID uuid.UUID) ([]domainadvert.PublicMedia, error) {
	rows, err := r.db.Query(ctx, `
SELECT am.asset_id, am.display_order, am.is_cover, COALESCE(v.object_key, ma.master_object_key)
FROM hrd_advert_media am JOIN hrd_media_assets ma ON ma.id = am.asset_id AND ma.lifecycle_status = 'MASTER_READY'
LEFT JOIN hrd_media_variants v ON v.asset_id = ma.id AND v.transform_profile = 'DETAIL' AND v.lifecycle_status = 'READY'
WHERE am.advert_id = $1 ORDER BY am.display_order, am.asset_id`, advertID)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list public media: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()
	out := []domainadvert.PublicMedia{}
	for rows.Next() {
		var m domainadvert.PublicMedia
		if err := rows.Scan(&m.AssetID, &m.DisplayOrder, &m.IsCover, &m.ObjectKey); err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan public media: %w", pg.SanitizeErr(err)))
		}
		out = append(out, m)
	}
	return out, nil
}

func (r *Repository) listPublicProperties(ctx context.Context, advertID uuid.UUID, raw []byte) ([]domainadvert.PublicProperty, error) {
	var values map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, apperr.Internal(fmt.Errorf("decode public properties: %w", err))
	}
	rows, err := r.db.Query(ctx, `
SELECT cp.code, cp.title FROM hrd_category_properties cp
JOIN hrd_adverts a ON a.category_id = cp.category_id
WHERE a.id = $1 AND cp.is_active = true AND cp.is_public_visible = true ORDER BY cp.sort_order, cp.code`, advertID)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list public properties: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()
	out := []domainadvert.PublicProperty{}
	for rows.Next() {
		var code, title string
		if err := rows.Scan(&code, &title); err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan public properties: %w", pg.SanitizeErr(err)))
		}
		if value, ok := values[code]; ok {
			out = append(out, domainadvert.PublicProperty{Code: code, Title: title, Value: value})
		}
	}
	return out, nil
}
