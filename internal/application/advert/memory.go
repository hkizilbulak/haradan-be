package advert

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
	domaingeo "github.com/hkizilbulak/haradan-be/internal/domain/geo"
	domainhorse "github.com/hkizilbulak/haradan-be/internal/domain/horse"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

const memoryAdvertNotFound = "İlan bulunamadı."

// MemoryStore holds in-memory advert state and reference data for tests.
type MemoryStore struct {
	mu   sync.Mutex
	txMu sync.Mutex // serializes BeginTx..Commit like a coarse row lock

	adverts map[uuid.UUID]domainadvert.Advert
	history []domainadvert.StatusHistory

	categories map[uuid.UUID]domaincatalog.Category
	children   map[uuid.UUID]int
	formProps  map[uuid.UUID][]domaincatalog.Property
	districts  map[uuid.UUID]domaingeo.District
	horses     map[uuid.UUID]domainhorse.Horse
	users      map[uuid.UUID]domainuser.User
}

// NewMemoryStore builds an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		adverts:    map[uuid.UUID]domainadvert.Advert{},
		categories: map[uuid.UUID]domaincatalog.Category{},
		children:   map[uuid.UUID]int{},
		formProps:  map[uuid.UUID][]domaincatalog.Property{},
		districts:  map[uuid.UUID]domaingeo.District{},
		horses:     map[uuid.UUID]domainhorse.Horse{},
		users:      map[uuid.UUID]domainuser.User{},
	}
}

// PutAdvert seeds or replaces an advert.
func (s *MemoryStore) PutAdvert(a domainadvert.Advert) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adverts[a.ID] = a
}

// Advert returns a seeded advert.
func (s *MemoryStore) Advert(id uuid.UUID) (domainadvert.Advert, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.adverts[id]
	return a, ok
}

// History returns the recorded status history rows in insertion order.
func (s *MemoryStore) History() []domainadvert.StatusHistory {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domainadvert.StatusHistory(nil), s.history...)
}

// PutCategory seeds an active category with its child count and form properties.
func (s *MemoryStore) PutCategory(cat domaincatalog.Category, activeChildren int, props []domaincatalog.Property) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.categories[cat.ID] = cat
	s.children[cat.ID] = activeChildren
	s.formProps[cat.ID] = props
}

// PutDistrict seeds an active district.
func (s *MemoryStore) PutDistrict(d domaingeo.District) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.districts[d.ID] = d
}

// PutHorse seeds a horse.
func (s *MemoryStore) PutHorse(h domainhorse.Horse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.horses[h.ID] = h
}

// PutUser seeds a user account.
func (s *MemoryStore) PutUser(u domainuser.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[u.ID] = u
}

// Catalog returns the CatalogReader view of the store.
func (s *MemoryStore) Catalog() CatalogReader { return memoryCatalog{store: s} }

// Geo returns the GeoReader view of the store.
func (s *MemoryStore) Geo() GeoReader { return memoryGeo{store: s} }

// Horses returns the HorseReader view of the store.
func (s *MemoryStore) Horses() HorseReader { return memoryHorses{store: s} }

// Users returns the UserReader view of the store.
func (s *MemoryStore) Users() UserReader { return memoryUsers{store: s} }

// Repo returns the Repository view of the store.
func (s *MemoryStore) Repo() Repository { return MemoryRepository{store: s} }

// NewMemoryService builds an advert service backed entirely by the store.
func NewMemoryService(store *MemoryStore, clock Clock) (*Service, error) {
	return NewService(Config{
		Repo:    store.Repo(),
		Catalog: store.Catalog(),
		Geo:     store.Geo(),
		Horses:  store.Horses(),
		Users:   store.Users(),
		Clock:   clock,
	})
}

// MemoryRepository implements Repository against a MemoryStore.
type MemoryRepository struct {
	store *MemoryStore
}

// BeginTx starts a fake transaction that serializes concurrent callers.
func (r MemoryRepository) BeginTx(context.Context) (pgx.Tx, error) {
	r.store.txMu.Lock()
	return &memoryTx{store: r.store}, nil
}

// WithTx returns the same store-backed repository.
func (r MemoryRepository) WithTx(pgx.Tx) Repository { return r }

