package favorite

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainfavorite "github.com/hkizilbulak/haradan-be/internal/domain/favorite"
)

// MemoryStore is an in-memory favorite + advert fixture store for unit tests.
type MemoryStore struct {
	mu        sync.Mutex
	favorites map[uuid.UUID]domainfavorite.Favorite
	adverts   map[int64]AdvertSnapshot
	byPair    map[string]uuid.UUID
}

// NewMemoryStore builds an empty store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		favorites: map[uuid.UUID]domainfavorite.Favorite{},
		adverts:   map[int64]AdvertSnapshot{},
		byPair:    map[string]uuid.UUID{},
	}
}

// PutAdvert seeds an advert snapshot.
func (s *MemoryStore) PutAdvert(a AdvertSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adverts[a.ID] = a
}

// Favorites returns a copy of all favorites.
func (s *MemoryStore) Favorites() []domainfavorite.Favorite {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domainfavorite.Favorite, 0, len(s.favorites))
	for _, f := range s.favorites {
		out = append(out, f)
	}
	return out
}

func pairKey(userID uuid.UUID, advertID int64) string {
	return userID.String() + "|" + fmt.Sprintf("%d", advertID)
}

// MemoryRepository implements Repository against MemoryStore.
type MemoryRepository struct {
	store *MemoryStore
}

// NewMemoryRepository wraps a store.
func NewMemoryRepository(store *MemoryStore) MemoryRepository {
	return MemoryRepository{store: store}
}

// NewMemoryService builds a Service on an in-memory store.
func NewMemoryService(store *MemoryStore, clock Clock) (*Service, error) {
	return NewService(Config{Repo: NewMemoryRepository(store), Clock: clock})
}

func (r MemoryRepository) FindAdvertForFavoriteLookup(_ context.Context, advertID int64) (AdvertSnapshot, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	a, ok := r.store.adverts[advertID]
	if !ok {
		return AdvertSnapshot{}, apperr.NotFound(advertNotFoundMessage)
	}
	return a, nil
}

func (r MemoryRepository) InsertFavorite(_ context.Context, fav domainfavorite.Favorite) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	key := pairKey(fav.UserID, fav.AdvertID)
	if _, exists := r.store.byPair[key]; exists {
		return ErrDuplicateFavorite
	}
	r.store.favorites[fav.ID] = fav
	r.store.byPair[key] = fav.ID
	return nil
}

func (r MemoryRepository) DeleteFavorite(_ context.Context, userID uuid.UUID, advertID int64) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	key := pairKey(userID, advertID)
	id, ok := r.store.byPair[key]
	if !ok {
		return nil
	}
	delete(r.store.favorites, id)
	delete(r.store.byPair, key)
	return nil
}

func (r MemoryRepository) ListFavoritesByUser(
	_ context.Context,
	userID uuid.UUID,
	afterCreatedAt *time.Time,
	afterID *uuid.UUID,
	limit int,
) ([]ListRow, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	rows := make([]ListRow, 0)
	for _, fav := range r.store.favorites {
		if fav.UserID != userID {
			continue
		}
		if afterCreatedAt != nil && afterID != nil {
			// Keyset DESC: keep rows where (created_at, id) < (cursorCreated, cursorID).
			keep := fav.CreatedAt.Before(*afterCreatedAt) ||
				(fav.CreatedAt.Equal(*afterCreatedAt) && uuidLess(fav.ID, *afterID))
			if !keep {
				continue
			}
		}
		advert, ok := r.store.adverts[fav.AdvertID]
		if !ok {
			advert = AdvertSnapshot{ID: fav.AdvertID, Status: "ARCHIVED"}
		}
		rows = append(rows, ListRow{Favorite: fav, Advert: advert})
	}

	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].Favorite.CreatedAt.Equal(rows[j].Favorite.CreatedAt) {
			return rows[i].Favorite.CreatedAt.After(rows[j].Favorite.CreatedAt)
		}
		return rows[i].Favorite.ID.String() > rows[j].Favorite.ID.String()
	})
	if limit < len(rows) {
		rows = rows[:limit]
	}
	return rows, nil
}

func uuidLess(a, b uuid.UUID) bool {
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
