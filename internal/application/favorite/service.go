// Package favorite implements FAVORITE-01..03.
package favorite

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainfavorite "github.com/hkizilbulak/haradan-be/internal/domain/favorite"
)

const (
	minPageLimit     = 1
	maxPageLimit     = 100
	defaultPageLimit = 20

	advertNotFoundMessage    = "İlan bulunamadı."
	unavailableReasonMessage = "Bu ilan artık kullanılamıyor."
	invalidCursorMessage     = "Geçersiz sayfalama imleci."
)

// Service implements the favorite use cases.
type Service struct {
	repo  Repository
	clock Clock
}

// Config wires favorite application dependencies.
type Config struct {
	Repo  Repository
	Clock Clock
}

// NewService constructs the favorite application service.
func NewService(cfg Config) (*Service, error) {
	if cfg.Repo == nil {
		return nil, fmt.Errorf("favorite service repository is required")
	}
	clock := cfg.Clock
	if clock == nil {
		clock = systemClock{}
	}
	return &Service{repo: cfg.Repo, clock: clock}, nil
}

// MutationView is FAVORITE-01/02 output.
type MutationView struct {
	AdvertID  uuid.UUID
	Favorited bool
}

// MoneyView is the card price pair.
type MoneyView struct {
	AmountMinor int64
	Currency    string
}

// CardView is the public-safe card for an available favorite. Cover stays nil
// until public media URL projection exists; publicUrl is never invented here.
type CardView struct {
	ID          uuid.UUID
	Title       string
	PublishedAt time.Time
	CategoryID  uuid.UUID
	DistrictID  uuid.UUID
	ProvinceID  uuid.UUID
	HorseID     *uuid.UUID
	Price       *MoneyView
	IsFavorite  bool
}

// ListItemView is one FavoriteListItem.
type ListItemView struct {
	AdvertID          uuid.UUID
	Available         bool
	Card              *CardView
	UnavailableReason *string
}

// ListInput is FAVORITE-03 input.
type ListInput struct {
	Cursor *string
	Limit  *int
}

// ListResult is FAVORITE-03 output.
type ListResult struct {
	Items      []ListItemView
	NextCursor *string
	HasMore    bool
}

// AddFavorite implements FAVORITE-01. Duplicate inserts are idempotent success.
// Missing, soft-deleted and non-PUBLISHED adverts all surface as the same
// NOT_FOUND to the client so resource existence cannot be probed.
func (s *Service) AddFavorite(ctx context.Context, userID, advertID uuid.UUID) (MutationView, error) {
	if err := requireAdvertID(advertID); err != nil {
		return MutationView{}, err
	}
	advert, err := s.repo.FindAdvertForFavoriteLookup(ctx, advertID)
	if err != nil {
		return MutationView{}, err
	}
	if !isFavoritableAdvert(advert) {
		return MutationView{}, apperr.NotFound(advertNotFoundMessage)
	}

	now := s.clock.Now()
	err = s.repo.InsertFavorite(ctx, domainfavorite.Favorite{
		ID:        uuid.New(),
		UserID:    userID,
		AdvertID:  advertID,
		CreatedAt: now,
	})
	if err != nil {
		if errors.Is(err, ErrDuplicateFavorite) {
			return MutationView{AdvertID: advertID, Favorited: true}, nil
		}
		return MutationView{}, err
	}
	return MutationView{AdvertID: advertID, Favorited: true}, nil
}

// RemoveFavorite implements FAVORITE-02. Missing relations are idempotent success.
func (s *Service) RemoveFavorite(ctx context.Context, userID, advertID uuid.UUID) (MutationView, error) {
	if err := requireAdvertID(advertID); err != nil {
		return MutationView{}, err
	}
	if err := s.repo.DeleteFavorite(ctx, userID, advertID); err != nil {
		return MutationView{}, err
	}
	return MutationView{AdvertID: advertID, Favorited: false}, nil
}

