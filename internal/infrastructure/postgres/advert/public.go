package advert

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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
SELECT a.description, a.address, c.name, c.slug, d.name, p.name, h.id, h.original_name, h.tjk_number, a.properties, u.phone, a.owner_user_id
FROM hrd_adverts a
LEFT JOIN hrd_categories c ON c.id = a.category_id
LEFT JOIN hrd_districts d ON d.id = a.district_id
LEFT JOIN hrd_provinces p ON p.id = d.province_id
LEFT JOIN hrd_horses h ON h.id = a.horse_id
LEFT JOIN hrd_users u ON u.id = a.owner_user_id
WHERE a.id = $1`
	var out domainadvert.PublicDetail
	out.PublicCard = card
	var (
		desc           *string
		addr           *string
		catName        *string
		catSlug        *string
		distName       *string
		provName       *string
		horseID        *uuid.UUID
		horseName      *string
		horseTJKNumber *string
		props          []byte
		userPhone      *string
		ownerUserID    *uuid.UUID
	)
	if err := r.db.QueryRow(ctx, metadata, advertID).Scan(&desc, &addr, &catName, &catSlug, &distName, &provName, &horseID, &horseName, &horseTJKNumber, &props, &userPhone, &ownerUserID); err != nil {
		return domainadvert.PublicDetail{}, apperr.Internal(fmt.Errorf("get published advert metadata: %w", pg.SanitizeErr(err)))
	}
	if desc != nil {
		out.Description = *desc
	}
	if addr != nil {
		out.Address = addr
	}
	if catName != nil {
		out.CategoryName = *catName
	}
	if catSlug != nil {
		out.CategorySlug = *catSlug
	}
	if distName != nil {
		out.DistrictName = *distName
	}
	if provName != nil {
		out.ProvinceName = *provName
	}
	if horseID != nil && horseName != nil {
		out.Horse = &domainadvert.PublicHorse{ID: *horseID, Name: *horseName, TJKNumber: horseTJKNumber}
	}
	if len(props) > 0 && string(props) != "null" {
		var propMap map[string]any
		if err := json.Unmarshal(props, &propMap); err == nil {
			if sp, ok := propMap["sellerPhone"].(string); ok && sp != "" {
				out.SellerPhone = &sp
			} else if ph, ok := propMap["phone"].(string); ok && ph != "" {
				out.SellerPhone = &ph
			}
		}
	}
	if out.SellerPhone == nil && userPhone != nil && *userPhone != "" {
		out.SellerPhone = userPhone
	}
	out.SellerID = ownerUserID

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
SELECT a.id, a.category_id, a.district_id, d.province_id, COALESCE(d.name, ''), COALESCE(p.name, ''), a.horse_id, a.title,
       a.price_amount_minor, a.price_currency, a.published_at,
       cover.asset_id, cover.display_order, cover.is_cover, cover.object_key,
       pkg.code, pkg.display_name, pkg.badge_text, COALESCE(pkg.search_priority, 0),
       (urgent.id IS NOT NULL), urgent.activated_at,
       (featured.id IS NOT NULL), featured.ends_at,
       CASE WHEN ` + actorArg + `::uuid IS NULL THEN NULL ELSE EXISTS (
           SELECT 1 FROM hrd_favorites f WHERE f.advert_id = a.id AND f.user_id = ` + actorArg + `
       ) END,
       a.view_count, a.properties,
       h.breed, h.gender, h.coat, h.birth_year
FROM hrd_adverts a
LEFT JOIN hrd_districts d ON d.id = a.district_id
LEFT JOIN hrd_provinces p ON p.id = d.province_id
LEFT JOIN hrd_horses h ON h.id = a.horse_id

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
  SELECT am.asset_id, am.display_order, am.is_cover, COALESCE(v.object_key, ma.master_object_key, ma.raw_object_key) AS object_key
  FROM hrd_advert_media am
  JOIN hrd_media_assets ma ON ma.id = am.asset_id AND ma.lifecycle_status NOT IN ('VALIDATION_FAILED', 'CLEANUP_CANDIDATE', 'DELETING', 'PHYSICALLY_DELETED')
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
	var rawProps []byte
	var hBreed *string
	var hGender *string
	var hCoat *string
	var hBirthYear *int
	if err := row.Scan(&card.ID, &card.CategoryID, &card.DistrictID, &card.ProvinceID, &card.DistrictName, &card.ProvinceName, &card.HorseID, &card.Title,
		&amount, &currency, &card.PublishedAt, &coverAsset, &coverOrder, &coverIsCover, &coverKey,
		&card.PackageCode, &card.PackageDisplayName, &card.PackageBadgeText, &card.SearchPriority,
		&card.IsUrgent, &card.UrgentActivatedAt, &card.IsFeatured, &card.FeaturedUntil, &card.IsFavorite, &card.ViewCount,
		&rawProps, &hBreed, &hGender, &hCoat, &hBirthYear); err != nil {
		return card, err
	}
	card.Properties = map[string]any{}
	if len(rawProps) > 0 && string(rawProps) != "null" {
		_ = json.Unmarshal(rawProps, &card.Properties)
	}
	if hBreed != nil && *hBreed != "" {
		if v, ok := card.Properties["HORSE_BREED"]; !ok || v == nil || v == "" {
			card.Properties["HORSE_BREED"] = *hBreed
		}
		if v, ok := card.Properties["STALLION_BREED"]; !ok || v == nil || v == "" {
			card.Properties["STALLION_BREED"] = *hBreed
		}
		if v, ok := card.Properties["breed"]; !ok || v == nil || v == "" {
			card.Properties["breed"] = *hBreed
		}
	}
	if hGender != nil && *hGender != "" {
		if v, ok := card.Properties["HORSE_GENDER"]; !ok || v == nil || v == "" {
			card.Properties["HORSE_GENDER"] = *hGender
		}
		if v, ok := card.Properties["gender"]; !ok || v == nil || v == "" {
			card.Properties["gender"] = *hGender
		}
	}
	if hCoat != nil && *hCoat != "" {
		if v, ok := card.Properties["COAT_COLOR"]; !ok || v == nil || v == "" {
			card.Properties["COAT_COLOR"] = *hCoat
		}
		if v, ok := card.Properties["coatColor"]; !ok || v == nil || v == "" {
			card.Properties["coatColor"] = *hCoat
		}
	}
	if hBirthYear != nil && *hBirthYear > 0 {
		age := time.Now().Year() - *hBirthYear
		if v, ok := card.Properties["HORSE_AGE"]; !ok || v == nil || v == 0 {
			card.Properties["HORSE_AGE"] = age
		}
		if v, ok := card.Properties["STALLION_AGE"]; !ok || v == nil || v == "" {
			if age >= 5 {
				card.Properties["STALLION_AGE"] = "5+"
			} else {
				card.Properties["STALLION_AGE"] = fmt.Sprintf("%d", age)
			}
		}
		if v, ok := card.Properties["age"]; !ok || v == nil || v == 0 {
			card.Properties["age"] = age
		}
		if v, ok := card.Properties["birthYear"]; !ok || v == nil || v == 0 {
			card.Properties["birthYear"] = *hBirthYear
		}
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
SELECT am.asset_id, am.display_order, am.is_cover, COALESCE(v.object_key, ma.master_object_key, ma.raw_object_key, '')
FROM hrd_advert_media am JOIN hrd_media_assets ma ON ma.id = am.asset_id AND ma.lifecycle_status NOT IN ('VALIDATION_FAILED', 'CLEANUP_CANDIDATE', 'DELETING', 'PHYSICALLY_DELETED')
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
	if len(raw) == 0 || string(raw) == "null" {
		return []domainadvert.PublicProperty{}, nil
	}
	var values map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		return []domainadvert.PublicProperty{}, nil
	}

	const q = `
