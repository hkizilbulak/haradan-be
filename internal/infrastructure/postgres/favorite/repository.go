// Package favorite implements PostgreSQL persistence for hrd_favorites.
package favorite

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainfavorite "github.com/hkizilbulak/haradan-be/internal/domain/favorite"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

// Querier is implemented by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository persists favorites with pgx.
type Repository struct {
	db Querier
}

// NewRepository constructs a PostgreSQL favorite repository.
func NewRepository(db Querier) *Repository {
	return &Repository{db: db}
}

// AdvertRow is the advert projection used by favorite lookups and list joins.
type AdvertRow struct {
	ID               int64
	Status           string
	DeletedAt        *time.Time
	Title            *string
	PublishedAt      *time.Time
	CategoryID       *uuid.UUID
	DistrictID       *uuid.UUID
	ProvinceID       *uuid.UUID
	HorseID          *uuid.UUID
	PriceAmountMinor *int64
	PriceCurrency    *string
}

// ListRow is one favorite joined with its advert row.
type ListRow struct {
	Favorite domainfavorite.Favorite
	Advert   AdvertRow
}

const advertLookupColumns = `id, status, deleted_at, title, published_at, category_id, district_id,
horse_id, price_amount_minor, price_currency`

// FindAdvertForFavoriteLookup returns advert fields needed for add-time checks.
func (r *Repository) FindAdvertForFavoriteLookup(ctx context.Context, advertID int64) (AdvertRow, error) {
	const q = `SELECT ` + advertLookupColumns + ` FROM hrd_adverts WHERE id = $1`
	var a AdvertRow
	err := r.db.QueryRow(ctx, q, advertID).Scan(
		&a.ID,
		&a.Status,
		&a.DeletedAt,
		&a.Title,
		&a.PublishedAt,
		&a.CategoryID,
		&a.DistrictID,
		&a.HorseID,
		&a.PriceAmountMinor,
		&a.PriceCurrency,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AdvertRow{}, apperr.NotFound("İlan bulunamadı.")
		}
		return AdvertRow{}, apperr.Internal(fmt.Errorf("find advert for favorite: %w", pg.SanitizeErr(err)))
	}
	return a, nil
}

// InsertFavorite inserts a favorite relation. Duplicate (user,advert) returns
// domainfavorite.ErrDuplicate.
func (r *Repository) InsertFavorite(ctx context.Context, fav domainfavorite.Favorite) error {
	const q = `
INSERT INTO hrd_favorites (id, user_id, advert_id, created_at)
VALUES ($1, $2, $3, $4)`
	_, err := r.db.Exec(ctx, q, fav.ID, fav.UserID, fav.AdvertID, fav.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domainfavorite.ErrDuplicate
		}
		return apperr.Internal(fmt.Errorf("insert favorite: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// DeleteFavorite deletes the owning user's relation; missing is success.
func (r *Repository) DeleteFavorite(ctx context.Context, userID uuid.UUID, advertID int64) error {
	const q = `DELETE FROM hrd_favorites WHERE user_id = $1 AND advert_id = $2`
	_, err := r.db.Exec(ctx, q, userID, advertID)
	if err != nil {
		return apperr.Internal(fmt.Errorf("delete favorite: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// ListFavoritesByUser returns favorites with advert + province enrichment.
func (r *Repository) ListFavoritesByUser(
	ctx context.Context,
	userID uuid.UUID,
	afterCreatedAt *time.Time,
	afterID *uuid.UUID,
	limit int,
) ([]ListRow, error) {
	const base = `
SELECT
  f.id, f.user_id, f.advert_id, f.created_at,
  a.id, a.status, a.deleted_at, a.title, a.published_at, a.category_id, a.district_id,
  d.province_id, a.horse_id, a.price_amount_minor, a.price_currency
FROM hrd_favorites f
JOIN hrd_adverts a ON a.id = f.advert_id
LEFT JOIN hrd_districts d ON d.id = a.district_id
WHERE f.user_id = $1`

	var (
		rows pgx.Rows
		err  error
	)
	if afterCreatedAt != nil && afterID != nil {
		const q = base + `
  AND (f.created_at, f.id) < ($2, $3)
ORDER BY f.created_at DESC, f.id DESC
LIMIT $4`
		rows, err = r.db.Query(ctx, q, userID, *afterCreatedAt, *afterID, limit)
	} else {
		const q = base + `
ORDER BY f.created_at DESC, f.id DESC
LIMIT $2`
		rows, err = r.db.Query(ctx, q, userID, limit)
	}
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list favorites: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()

	out := make([]ListRow, 0, limit)
	for rows.Next() {
		var row ListRow
		if err := rows.Scan(
			&row.Favorite.ID,
			&row.Favorite.UserID,
			&row.Favorite.AdvertID,
			&row.Favorite.CreatedAt,
			&row.Advert.ID,
			&row.Advert.Status,
			&row.Advert.DeletedAt,
			&row.Advert.Title,
			&row.Advert.PublishedAt,
			&row.Advert.CategoryID,
			&row.Advert.DistrictID,
			&row.Advert.ProvinceID,
			&row.Advert.HorseID,
			&row.Advert.PriceAmountMinor,
			&row.Advert.PriceCurrency,
		); err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan favorite row: %w", pg.SanitizeErr(err)))
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("iterate favorites: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