// ListMyFavorites implements FAVORITE-03. Non-public adverts stay in the list as
// safe placeholders; the favorite relation is never deleted by this read.
func (s *Service) ListMyFavorites(ctx context.Context, userID uuid.UUID, in ListInput) (ListResult, error) {
	limit, err := resolveLimit(in.Limit)
	if err != nil {
		return ListResult{}, err
	}

	var afterCreated *time.Time
	var afterID *uuid.UUID
	if in.Cursor != nil && strings.TrimSpace(*in.Cursor) != "" {
		created, id, err := decodeFavoriteCursor(strings.TrimSpace(*in.Cursor))
		if err != nil {
			return ListResult{}, err
		}
		afterCreated = &created
		afterID = &id
	}

	rows, err := s.repo.ListFavoritesByUser(ctx, userID, afterCreated, afterID, limit+1)
	if err != nil {
		return ListResult{}, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	items := make([]ListItemView, 0, len(rows))
	for _, row := range rows {
		items = append(items, projectListItem(row))
	}

	out := ListResult{Items: items, HasMore: hasMore}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		cursor := encodeFavoriteCursor(last.Favorite.CreatedAt, last.Favorite.ID)
		out.NextCursor = &cursor
	}
	return out, nil
}

func projectListItem(row ListRow) ListItemView {
	item := ListItemView{AdvertID: row.Favorite.AdvertID}
	if !isPubliclyAvailable(row.Advert) {
		reason := unavailableReasonMessage
		item.Available = false
		item.UnavailableReason = &reason
		return item
	}
	card, ok := buildCard(row.Advert)
	if !ok {
		reason := unavailableReasonMessage
		item.Available = false
		item.UnavailableReason = &reason
		return item
	}
	item.Available = true
	item.Card = &card
	return item
}

func isFavoritableAdvert(a AdvertSnapshot) bool {
	return a.DeletedAt == nil && a.Status == string(domainadvert.StatusPublished)
}

func isPubliclyAvailable(a AdvertSnapshot) bool {
	return isFavoritableAdvert(a)
}

func buildCard(a AdvertSnapshot) (CardView, bool) {
	if a.Title == nil || strings.TrimSpace(*a.Title) == "" {
		return CardView{}, false
	}
	if a.PublishedAt == nil || a.CategoryID == nil || a.DistrictID == nil || a.ProvinceID == nil {
		return CardView{}, false
	}
	card := CardView{
		ID:          a.ID,
		Title:       *a.Title,
		PublishedAt: a.PublishedAt.UTC(),
		CategoryID:  *a.CategoryID,
		DistrictID:  *a.DistrictID,
		ProvinceID:  *a.ProvinceID,
		HorseID:     a.HorseID,
		IsFavorite:  true,
	}
	if a.PriceAmountMinor != nil && a.PriceCurrency != nil {
		card.Price = &MoneyView{AmountMinor: *a.PriceAmountMinor, Currency: *a.PriceCurrency}
	}
	return card, true
}

func requireAdvertID(id uuid.UUID) error {
	if id == uuid.Nil {
		return apperr.Validation("Geçersiz istek.", apperr.FieldError{
			Field:   "advertId",
			Message: "İlan zorunludur.",
		})
	}
	return nil
}

func resolveLimit(limit *int) (int, error) {
	if limit == nil {
		return defaultPageLimit, nil
	}
	if *limit < minPageLimit || *limit > maxPageLimit {
		return 0, apperr.Validation("Geçersiz istek.", apperr.FieldError{
			Field:   "limit",
			Message: "Sayfa boyutu 1 ile 100 arasında olmalıdır.",
		})
	}
	return *limit, nil
}

func encodeFavoriteCursor(createdAt time.Time, id uuid.UUID) string {
	raw := createdAt.UTC().Format(time.RFC3339Nano) + "|" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeFavoriteCursor(cursor string) (time.Time, uuid.UUID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, apperr.Validation(invalidCursorMessage, apperr.FieldError{
			Field:   "cursor",
			Message: invalidCursorMessage,
		})
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, apperr.Validation(invalidCursorMessage, apperr.FieldError{
			Field:   "cursor",
			Message: invalidCursorMessage,
		})
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, apperr.Validation(invalidCursorMessage, apperr.FieldError{
			Field:   "cursor",
			Message: invalidCursorMessage,
		})
	}
	id, err := uuid.Parse(parts[1])
	if err != nil || id == uuid.Nil {
		return time.Time{}, uuid.Nil, apperr.Validation(invalidCursorMessage, apperr.FieldError{
			Field:   "cursor",
			Message: invalidCursorMessage,
		})
	}
	return createdAt, id, nil
}
