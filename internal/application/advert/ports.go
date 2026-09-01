package advert

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
	domaingeo "github.com/hkizilbulak/haradan-be/internal/domain/geo"
	domainhorse "github.com/hkizilbulak/haradan-be/internal/domain/horse"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

// Repository persists adverts and their immutable status history.
//
// Every owner-scoped read/write is filtered by owner id so a foreign advert is
// indistinguishable from a missing one (NOT_FOUND, never an existence leak).
type Repository interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	WithTx(tx pgx.Tx) Repository

	Create(ctx context.Context, a *domainadvert.Advert) error
	InsertHistory(ctx context.Context, h domainadvert.StatusHistory) error

	FindByIDForOwner(ctx context.Context, ownerID uuid.UUID, advertID int64) (domainadvert.Advert, error)
	FindByIDForOwnerForUpdate(ctx context.Context, ownerID uuid.UUID, advertID int64) (domainadvert.Advert, error)

	// FindByID returns a non-deleted advert by id (admin scope; no ownership filter).
	FindByID(ctx context.Context, advertID int64) (domainadvert.Advert, error)
	// FindByIDForUpdate locks a non-deleted advert by id for admin transitions.
	FindByIDForUpdate(ctx context.Context, advertID int64) (domainadvert.Advert, error)

	ListByOwner(
		ctx context.Context,
		ownerID uuid.UUID,
		status *domainadvert.Status,
		afterCreated *time.Time,
		afterID *int64,
		limit int,
	) ([]domainadvert.Advert, error)

	// ListMediaRelations returns owner-visible media links for the given adverts.
	ListMediaRelations(ctx context.Context, advertIDs []int64) (map[int64][]domainadvert.MediaRelation, error)

	// ListForModeration returns non-deleted adverts matching status (optional) with keyset paging.
	ListForModeration(
		ctx context.Context,
		status *domainadvert.Status,
		afterCreated *time.Time,
		afterID *int64,
		limit int,
	) ([]domainadvert.Advert, int, error)

	// ListStatusHistory returns history for one advert, oldest first.
	ListStatusHistory(ctx context.Context, advertID int64) ([]domainadvert.StatusHistory, error)

	// UpdateDetails applies core content fields when owner+id+version still match
	// and the status is owner-editable.
	UpdateDetails(
		ctx context.Context,
		ownerID uuid.UUID, advertID int64,
		patch domainadvert.DetailsPatch,
		expectedVersion int,
		now time.Time,
	) (domainadvert.Advert, error)

	// UpdateCategoryClearProperties sets the category and resets properties to {}.
	UpdateCategoryClearProperties(
		ctx context.Context,
		ownerID uuid.UUID, advertID int64, categoryID uuid.UUID,
		expectedVersion int,
		now time.Time,
	) (domainadvert.Advert, error)

	// ReplaceProperties overwrites the dynamic property object.
	ReplaceProperties(
		ctx context.Context,
		ownerID uuid.UUID, advertID int64,
		properties json.RawMessage,
		expectedVersion int,
		now time.Time,
	) (domainadvert.Advert, error)

	// SoftDeleteDraft stamps deleted_at on a DRAFT advert.
	SoftDeleteDraft(
		ctx context.Context,
		ownerID uuid.UUID, advertID int64,
		expectedVersion int,
		now time.Time,
	) (domainadvert.Advert, error)

	// TransitionStatus moves status when owner+id+version+from status match.
	TransitionStatus(
		ctx context.Context,
		ownerID uuid.UUID, advertID int64,
		from, to domainadvert.Status,
		expectedVersion int,
		publishedAt *time.Time,
		now time.Time,
	) (domainadvert.Advert, error)

	// ListSoldForAutoArchive returns SOLD adverts sold before the cutoff.
	ListSoldForAutoArchive(ctx context.Context, soldBefore time.Time, limit int) ([]domainadvert.Advert, error)

	// SystemTransitionStatus moves status without owner filter (background jobs).
	SystemTransitionStatus(
		ctx context.Context,
		advertID int64,
		from, to domainadvert.Status,
		expectedVersion int,
		now time.Time,
	) (domainadvert.Advert, error)
}

// PublicRepository returns denormalized buyer-facing projections. It is kept
// separate from Repository because public reads deliberately join package,
// media, geography, and favorite state in one query.
type PublicRepository interface {
	SearchPublished(ctx context.Context, q domainadvert.PublicSearchQuery) ([]domainadvert.PublicCard, error)
	ListHomepageNew(ctx context.Context, q domainadvert.HomepageNewQuery) ([]domainadvert.PublicCard, error)
	ListHomepageShowcase(ctx context.Context, seed string, limit int, actorUserID *uuid.UUID) ([]domainadvert.PublicCard, error)
	ListHomepageUrgent(ctx context.Context, limit int, actorUserID *uuid.UUID) ([]domainadvert.PublicCard, error)
	ListHomepageFeatured(ctx context.Context, limit int, actorUserID *uuid.UUID) ([]domainadvert.PublicCard, error)
	GetPublishedDetail(ctx context.Context, advertID int64, actorUserID *uuid.UUID) (domainadvert.PublicDetail, error)
	RecordView(ctx context.Context, advertID int64, clientIP string) error
}

// CatalogReader reads the category metadata the advert core depends on.
type CatalogReader interface {
	GetActiveCategory(ctx context.Context, id uuid.UUID) (domaincatalog.Category, error)
	CountActiveChildren(ctx context.Context, parentID uuid.UUID) (int, error)
	ListFormProperties(ctx context.Context, categoryID uuid.UUID) ([]domaincatalog.Property, error)
}

// GeoReader resolves the advert district reference.
type GeoReader interface {
	GetActiveDistrict(ctx context.Context, id uuid.UUID) (domaingeo.District, error)
}

// HorseReader resolves the advert horse reference.
type HorseReader interface {
	FindByID(ctx context.Context, id uuid.UUID) (domainhorse.Horse, error)
}

// UserReader reads the owner account state required before moderation submit.
type UserReader interface {
	FindByID(ctx context.Context, id uuid.UUID) (domainuser.User, error)
}

// Clock provides the current time.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// MoneyInput is the raw price pair from a request; both halves or neither.
type MoneyInput struct {
	AmountMinor *int64
	Currency    *string
}
