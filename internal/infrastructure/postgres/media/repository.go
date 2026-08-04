// Package media persists media assets, variants and advert media relations.
package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

const (
	assetNotFoundMessage    = "Görsel bulunamadı."
	variantNotFoundMessage  = "Görsel varyantı bulunamadı."
	advertNotFoundMessage   = "İlan bulunamadı."
	relationNotFoundMessage = "Bu görsel ilana ekli değil."
	stateChangedMessage     = "Görsel durumu değişti; tekrar deneyin."
	staleMediaVersion       = "İlan görselleri başka bir yerden güncellendi; sayfayı yenileyin."
	coverConflictMessage    = "İlanda zaten bir kapak görseli var."
	displayOrderTaken       = "Bu sıra numarası zaten kullanılıyor."
)

const assetColumns = `id, owner_user_id, provider, raw_object_key, master_object_key, content_type,
byte_size, checksum_sha256, width_px, height_px, lifecycle_status, technical_metadata, failure_reason,
created_at, updated_at`

const variantColumns = `id, asset_id, transform_profile, object_key, lifecycle_status, width_px,
height_px, byte_size, content_type, failure_reason, technical_metadata, created_at, updated_at`

// Querier is implemented by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository persists the media aggregate and its durable jobs.
type Repository struct {
	pool *pgxpool.Pool
	db   Querier
}

// NewRepository constructs a media repository bound to a pool.
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
		return nil, apperr.Internal(errors.New("media repository has no pool"))
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("begin media tx: %w", pg.SanitizeErr(err)))
	}
	return tx, nil
}