// Create inserts a new advert.
func (r MemoryRepository) Create(_ context.Context, a domainadvert.Advert) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if _, ok := r.store.adverts[a.ID]; ok {
		return apperr.Conflict("advert already exists")
	}
	r.store.adverts[a.ID] = a
	return nil
}

// InsertHistory appends an immutable status history row.
func (r MemoryRepository) InsertHistory(_ context.Context, h domainadvert.StatusHistory) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	r.store.history = append(r.store.history, h)
	return nil
}

// FindByIDForOwner returns an owner-scoped advert.
func (r MemoryRepository) FindByIDForOwner(_ context.Context, ownerID, advertID uuid.UUID) (domainadvert.Advert, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	return r.lookupLocked(ownerID, advertID)
}

// FindByIDForOwnerForUpdate behaves like FindByIDForOwner; the fake transaction
// already serializes writers.
func (r MemoryRepository) FindByIDForOwnerForUpdate(ctx context.Context, ownerID, advertID uuid.UUID) (domainadvert.Advert, error) {
	return r.FindByIDForOwner(ctx, ownerID, advertID)
}

// FindByID returns a non-deleted advert by id.
func (r MemoryRepository) FindByID(_ context.Context, advertID uuid.UUID) (domainadvert.Advert, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	a, ok := r.store.adverts[advertID]
	if !ok || a.IsDeleted() {
		return domainadvert.Advert{}, apperr.NotFound(memoryAdvertNotFound)
	}
	return a, nil
}

// FindByIDForUpdate behaves like FindByID under the fake transaction lock.
func (r MemoryRepository) FindByIDForUpdate(ctx context.Context, advertID uuid.UUID) (domainadvert.Advert, error) {
	return r.FindByID(ctx, advertID)
}

