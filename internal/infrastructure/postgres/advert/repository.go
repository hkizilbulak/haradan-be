package advert

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

	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

const (
	advertNotFoundMessage = "İlan bulunamadı."
	staleVersionMessage   = "İlan başka bir yerden güncellendi; sayfayı yenileyin."
)

const advertColumns = `id, owner_user_id, category_id, district_id, horse_id, title, description, address,
price_amount_minor, price_currency, status, properties, published_at, sold_at, version, media_version,
deleted_at, created_at, updated_at`

// Querier is implemented by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository persists adverts and advert status history.
type Repository struct {
	pool *pgxpool.Pool
	db   Querier
}

// NewRepository constructs an advert repository bound to a pool.
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
		return nil, apperr.Internal(errors.New("advert repository has no pool"))
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("begin advert tx: %w", pg.SanitizeErr(err)))
	}
	return tx, nil
}

// Create inserts a new advert row and assigns the generated id on a.
func (r *Repository) Create(ctx context.Context, a *domainadvert.Advert) error {
	const q = `
INSERT INTO hrd_adverts (
  owner_user_id, category_id, district_id, horse_id, title, description, address,
  price_amount_minor, price_currency, status, properties, published_at, sold_at, version, media_version,
  deleted_at, created_at, updated_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,$12,$13,$14,$15,$16,$17,$18
) RETURNING id`
	amount, currency := splitMoney(a.Price)
	err := r.db.QueryRow(ctx, q,
		a.OwnerUserID, a.CategoryID, a.DistrictID, a.HorseID, a.Title, a.Description, a.Address,
		amount, currency, string(a.Status), propertiesOrEmpty(a.Properties), a.PublishedAt, a.SoldAt,
		a.Version, a.MediaVersion, a.DeletedAt, a.CreatedAt, a.UpdatedAt,
	).Scan(&a.ID)
	if err != nil {
		return apperr.Internal(fmt.Errorf("create advert: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// InsertHistory appends an immutable status history row.
func (r *Repository) InsertHistory(ctx context.Context, h domainadvert.StatusHistory) error {
	const q = `
INSERT INTO hrd_advert_status_history (
  id, advert_id, from_status, to_status, actor_user_id, is_system, reason, created_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8
)`
	var from *string
	if h.FromStatus != nil {
		v := string(*h.FromStatus)
		from = &v
	}
	_, err := r.db.Exec(ctx, q,
		h.ID, h.AdvertID, from, string(h.ToStatus), h.ActorUserID, h.IsSystem, h.Reason, h.CreatedAt,
	)
	if err != nil {
		return apperr.Internal(fmt.Errorf("insert advert status history: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// FindByIDForOwner returns an owner-scoped advert. A foreign advert is reported
// as NOT_FOUND so ownership cannot be probed.
func (r *Repository) FindByIDForOwner(ctx context.Context, ownerID uuid.UUID, advertID int64) (domainadvert.Advert, error) {
	const q = `SELECT ` + advertColumns + ` FROM hrd_adverts WHERE id = $1 AND owner_user_id = $2`
	return r.queryOne(ctx, "find advert for owner", q, advertID, ownerID)
}

// FindByIDForOwnerForUpdate locks an owner-scoped advert row.
func (r *Repository) FindByIDForOwnerForUpdate(ctx context.Context, ownerID uuid.UUID, advertID int64) (domainadvert.Advert, error) {
	const q = `SELECT ` + advertColumns + ` FROM hrd_adverts WHERE id = $1 AND owner_user_id = $2 FOR UPDATE`
	return r.queryOne(ctx, "find advert for owner for update", q, advertID, ownerID)
}

// FindByID returns a non-deleted advert by id (admin scope).
func (r *Repository) FindByID(ctx context.Context, advertID int64) (domainadvert.Advert, error) {
	const q = `SELECT ` + advertColumns + ` FROM hrd_adverts WHERE id = $1 AND deleted_at IS NULL`
	return r.queryOne(ctx, "find advert by id", q, advertID)
}

// FindByIDForUpdate locks a non-deleted advert by id for admin transitions.
func (r *Repository) FindByIDForUpdate(ctx context.Context, advertID int64) (domainadvert.Advert, error) {
	const q = `SELECT ` + advertColumns + ` FROM hrd_adverts WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`
	return r.queryOne(ctx, "find advert by id for update", q, advertID)
}

// ListForModeration returns non-deleted adverts matching status (optional), newest first.
func (r *Repository) ListForModeration(
	ctx context.Context,
	status *domainadvert.Status,
	afterCreated *time.Time,
	afterID *int64,
	limit int,
) ([]domainadvert.Advert, int, error) {
	var (
		q    string
		args []any
	)
	if afterCreated != nil && afterID != nil {
		if status != nil {
			q = `SELECT ` + advertColumns + ` FROM hrd_adverts WHERE deleted_at IS NULL AND status = $1 AND (created_at, id) < ($2, $3) ORDER BY created_at DESC, id DESC LIMIT $4`
			args = []any{string(*status), *afterCreated, *afterID, limit}
		} else {
			q = `SELECT ` + advertColumns + ` FROM hrd_adverts WHERE deleted_at IS NULL AND (created_at, id) < ($1, $2) ORDER BY created_at DESC, id DESC LIMIT $3`
			args = []any{*afterCreated, *afterID, limit}
		}
	} else {
		if status != nil {
			q = `SELECT ` + advertColumns + ` FROM hrd_adverts WHERE deleted_at IS NULL AND status = $1 ORDER BY created_at DESC, id DESC LIMIT $2`
			args = []any{string(*status), limit}
		} else {
			q = `SELECT ` + advertColumns + ` FROM hrd_adverts WHERE deleted_at IS NULL ORDER BY created_at DESC, id DESC LIMIT $1`
			args = []any{limit}
		}
	}

	var totalCount int
	var countQ string
	var countArgs []any
	if status != nil {
		countQ = `SELECT count(*) FROM hrd_adverts WHERE deleted_at IS NULL AND status = $1`
		countArgs = []any{string(*status)}
	} else {
		countQ = `SELECT count(*) FROM hrd_adverts WHERE deleted_at IS NULL`
	}
	if err := r.db.QueryRow(ctx, countQ, countArgs...).Scan(&totalCount); err != nil {
		return nil, 0, apperr.Internal(fmt.Errorf("count moderation adverts: %w", pg.SanitizeErr(err)))
	}

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, apperr.Internal(fmt.Errorf("list moderation adverts: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()

	out := make([]domainadvert.Advert, 0, limit)
	for rows.Next() {
		a, err := scanAdvert(rows)
		if err != nil {
			return nil, 0, apperr.Internal(fmt.Errorf("scan moderation advert: %w", pg.SanitizeErr(err)))
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, apperr.Internal(fmt.Errorf("iterate moderation adverts: %w", pg.SanitizeErr(err)))
	}
	return out, totalCount, nil
}

// ListStatusHistory returns status history for one advert, oldest first.
func (r *Repository) ListStatusHistory(ctx context.Context, advertID int64) ([]domainadvert.StatusHistory, error) {
	const q = `
SELECT id, advert_id, from_status, to_status, actor_user_id, is_system, reason, created_at
FROM hrd_advert_status_history
WHERE advert_id = $1
ORDER BY created_at ASC, id ASC`

	rows, err := r.db.Query(ctx, q, advertID)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list advert status history: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()

	out := make([]domainadvert.StatusHistory, 0)
	for rows.Next() {
		var (
			h          domainadvert.StatusHistory
			fromStatus *string
			toStatus   string
		)
		if err := rows.Scan(
			&h.ID, &h.AdvertID, &fromStatus, &toStatus, &h.ActorUserID, &h.IsSystem, &h.Reason, &h.CreatedAt,
		); err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan advert status history: %w", pg.SanitizeErr(err)))
		}
		if fromStatus != nil {
			s := domainadvert.Status(*fromStatus)
			h.FromStatus = &s
		}
		h.ToStatus = domainadvert.Status(toStatus)
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate advert status history: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// ListMediaRelations returns advert/media links with asset lifecycle for owner views.
func (r *Repository) ListMediaRelations(ctx context.Context, advertIDs []int64) (map[int64][]domainadvert.MediaRelation, error) {
	out := make(map[int64][]domainadvert.MediaRelation, len(advertIDs))
	if len(advertIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `
SELECT am.advert_id, am.asset_id, am.display_order, am.is_cover, ma.lifecycle_status
FROM hrd_advert_media am
JOIN hrd_media_assets ma ON ma.id = am.asset_id
WHERE am.advert_id = ANY($1)
ORDER BY am.advert_id, am.display_order, am.asset_id`, advertIDs)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list owner advert media: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()
	for rows.Next() {
		var advertID int64
		var rel domainadvert.MediaRelation
		if err := rows.Scan(&advertID, &rel.AssetID, &rel.DisplayOrder, &rel.IsCover, &rel.LifecycleStatus); err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan owner advert media: %w", pg.SanitizeErr(err)))
		}
		out[advertID] = append(out[advertID], rel)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate owner advert media: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// ListByOwner returns non-deleted adverts newest first with keyset paging.
func (r *Repository) ListByOwner(
	ctx context.Context,
	ownerID uuid.UUID,
	status *domainadvert.Status,
	afterCreated *time.Time,
	afterID *int64,
	limit int,
) ([]domainadvert.Advert, error) {
	const q = `
SELECT ` + advertColumns + `
FROM hrd_adverts
WHERE owner_user_id = $1
  AND deleted_at IS NULL
  AND ($2::varchar IS NULL OR status = $2::varchar)
  AND (
    $3::timestamptz IS NULL
    OR (created_at, id) < ($3::timestamptz, $4::bigint)
  )
ORDER BY created_at DESC, id DESC
LIMIT $5`

	var statusArg *string
	if status != nil {
		v := string(*status)
		statusArg = &v
	}
	rows, err := r.db.Query(ctx, q, ownerID, statusArg, afterCreated, afterID, limit)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list owner adverts: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()

	out := make([]domainadvert.Advert, 0, limit)
	for rows.Next() {
		a, err := scanAdvert(rows)
		if err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan owner advert: %w", pg.SanitizeErr(err)))
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate owner adverts: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// UpdateDetails applies owner-editable core fields under an optimistic version
// guard. Callers hold the row lock, so zero rows means the guard lost a race.
func (r *Repository) UpdateDetails(
	ctx context.Context,
	ownerID uuid.UUID, advertID int64,
	patch domainadvert.DetailsPatch,
	expectedVersion int,
	now time.Time,
) (domainadvert.Advert, error) {
	const q = `
UPDATE hrd_adverts
SET district_id = CASE WHEN $4 THEN $5::uuid ELSE district_id END,
    horse_id = CASE WHEN $6 THEN $7::uuid ELSE horse_id END,
    properties = CASE WHEN $8 THEN $9::jsonb ELSE properties END,
    title = CASE WHEN $10 THEN $11::varchar ELSE title END,
    description = CASE WHEN $12 THEN $13::text ELSE description END,
    address = CASE WHEN $14 THEN $15::text ELSE address END,
    price_amount_minor = CASE WHEN $16 THEN $17::bigint ELSE price_amount_minor END,
    price_currency = CASE WHEN $16 THEN $18::varchar ELSE price_currency END,
    version = version + 1,
    updated_at = $19
WHERE id = $1
  AND owner_user_id = $2
  AND version = $3
  AND deleted_at IS NULL
  AND status IN ('DRAFT', 'CHANGES_REQUESTED', 'PUBLISHED')
RETURNING ` + advertColumns

	amount, currency := splitMoney(patch.Price)
	return r.updateOne(ctx, "update advert details", q,
		advertID, ownerID, expectedVersion,
		patch.DistrictIDSet, patch.DistrictID,
		patch.HorseIDSet, patch.HorseID,
		patch.PropertiesSet, patch.Properties,
		patch.TitleSet, patch.Title,
		patch.DescriptionSet, patch.Description,
		patch.AddressSet, patch.Address,
		patch.PriceSet, amount, currency,
		now,
	)
}

// UpdateCategoryClearProperties sets the category and resets properties to {}.
func (r *Repository) UpdateCategoryClearProperties(
	ctx context.Context,
	ownerID uuid.UUID, advertID int64, categoryID uuid.UUID,
	expectedVersion int,
	now time.Time,
) (domainadvert.Advert, error) {
	const q = `
UPDATE hrd_adverts
SET category_id = $4,
    properties = '{}'::jsonb,
    version = version + 1,
    updated_at = $5
WHERE id = $1
  AND owner_user_id = $2
  AND version = $3
  AND deleted_at IS NULL
  AND status = 'DRAFT'
RETURNING ` + advertColumns

	return r.updateOne(ctx, "change advert category", q, advertID, ownerID, expectedVersion, categoryID, now)
}

// ReplaceProperties overwrites the dynamic property object.
func (r *Repository) ReplaceProperties(
	ctx context.Context,
	ownerID uuid.UUID, advertID int64,
	properties json.RawMessage,
	expectedVersion int,
	now time.Time,
) (domainadvert.Advert, error) {
	const q = `
UPDATE hrd_adverts
SET properties = $4::jsonb,
    version = version + 1,
    updated_at = $5
WHERE id = $1
  AND owner_user_id = $2
  AND version = $3
  AND deleted_at IS NULL
  AND status IN ('DRAFT', 'CHANGES_REQUESTED', 'PUBLISHED')
RETURNING ` + advertColumns

	return r.updateOne(ctx, "replace advert properties", q,
		advertID, ownerID, expectedVersion, propertiesOrEmpty(properties), now)
}

// SoftDeleteDraft stamps deleted_at on a DRAFT advert.
func (r *Repository) SoftDeleteDraft(
	ctx context.Context,
	ownerID uuid.UUID, advertID int64,
	expectedVersion int,
	now time.Time,
) (domainadvert.Advert, error) {
	const q = `
UPDATE hrd_adverts
SET deleted_at = $4,
    version = version + 1,
    updated_at = $4
WHERE id = $1
  AND owner_user_id = $2
  AND version = $3
  AND deleted_at IS NULL
  AND status = 'DRAFT'
RETURNING ` + advertColumns

	return r.updateOne(ctx, "soft delete advert draft", q, advertID, ownerID, expectedVersion, now)
}

// TransitionStatus moves the status when owner, id, version and from status all
// still match. published_at is only overwritten when a value is supplied.
// sold_at is stamped automatically when transitioning to SOLD.
func (r *Repository) TransitionStatus(
	ctx context.Context,
	ownerID uuid.UUID, advertID int64,
	from, to domainadvert.Status,
	expectedVersion int,
	publishedAt *time.Time,
	now time.Time,
) (domainadvert.Advert, error) {
	// sold_at is set only on the first SOLD transition; other transitions leave it unchanged.
	var soldAt *time.Time
	if to == domainadvert.StatusSold {
		soldAt = &now
	}
	const q = `
UPDATE hrd_adverts
SET status = $5::varchar,
    published_at = COALESCE($6::timestamptz, published_at),
    sold_at = CASE WHEN $8::timestamptz IS NOT NULL THEN $8::timestamptz ELSE sold_at END,
    version = version + 1,
    updated_at = $7
WHERE id = $1
  AND owner_user_id = $2
  AND version = $3
  AND deleted_at IS NULL
  AND status = $4::varchar
RETURNING ` + advertColumns

	return r.updateOne(ctx, "transition advert status", q,
		advertID, ownerID, expectedVersion, string(from), string(to), publishedAt, now, soldAt)
}

// ListSoldForAutoArchive returns non-deleted SOLD adverts whose sold_at is
// older than the given cutoff. Used by the 24-hour auto-archive background job.
func (r *Repository) ListSoldForAutoArchive(ctx context.Context, soldBefore time.Time, limit int) ([]domainadvert.Advert, error) {
	const q = `
SELECT ` + advertColumns + `
FROM hrd_adverts
WHERE status = 'SOLD'
  AND deleted_at IS NULL
  AND sold_at IS NOT NULL
  AND sold_at < $1
ORDER BY sold_at ASC, id ASC
LIMIT $2`

	rows, err := r.db.Query(ctx, q, soldBefore, limit)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list sold for auto-archive: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()

	out := make([]domainadvert.Advert, 0, limit)
	for rows.Next() {
		a, err := scanAdvert(rows)
		if err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan sold advert: %w", pg.SanitizeErr(err)))
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate sold adverts: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// SystemTransitionStatus moves status when id, version and from status match
// (no owner filter). Used by background jobs such as auto-archive.
func (r *Repository) SystemTransitionStatus(
	ctx context.Context,
	advertID int64,
	from, to domainadvert.Status,
	expectedVersion int,
	now time.Time,
) (domainadvert.Advert, error) {
	const q = `
UPDATE hrd_adverts
SET status = $4::varchar,
    version = version + 1,
    updated_at = $5
WHERE id = $1
  AND version = $2
  AND deleted_at IS NULL
  AND status = $3::varchar
RETURNING ` + advertColumns

	return r.updateOne(ctx, "system transition advert status", q,
		advertID, expectedVersion, string(from), string(to), now)
}

func (r *Repository) queryOne(ctx context.Context, op, q string, args ...any) (domainadvert.Advert, error) {
	a, err := scanAdvert(r.db.QueryRow(ctx, q, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainadvert.Advert{}, apperr.NotFound(advertNotFoundMessage)
	}
	if err != nil {
		return domainadvert.Advert{}, apperr.Internal(fmt.Errorf("%s: %w", op, pg.SanitizeErr(err)))
	}
	return a, nil
}

func (r *Repository) updateOne(ctx context.Context, op, q string, args ...any) (domainadvert.Advert, error) {
	a, err := scanAdvert(r.db.QueryRow(ctx, q, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainadvert.Advert{}, apperr.StaleVersion(staleVersionMessage)
	}
	if err != nil {
		return domainadvert.Advert{}, apperr.Internal(fmt.Errorf("%s: %w", op, pg.SanitizeErr(err)))
	}
	return a, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAdvert(row rowScanner) (domainadvert.Advert, error) {
	var (
		a        domainadvert.Advert
		status   string
		amount   *int64
		currency *string
		props    []byte
	)
	if err := row.Scan(
		&a.ID, &a.OwnerUserID, &a.CategoryID, &a.DistrictID, &a.HorseID, &a.Title, &a.Description, &a.Address,
		&amount, &currency, &status, &props, &a.PublishedAt, &a.SoldAt, &a.Version, &a.MediaVersion,
		&a.DeletedAt, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return domainadvert.Advert{}, err
	}
	a.Status = domainadvert.Status(status)
	if amount != nil && currency != nil {
		a.Price = &domainadvert.Money{AmountMinor: *amount, Currency: *currency}
	}
	a.Properties = propertiesOrEmpty(props)
	return a, nil
}

func splitMoney(m *domainadvert.Money) (*int64, *string) {
	if m == nil {
		return nil, nil
	}
	amount := m.AmountMinor
	currency := m.Currency
	return &amount, &currency
}

func propertiesOrEmpty(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return domainadvert.EmptyProperties()
	}
	return json.RawMessage(raw)
}