// CreateAsset inserts a new media asset row.
func (r *Repository) CreateAsset(ctx context.Context, a domainmedia.Asset) error {
	const q = `
INSERT INTO hrd_media_assets (
  id, owner_user_id, provider, raw_object_key, master_object_key, content_type,
  byte_size, checksum_sha256, width_px, height_px, lifecycle_status, technical_metadata,
  failure_reason, created_at, updated_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,$13,$14,$15
)`
	_, err := r.db.Exec(ctx, q,
		a.ID, a.OwnerUserID, providerOrDefault(a.Provider), a.RawObjectKey, a.MasterObjectKey,
		a.ContentType, a.ByteSize, a.ChecksumSHA256, a.WidthPx, a.HeightPx,
		string(a.LifecycleStatus), metadataOrEmpty(a.TechnicalMetadata), a.FailureReason,
		a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict("media asset already exists")
		}
		return apperr.Internal(fmt.Errorf("create media asset: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// FindAssetByIDForOwner returns an owner-scoped asset. A foreign asset is
// reported as NOT_FOUND so ownership cannot be probed.
func (r *Repository) FindAssetByIDForOwner(ctx context.Context, ownerID, assetID uuid.UUID) (domainmedia.Asset, error) {
	const q = `SELECT ` + assetColumns + ` FROM hrd_media_assets WHERE id = $1 AND owner_user_id = $2`
	return r.queryAsset(ctx, "find media asset for owner", q, assetID, ownerID)
}

// FindAssetByIDForOwnerForUpdate locks an owner-scoped asset row.
func (r *Repository) FindAssetByIDForOwnerForUpdate(ctx context.Context, ownerID, assetID uuid.UUID) (domainmedia.Asset, error) {
	const q = `SELECT ` + assetColumns + ` FROM hrd_media_assets WHERE id = $1 AND owner_user_id = $2 FOR UPDATE`
	return r.queryAsset(ctx, "find media asset for owner for update", q, assetID, ownerID)
}

// FindAssetByID returns an asset without owner scoping, for worker steps driven
// by a job payload rather than a session.
func (r *Repository) FindAssetByID(ctx context.Context, assetID uuid.UUID) (domainmedia.Asset, error) {
	const q = `SELECT ` + assetColumns + ` FROM hrd_media_assets WHERE id = $1`
	return r.queryAsset(ctx, "find media asset", q, assetID)
}

// FindAssetByIDForUpdate locks an asset row for a worker step.
func (r *Repository) FindAssetByIDForUpdate(ctx context.Context, assetID uuid.UUID) (domainmedia.Asset, error) {
	const q = `SELECT ` + assetColumns + ` FROM hrd_media_assets WHERE id = $1 FOR UPDATE`
	return r.queryAsset(ctx, "find media asset for update", q, assetID)
}

// UpdateAssetLifecycle moves an asset between two lifecycles when it still holds
// the expected one.
func (r *Repository) UpdateAssetLifecycle(
	ctx context.Context,
	assetID uuid.UUID,
	from, to domainmedia.AssetLifecycle,
	now time.Time,
) (domainmedia.Asset, error) {
	const q = `
UPDATE hrd_media_assets
SET lifecycle_status = $3::varchar,
    updated_at = $4
WHERE id = $1
  AND lifecycle_status = $2::varchar
RETURNING ` + assetColumns

	return r.updateAsset(ctx, "update media asset lifecycle", q, assetID, string(from), string(to), now)
}

// SetAssetUploaded moves UPLOAD_PENDING to UPLOADED and records the raw key,
// which the UPLOADED/VALIDATING table CHECK requires.
func (r *Repository) SetAssetUploaded(
	ctx context.Context,
	assetID uuid.UUID,
	rawObjectKey string,
	now time.Time,
) (domainmedia.Asset, error) {
	const q = `
UPDATE hrd_media_assets
SET raw_object_key = $2,
    lifecycle_status = 'UPLOADED',
    updated_at = $3
WHERE id = $1
  AND lifecycle_status = 'UPLOAD_PENDING'
RETURNING ` + assetColumns

	return r.updateAsset(ctx, "set media asset uploaded", q, assetID, rawObjectKey, now)
}

// SetAssetValidating moves UPLOADED to VALIDATING.
func (r *Repository) SetAssetValidating(ctx context.Context, assetID uuid.UUID, now time.Time) (domainmedia.Asset, error) {
	return r.UpdateAssetLifecycle(ctx, assetID, domainmedia.AssetUploaded, domainmedia.AssetValidating, now)
}

// SetAssetMasterReady records the canonical master together with every field the
// MASTER_READY CHECK requires.
func (r *Repository) SetAssetMasterReady(
	ctx context.Context,
	assetID uuid.UUID,
	masterObjectKey string,
	contentType string,
	byteSize int64,
	width, height int,
	now time.Time,
) (domainmedia.Asset, error) {
	const q = `
UPDATE hrd_media_assets
SET master_object_key = $2,
    content_type = $3,
    byte_size = $4,
    width_px = $5,
    height_px = $6,
    lifecycle_status = 'MASTER_READY',
    failure_reason = NULL,
    updated_at = $7
WHERE id = $1
  AND lifecycle_status IN ('UPLOADED', 'VALIDATING')
RETURNING ` + assetColumns

	return r.updateAsset(ctx, "set media asset master ready", q,
		assetID, masterObjectKey, contentType, byteSize, width, height, now)
}

// SetAssetValidationFailed records a terminal source-side failure. The reason is
// required by the VALIDATION_FAILED CHECK.
func (r *Repository) SetAssetValidationFailed(
	ctx context.Context,
	assetID uuid.UUID,
	reason string,
	now time.Time,
) (domainmedia.Asset, error) {
	const q = `
UPDATE hrd_media_assets
SET lifecycle_status = 'VALIDATION_FAILED',
    failure_reason = $2,
    updated_at = $3
WHERE id = $1
  AND lifecycle_status IN ('UPLOAD_PENDING', 'UPLOADED', 'VALIDATING')
RETURNING ` + assetColumns

	return r.updateAsset(ctx, "set media asset validation failed", q, assetID, reason, now)
}

// UpsertPendingVariant inserts a PENDING variant row and keeps an existing one,
// so the same master and profile are never duplicated.
func (r *Repository) UpsertPendingVariant(ctx context.Context, v domainmedia.Variant) (domainmedia.Variant, error) {
	const q = `
INSERT INTO hrd_media_variants (
  id, asset_id, transform_profile, object_key, lifecycle_status, width_px, height_px,
  byte_size, content_type, failure_reason, technical_metadata, created_at, updated_at
) VALUES (
  $1,$2,$3,NULL,'PENDING',NULL,NULL,NULL,NULL,NULL,$4::jsonb,$5,$5
)
ON CONFLICT (asset_id, transform_profile) DO NOTHING`

	if _, err := r.db.Exec(ctx, q,
		v.ID, v.AssetID, v.TransformProfile, metadataOrEmpty(v.TechnicalMetadata), v.CreatedAt,
	); err != nil {
		return domainmedia.Variant{}, apperr.Internal(fmt.Errorf("upsert media variant: %w", pg.SanitizeErr(err)))
	}

	const sel = `SELECT ` + variantColumns + `
FROM hrd_media_variants WHERE asset_id = $1 AND transform_profile = $2`
	return r.queryVariant(ctx, "read media variant", sel, v.AssetID, v.TransformProfile)
}

// ListVariantsByAsset returns the variants of one asset ordered by profile.
func (r *Repository) ListVariantsByAsset(ctx context.Context, assetID uuid.UUID) ([]domainmedia.Variant, error) {
	const q = `SELECT ` + variantColumns + `
FROM hrd_media_variants WHERE asset_id = $1 ORDER BY transform_profile`

	rows, err := r.db.Query(ctx, q, assetID)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list media variants: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()

	out := make([]domainmedia.Variant, 0, len(domainmedia.RequiredTransformProfiles()))
	for rows.Next() {
		v, err := scanVariant(rows)
		if err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan media variant: %w", pg.SanitizeErr(err)))
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate media variants: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// MarkVariantReady records a finished variant with every field the READY CHECK
// requires.
func (r *Repository) MarkVariantReady(
	ctx context.Context,
	assetID uuid.UUID,
	profile string,
	objectKey string,
	contentType string,
	byteSize int64,
	width, height int,
	now time.Time,
) (domainmedia.Variant, error) {
	const q = `
UPDATE hrd_media_variants
SET object_key = $3,
    content_type = $4,
    byte_size = $5,
    width_px = $6,
    height_px = $7,
    lifecycle_status = 'READY',
    failure_reason = NULL,
    updated_at = $8
WHERE asset_id = $1
  AND transform_profile = $2
  AND lifecycle_status IN ('PENDING', 'PROCESSING', 'FAILED')
RETURNING ` + variantColumns

	return r.updateVariant(ctx, "mark media variant ready", q,
		assetID, profile, objectKey, contentType, byteSize, width, height, now)
}

// MarkVariantFailed records a per-profile failure without touching the others.
func (r *Repository) MarkVariantFailed(
	ctx context.Context,
	assetID uuid.UUID,
	profile string,
	reason string,
	now time.Time,
) (domainmedia.Variant, error) {
	const q = `
UPDATE hrd_media_variants
SET lifecycle_status = 'FAILED',
    failure_reason = $3,
    updated_at = $4
WHERE asset_id = $1
  AND transform_profile = $2
  AND lifecycle_status IN ('PENDING', 'PROCESSING', 'FAILED')
RETURNING ` + variantColumns

	return r.updateVariant(ctx, "mark media variant failed", q, assetID, profile, reason, now)
}

// ListAdvertMediaByAdvert returns an advert's relations joined with the lifecycle
// of each asset, ordered by display order.
func (r *Repository) ListAdvertMediaByAdvert(ctx context.Context, advertID uuid.UUID) ([]domainmedia.RelationWithAsset, error) {
	const q = `
SELECT am.id, am.advert_id, am.asset_id, am.display_order, am.is_cover, am.created_at, am.updated_at,
       a.lifecycle_status
FROM hrd_advert_media am
JOIN hrd_media_assets a ON a.id = am.asset_id
WHERE am.advert_id = $1
ORDER BY am.display_order`

	rows, err := r.db.Query(ctx, q, advertID)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list advert media: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()

	var out []domainmedia.RelationWithAsset
	for rows.Next() {
		var (
			rel       domainmedia.AdvertMediaRelation
			lifecycle string
		)
		if err := rows.Scan(
			&rel.ID, &rel.AdvertID, &rel.AssetID, &rel.DisplayOrder, &rel.IsCover,
			&rel.CreatedAt, &rel.UpdatedAt, &lifecycle,
		); err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan advert media: %w", pg.SanitizeErr(err)))
		}
		out = append(out, domainmedia.RelationWithAsset{
			Relation:       rel,
			AssetLifecycle: domainmedia.AssetLifecycle(lifecycle),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate advert media: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// CountAdvertMediaByAdvert counts the relations of one advert.
func (r *Repository) CountAdvertMediaByAdvert(ctx context.Context, advertID uuid.UUID) (int, error) {
	const q = `SELECT COUNT(*) FROM hrd_advert_media WHERE advert_id = $1`
	var count int
	if err := r.db.QueryRow(ctx, q, advertID).Scan(&count); err != nil {
		return 0, apperr.Internal(fmt.Errorf("count advert media: %w", pg.SanitizeErr(err)))
	}
	return count, nil
}

// AttachAdvertMedia inserts a relation row.
func (r *Repository) AttachAdvertMedia(ctx context.Context, rel domainmedia.AdvertMediaRelation) error {
	const q = `
INSERT INTO hrd_advert_media (
  id, advert_id, asset_id, display_order, is_cover, created_at, updated_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7
)`
	_, err := r.db.Exec(ctx, q,
		rel.ID, rel.AdvertID, rel.AssetID, rel.DisplayOrder, rel.IsCover, rel.CreatedAt, rel.UpdatedAt,
	)
	if err != nil {
		// Any of the three unique constraints (advert+asset, advert+order, one
		// cover per advert) is a client-visible conflict, not an internal error.
		if isUniqueViolation(err) {
			return apperr.Conflict("Bu görsel ilana eklenemedi; listeyi yenileyin.")
		}
		return apperr.Internal(fmt.Errorf("attach advert media: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// DetachAdvertMedia removes a relation and reports whether it was the cover.
func (r *Repository) DetachAdvertMedia(ctx context.Context, advertID, assetID uuid.UUID) (bool, bool, error) {
	const q = `
DELETE FROM hrd_advert_media
WHERE advert_id = $1 AND asset_id = $2
RETURNING is_cover`

	var wasCover bool
	err := r.db.QueryRow(ctx, q, advertID, assetID).Scan(&wasCover)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, apperr.Internal(fmt.Errorf("detach advert media: %w", pg.SanitizeErr(err)))
	}
	return true, wasCover, nil
}

// UpdateAdvertMediaDisplayOrder rewrites one relation's display order.
func (r *Repository) UpdateAdvertMediaDisplayOrder(
	ctx context.Context,
	advertID, assetID uuid.UUID,
	displayOrder int,
	now time.Time,
) error {
	const q = `
UPDATE hrd_advert_media
SET display_order = $3,
    updated_at = $4
WHERE advert_id = $1 AND asset_id = $2`

	tag, err := r.db.Exec(ctx, q, advertID, assetID, displayOrder, now)
	if err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict(displayOrderTaken)
		}
		return apperr.Internal(fmt.Errorf("update advert media order: %w", pg.SanitizeErr(err)))
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound(relationNotFoundMessage)
	}
	return nil
}

// ClearAdvertCover unsets the cover flag so a new one can be set in the same
// transaction without tripping the one-cover partial unique index.
func (r *Repository) ClearAdvertCover(ctx context.Context, advertID uuid.UUID, now time.Time) error {
	const q = `
UPDATE hrd_advert_media
SET is_cover = false,
    updated_at = $2
WHERE advert_id = $1 AND is_cover = true`

	if _, err := r.db.Exec(ctx, q, advertID, now); err != nil {
		return apperr.Internal(fmt.Errorf("clear advert cover: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// SetAdvertCover flags one relation as the cover.
func (r *Repository) SetAdvertCover(ctx context.Context, advertID, assetID uuid.UUID, now time.Time) error {
	const q = `
UPDATE hrd_advert_media
SET is_cover = true,
    updated_at = $3
WHERE advert_id = $1 AND asset_id = $2`

	tag, err := r.db.Exec(ctx, q, advertID, assetID, now)
	if err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict(coverConflictMessage)
		}
		return apperr.Internal(fmt.Errorf("set advert cover: %w", pg.SanitizeErr(err)))
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound(relationNotFoundMessage)
	}
	return nil
}

// FindOwnerAdvertForUpdate locks the owner's advert and returns only the fields
// the media domain is allowed to read.
func (r *Repository) FindOwnerAdvertForUpdate(ctx context.Context, ownerID, advertID uuid.UUID) (domainmedia.AdvertRef, error) {
	const q = `
SELECT id, status, media_version, deleted_at
FROM hrd_adverts
WHERE id = $1 AND owner_user_id = $2
FOR UPDATE`

	var ref domainmedia.AdvertRef
	err := r.db.QueryRow(ctx, q, advertID, ownerID).Scan(&ref.ID, &ref.Status, &ref.MediaVersion, &ref.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainmedia.AdvertRef{}, apperr.NotFound(advertNotFoundMessage)
	}
	if err != nil {
		return domainmedia.AdvertRef{}, apperr.Internal(fmt.Errorf("find owner advert for media: %w", pg.SanitizeErr(err)))
	}
	return ref, nil
}

// BumpAdvertMediaVersion increments media_version under an optimistic guard and
// only while the advert is still open to media edits.
func (r *Repository) BumpAdvertMediaVersion(
	ctx context.Context,
	ownerID, advertID uuid.UUID,
	expectedMediaVersion int,
	now time.Time,
) (int, error) {
	const q = `
UPDATE hrd_adverts
SET media_version = media_version + 1,
    updated_at = $4
WHERE id = $1
  AND owner_user_id = $2
  AND media_version = $3
  AND deleted_at IS NULL
  AND status IN ('DRAFT', 'CHANGES_REQUESTED')
RETURNING media_version`

	var newVersion int
	err := r.db.QueryRow(ctx, q, advertID, ownerID, expectedMediaVersion, now).Scan(&newVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, apperr.StaleVersion(staleMediaVersion)
	}
	if err != nil {
		return 0, apperr.Internal(fmt.Errorf("bump advert media version: %w", pg.SanitizeErr(err)))
	}
	return newVersion, nil
}

func (r *Repository) queryAsset(ctx context.Context, op, q string, args ...any) (domainmedia.Asset, error) {
	a, err := scanAsset(r.db.QueryRow(ctx, q, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainmedia.Asset{}, apperr.NotFound(assetNotFoundMessage)
	}
	if err != nil {
		return domainmedia.Asset{}, apperr.Internal(fmt.Errorf("%s: %w", op, pg.SanitizeErr(err)))
	}
	return a, nil
}

// updateAsset treats zero rows as a lost race: the caller holds the row lock, so
// a missed guard means the lifecycle moved underneath it.
func (r *Repository) updateAsset(ctx context.Context, op, q string, args ...any) (domainmedia.Asset, error) {
	a, err := scanAsset(r.db.QueryRow(ctx, q, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainmedia.Asset{}, apperr.InvalidState(stateChangedMessage)
	}
	if err != nil {
		return domainmedia.Asset{}, apperr.Internal(fmt.Errorf("%s: %w", op, pg.SanitizeErr(err)))
	}
	return a, nil
}

func (r *Repository) queryVariant(ctx context.Context, op, q string, args ...any) (domainmedia.Variant, error) {
	v, err := scanVariant(r.db.QueryRow(ctx, q, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainmedia.Variant{}, apperr.NotFound(variantNotFoundMessage)
	}
	if err != nil {
		return domainmedia.Variant{}, apperr.Internal(fmt.Errorf("%s: %w", op, pg.SanitizeErr(err)))
	}
	return v, nil
}

func (r *Repository) updateVariant(ctx context.Context, op, q string, args ...any) (domainmedia.Variant, error) {
	v, err := scanVariant(r.db.QueryRow(ctx, q, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainmedia.Variant{}, apperr.InvalidState(stateChangedMessage)
	}
	if err != nil {
		return domainmedia.Variant{}, apperr.Internal(fmt.Errorf("%s: %w", op, pg.SanitizeErr(err)))
	}
	return v, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAsset(row rowScanner) (domainmedia.Asset, error) {
	var (
		a         domainmedia.Asset
		lifecycle string
		metadata  []byte
	)
	if err := row.Scan(
		&a.ID, &a.OwnerUserID, &a.Provider, &a.RawObjectKey, &a.MasterObjectKey, &a.ContentType,
		&a.ByteSize, &a.ChecksumSHA256, &a.WidthPx, &a.HeightPx, &lifecycle, &metadata,
		&a.FailureReason, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return domainmedia.Asset{}, err
	}
	a.LifecycleStatus = domainmedia.AssetLifecycle(lifecycle)
	a.TechnicalMetadata = metadataOrEmpty(metadata)
	return a, nil
}

func scanVariant(row rowScanner) (domainmedia.Variant, error) {
	var (
		v         domainmedia.Variant
		lifecycle string
		metadata  []byte
	)
	if err := row.Scan(
		&v.ID, &v.AssetID, &v.TransformProfile, &v.ObjectKey, &lifecycle, &v.WidthPx,
		&v.HeightPx, &v.ByteSize, &v.ContentType, &v.FailureReason, &metadata,
		&v.CreatedAt, &v.UpdatedAt,
	); err != nil {
		return domainmedia.Variant{}, err
	}
	v.LifecycleStatus = domainmedia.VariantLifecycle(lifecycle)
	v.TechnicalMetadata = metadataOrEmpty(metadata)
	return v, nil
}

func providerOrDefault(provider string) string {
	if provider == "" {
		return domainmedia.ProviderB2
	}
	return provider
}

func metadataOrEmpty(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return domainmedia.EmptyMetadata()
	}
	return json.RawMessage(raw)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