// ListForModeration returns non-deleted adverts matching status, newest first.
func (r MemoryRepository) ListForModeration(
	_ context.Context,
	status domainadvert.Status,
	afterCreated *time.Time,
	afterID *uuid.UUID,
	limit int,
) ([]domainadvert.Advert, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	var out []domainadvert.Advert
	for _, a := range r.store.adverts {
		if a.IsDeleted() || a.Status != status {
			continue
		}
		out = append(out, a)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID.String() > out[j].ID.String()
	})
	if afterCreated != nil && afterID != nil {
		filtered := out[:0:0]
		for _, a := range out {
			if a.CreatedAt.After(*afterCreated) {
				continue
			}
			if a.CreatedAt.Equal(*afterCreated) && a.ID.String() >= afterID.String() {
				continue
			}
			filtered = append(filtered, a)
		}
		out = filtered
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ListStatusHistory returns history for one advert, oldest first.
func (r MemoryRepository) ListStatusHistory(_ context.Context, advertID uuid.UUID) ([]domainadvert.StatusHistory, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	var out []domainadvert.StatusHistory
	for _, h := range r.store.history {
		if h.AdvertID == advertID {
			out = append(out, h)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	return out, nil
}

// ListMediaRelations returns no media in the memory store (tests seed empty).
func (r MemoryRepository) ListMediaRelations(_ context.Context, advertIDs []uuid.UUID) (map[uuid.UUID][]domainadvert.MediaRelation, error) {
	out := make(map[uuid.UUID][]domainadvert.MediaRelation, len(advertIDs))
	return out, nil
}

// ListByOwner returns non-deleted adverts newest first with keyset paging.
func (r MemoryRepository) ListByOwner(
	_ context.Context,
	ownerID uuid.UUID,
	status *domainadvert.Status,
	afterCreated *time.Time,
	afterID *uuid.UUID,
	limit int,
) ([]domainadvert.Advert, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	var out []domainadvert.Advert
	for _, a := range r.store.adverts {
		if a.OwnerUserID != ownerID || a.IsDeleted() {
			continue
		}
		if status != nil && a.Status != *status {
			continue
		}
		out = append(out, a)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID.String() > out[j].ID.String()
	})
	if afterCreated != nil && afterID != nil {
		filtered := out[:0:0]
		for _, a := range out {
			if a.CreatedAt.After(*afterCreated) {
				continue
			}
			if a.CreatedAt.Equal(*afterCreated) && a.ID.String() >= afterID.String() {
				continue
			}
			filtered = append(filtered, a)
		}
		out = filtered
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// UpdateDetails applies the patch when the version still matches.
func (r MemoryRepository) UpdateDetails(
	_ context.Context,
	ownerID, advertID uuid.UUID,
	patch domainadvert.DetailsPatch,
	expectedVersion int,
	now time.Time,
) (domainadvert.Advert, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	current, err := r.conditionLocked(ownerID, advertID, expectedVersion)
	if err != nil {
		return domainadvert.Advert{}, err
	}
	if !domainadvert.CanOwnerEditDetails(current.Status) {
		return domainadvert.Advert{}, apperr.StaleVersion(staleVersionMessage)
	}
	if patch.DistrictIDSet {
		current.DistrictID = patch.DistrictID
	}
	if patch.HorseIDSet {
		current.HorseID = patch.HorseID
	}
	if patch.TitleSet {
		current.Title = patch.Title
	}
	if patch.DescriptionSet {
		current.Description = patch.Description
	}
	if patch.PriceSet {
		current.Price = patch.Price
	}
	current.Version++
	current.UpdatedAt = now
	r.store.adverts[advertID] = current
	return current, nil
}

// UpdateCategoryClearProperties sets a new category and resets properties.
func (r MemoryRepository) UpdateCategoryClearProperties(
	_ context.Context,
	ownerID, advertID, categoryID uuid.UUID,
	expectedVersion int,
	now time.Time,
) (domainadvert.Advert, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	current, err := r.conditionLocked(ownerID, advertID, expectedVersion)
	if err != nil {
		return domainadvert.Advert{}, err
	}
	if current.Status != domainadvert.StatusDraft {
		return domainadvert.Advert{}, apperr.StaleVersion(staleVersionMessage)
	}
	id := categoryID
	current.CategoryID = &id
	current.Properties = domainadvert.EmptyProperties()
	current.Version++
	current.UpdatedAt = now
	r.store.adverts[advertID] = current
	return current, nil
}

// ReplaceProperties overwrites the dynamic property object.
func (r MemoryRepository) ReplaceProperties(
	_ context.Context,
	ownerID, advertID uuid.UUID,
	properties json.RawMessage,
	expectedVersion int,
	now time.Time,
) (domainadvert.Advert, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	current, err := r.conditionLocked(ownerID, advertID, expectedVersion)
	if err != nil {
		return domainadvert.Advert{}, err
	}
	if !domainadvert.CanOwnerEditDetails(current.Status) {
		return domainadvert.Advert{}, apperr.StaleVersion(staleVersionMessage)
	}
	current.Properties = append(json.RawMessage(nil), properties...)
	current.Version++
	current.UpdatedAt = now
	r.store.adverts[advertID] = current
	return current, nil
}

// SoftDeleteDraft stamps deleted_at on a DRAFT advert.
func (r MemoryRepository) SoftDeleteDraft(
	_ context.Context,
	ownerID, advertID uuid.UUID,
	expectedVersion int,
	now time.Time,
) (domainadvert.Advert, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	current, err := r.conditionLocked(ownerID, advertID, expectedVersion)
	if err != nil {
		return domainadvert.Advert{}, err
	}
	if current.Status != domainadvert.StatusDraft {
		return domainadvert.Advert{}, apperr.StaleVersion(staleVersionMessage)
	}
	deletedAt := now
	current.DeletedAt = &deletedAt
	current.Version++
	current.UpdatedAt = now
	r.store.adverts[advertID] = current
	return current, nil
}

// TransitionStatus moves the status when the from status and version match.
func (r MemoryRepository) TransitionStatus(
	_ context.Context,
	ownerID, advertID uuid.UUID,
	from, to domainadvert.Status,
	expectedVersion int,
	publishedAt *time.Time,
	now time.Time,
) (domainadvert.Advert, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	current, err := r.conditionLocked(ownerID, advertID, expectedVersion)
	if err != nil {
		return domainadvert.Advert{}, err
	}
	if current.Status != from {
		return domainadvert.Advert{}, apperr.StaleVersion(staleVersionMessage)
	}
	current.Status = to
	if publishedAt != nil {
		published := *publishedAt
		current.PublishedAt = &published
	}
	current.Version++
	current.UpdatedAt = now
	r.store.adverts[advertID] = current
	return current, nil
}

func (r MemoryRepository) lookupLocked(ownerID, advertID uuid.UUID) (domainadvert.Advert, error) {
	a, ok := r.store.adverts[advertID]
	if !ok || a.OwnerUserID != ownerID {
		return domainadvert.Advert{}, apperr.NotFound(memoryAdvertNotFound)
	}
	return a, nil
}

// conditionLocked mirrors the conditional SQL update guard: a missing row is
// NOT_FOUND, a version mismatch is STALE_VERSION.
func (r MemoryRepository) conditionLocked(ownerID, advertID uuid.UUID, expectedVersion int) (domainadvert.Advert, error) {
	current, err := r.lookupLocked(ownerID, advertID)
	if err != nil {
		return domainadvert.Advert{}, err
	}
	if current.IsDeleted() || current.Version != expectedVersion {
		return domainadvert.Advert{}, apperr.StaleVersion(staleVersionMessage)
	}
	return current, nil
}

type memoryCatalog struct{ store *MemoryStore }

func (c memoryCatalog) GetActiveCategory(_ context.Context, id uuid.UUID) (domaincatalog.Category, error) {
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	cat, ok := c.store.categories[id]
	if !ok || !cat.IsActive {
		return domaincatalog.Category{}, apperr.NotFound("Kategori bulunamadı.")
	}
	return cat, nil
}

func (c memoryCatalog) CountActiveChildren(_ context.Context, parentID uuid.UUID) (int, error) {
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	return c.store.children[parentID], nil
}

func (c memoryCatalog) ListFormProperties(_ context.Context, categoryID uuid.UUID) ([]domaincatalog.Property, error) {
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	return append([]domaincatalog.Property(nil), c.store.formProps[categoryID]...), nil
}

type memoryGeo struct{ store *MemoryStore }

func (g memoryGeo) GetActiveDistrict(_ context.Context, id uuid.UUID) (domaingeo.District, error) {
	g.store.mu.Lock()
	defer g.store.mu.Unlock()
	d, ok := g.store.districts[id]
	if !ok || !d.IsActive {
		return domaingeo.District{}, apperr.NotFound("İlçe bulunamadı.")
	}
	return d, nil
}

type memoryHorses struct{ store *MemoryStore }

func (h memoryHorses) FindByID(_ context.Context, id uuid.UUID) (domainhorse.Horse, error) {
	h.store.mu.Lock()
	defer h.store.mu.Unlock()
	horse, ok := h.store.horses[id]
	if !ok {
		return domainhorse.Horse{}, apperr.NotFound("At kaydı bulunamadı.")
	}
	return horse, nil
}

type memoryUsers struct{ store *MemoryStore }

func (u memoryUsers) FindByID(_ context.Context, id uuid.UUID) (domainuser.User, error) {
	u.store.mu.Lock()
	defer u.store.mu.Unlock()
	user, ok := u.store.users[id]
	if !ok {
		return domainuser.User{}, apperr.NotFound("user not found")
	}
	return user, nil
}

// memoryTx is a no-op pgx.Tx used by the in-memory repository.
type memoryTx struct {
	store *MemoryStore
	once  sync.Once
}

func (t *memoryTx) release() {
	t.once.Do(func() { t.store.txMu.Unlock() })
}

func (t *memoryTx) Begin(context.Context) (pgx.Tx, error) { return t, nil }
func (t *memoryTx) Commit(context.Context) error {
	t.release()
	return nil
}
func (t *memoryTx) Rollback(context.Context) error {
	t.release()
	return nil
}
func (t *memoryTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *memoryTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (t *memoryTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (t *memoryTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *memoryTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (t *memoryTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (t *memoryTx) QueryRow(context.Context, string, ...any) pgx.Row        { return nil }
func (t *memoryTx) Conn() *pgx.Conn                                         { return nil }