WITH RECURSIVE cat_tree AS (
    SELECT c.id, c.parent_id, ARRAY[c.id] AS path, 0 AS depth
    FROM hrd_categories c
    JOIN hrd_adverts a ON a.category_id = c.id
    WHERE a.id = $1 AND c.is_active = true
    UNION ALL
    SELECT c.id, c.parent_id, t.path || c.id, t.depth + 1
    FROM hrd_categories c
    JOIN cat_tree t ON t.parent_id = c.id
    WHERE c.is_active = true AND NOT c.id = ANY(t.path)
)
SELECT cp.id, cp.code, cp.title, cp.data_type, cp.options, cp.sort_order, cp.is_active, cp.is_public_visible, ct.depth
FROM cat_tree ct
JOIN hrd_category_properties cp ON cp.category_id = ct.id
ORDER BY ct.depth ASC, cp.sort_order ASC, cp.code ASC, cp.id ASC`

	rows, err := r.db.Query(ctx, q, advertID)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list public properties: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()

	type propDef struct {
		id        uuid.UUID
		code      string
		title     string
		dataType  string
		options   []byte
		sortOrder int
	}

	var defs []propDef
	seenCodes := make(map[string]struct{})

	for rows.Next() {
		var (
			id              uuid.UUID
			code            string
			title           string
			dataType        string
			options         []byte
			sortOrder       int
			isActive        bool
			isPublicVisible bool
			depth           int
		)
		if err := rows.Scan(&id, &code, &title, &dataType, &options, &sortOrder, &isActive, &isPublicVisible, &depth); err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan public property: %w", pg.SanitizeErr(err)))
		}
		if _, seen := seenCodes[code]; seen {
			continue // child override: skip duplicate codes from higher ancestors
		}
		seenCodes[code] = struct{}{}

		if !isActive || !isPublicVisible {
			continue
		}

		defs = append(defs, propDef{
			id:        id,
			code:      code,
			title:     title,
			dataType:  dataType,
			options:   options,
			sortOrder: sortOrder,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate public properties: %w", pg.SanitizeErr(err)))
	}

	sort.SliceStable(defs, func(i, j int) bool {
		if defs[i].sortOrder != defs[j].sortOrder {
			return defs[i].sortOrder < defs[j].sortOrder
		}
		if defs[i].code != defs[j].code {
			return defs[i].code < defs[j].code
		}
		return defs[i].id.String() < defs[j].id.String()
	})

	out := make([]domainadvert.PublicProperty, 0, len(defs))
	for _, d := range defs {
		val, ok := values[d.code]
		if !ok || val == nil {
			continue
		}
		if strVal, isStr := val.(string); isStr {
			if strVal == "" || strVal == "null" || strVal == "undefined" {
				continue
			}
		}

		pubProp := domainadvert.PublicProperty{
			Code:  d.code,
			Title: d.title,
			Value: val,
		}

		if len(d.options) > 0 && string(d.options) != "null" && string(d.options) != "[]" {
			var optList []struct {
				Value string `json:"value"`
				Label string `json:"label"`
			}
			if err := json.Unmarshal(d.options, &optList); err == nil {
				strVal := fmt.Sprint(val)
				for _, opt := range optList {
					if opt.Value == strVal || opt.Label == strVal {
						lbl := opt.Label
						pubProp.DisplayValue = &lbl
						break
					}
				}
			}
		} else if bVal, isBool := val.(bool); isBool {
			var display string
			if bVal {
				display = "Evet"
			} else {
				display = "Hayır"
			}
			pubProp.DisplayValue = &display
		}

		out = append(out, pubProp)
	}
	return out, nil
}
