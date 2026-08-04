package favorite

import (
	"context"
	"time"

	"github.com/google/uuid"

	domainfavorite "github.com/hkizilbulak/haradan-be/internal/domain/favorite"
)

// AdvertSnapshot is the advert projection needed to decide list availability and
// to build a PublishedAdvertCard without inventing public media URLs.
type AdvertSnapshot struct {
	ID               uuid.UUID
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

// ListRow is one favorite joined with its advert snapshot.
type ListRow struct {
	Favorite domainfavorite.Favorite
	Advert   AdvertSnapshot
}

// Repository is the favorite persistence port.
type Repository interface {
	// FindAdvertForFavoriteLookup returns an advert by id for add-time
	// visibility checks. Missing rows are NotFound.
	FindAdvertForFavoriteLookup(ctx context.Context, advertID uuid.UUID) (AdvertSnapshot, error)

	// InsertFavorite inserts a relation. Duplicate (user,advert) must be reported
	// as ErrDuplicateFavorite so the application can treat it as idempotent success.
	InsertFavorite(ctx context.Context, fav domainfavorite.Favorite) error

	// DeleteFavorite removes the relation for the owning user. Missing rows are
	// not an error (idempotent remove).
	DeleteFavorite(ctx context.Context, userID, advertID uuid.UUID) error

	// ListFavoritesByUser returns favorites for one user in created_at DESC, id
	// DESC order, optionally after a keyset cursor, limited to limit rows.
	ListFavoritesByUser(
		ctx context.Context,
		userID uuid.UUID,
		afterCreatedAt *time.Time,
		afterID *uuid.UUID,
		limit int,
	) ([]ListRow, error)
}

// ErrDuplicateFavorite is returned by InsertFavorite on unique (user,advert).
var ErrDuplicateFavorite = domainfavorite.ErrDuplicate

// Clock supplies timestamps for inserts.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }
