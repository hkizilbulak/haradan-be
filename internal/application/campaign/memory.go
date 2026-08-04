package campaign

import (
	"context"
	"sort"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincampaign "github.com/hkizilbulak/haradan-be/internal/domain/campaign"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

// MemoryStore holds in-memory campaign state for unit tests.
type MemoryStore struct {
	mu   sync.Mutex
	txMu sync.Mutex

	campaigns map[uuid.UUID]domaincampaign.Campaign
	packages  map[uuid.UUID]domainpackaging.Package
	assets    map[uuid.UUID]domainmedia.Asset
	users     map[uuid.UUID]domainuser.User
}

// NewMemoryStore builds an empty campaign memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		campaigns: map[uuid.UUID]domaincampaign.Campaign{},
		packages:  map[uuid.UUID]domainpackaging.Package{},
		assets:    map[uuid.UUID]domainmedia.Asset{},
		users:     map[uuid.UUID]domainuser.User{},
	}
}

func (s *MemoryStore) PutPackage(p domainpackaging.Package) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.packages[p.ID] = p
}

func (s *MemoryStore) PutAsset(a domainmedia.Asset) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.assets[a.ID] = a
}

func (s *MemoryStore) PutUser(u domainuser.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[u.ID] = u
}

func (s *MemoryStore) PutCampaign(c domaincampaign.Campaign) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.campaigns[c.ID] = c
}

func (s *MemoryStore) Repo() Repository        { return memoryRepo{store: s} }
func (s *MemoryStore) Packages() PackageLookup { return memoryPackages{store: s} }
func (s *MemoryStore) Assets() AssetLookup     { return memoryAssets{store: s} }
func (s *MemoryStore) Users() UserReader       { return memoryUsers{store: s} }

// NewMemoryService builds a campaign service backed by the store.
func NewMemoryService(store *MemoryStore, clock Clock) (*Service, error) {
	return NewService(Config{
		Repo:     store.Repo(),
		Packages: store.Packages(),
		Assets:   store.Assets(),
		Users:    store.Users(),
		Clock:    clock,
	})
}

type memoryRepo struct{ store *MemoryStore }

func (m memoryRepo) BeginTx(context.Context) (pgx.Tx, error) {
	m.store.txMu.Lock()
	return &memoryTx{store: m.store}, nil
}

func (m memoryRepo) WithTx(pgx.Tx) Repository { return m }

func (m memoryRepo) Create(_ context.Context, c domaincampaign.Campaign) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	for _, existing := range m.store.campaigns {
		if existing.Code == c.Code {
			return apperr.Conflict(campaignConflictMessage)
		}
	}
	m.store.campaigns[c.ID] = c
	return nil
}

func (m memoryRepo) GetByID(_ context.Context, id uuid.UUID) (domaincampaign.Campaign, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	c, ok := m.store.campaigns[id]
	if !ok {
		return domaincampaign.Campaign{}, apperr.NotFound(campaignNotFoundMessage)
	}
	return c, nil
}

func (m memoryRepo) List(_ context.Context, f ListFilter) ([]domaincampaign.Campaign, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	rows := make([]domaincampaign.Campaign, 0)
	for _, c := range m.store.campaigns {
		if f.EventType != nil && c.EventType != *f.EventType {
			continue
		}
		if f.IsActive != nil && c.IsActive != *f.IsActive {
			continue
		}
		if f.SourcePackageID != nil {
			if c.SourcePackageID == nil || *c.SourcePackageID != *f.SourcePackageID {
				continue
			}
		}
		if f.TargetPackageID != nil {
			if c.TargetPackageID == nil || *c.TargetPackageID != *f.TargetPackageID {
				continue
			}
		}
		if f.AfterCreatedAt != nil && f.AfterID != nil {
			if c.CreatedAt.After(*f.AfterCreatedAt) {
				continue
			}
			if c.CreatedAt.Equal(*f.AfterCreatedAt) && !uuidLess(c.ID, *f.AfterID) {
				continue
			}
		}
		rows = append(rows, c)
	}
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].CreatedAt.After(rows[j].CreatedAt)
		}
		return uuidLess(rows[j].ID, rows[i].ID)
	})
	if f.Limit > 0 && len(rows) > f.Limit {
		rows = rows[:f.Limit]
	}
	return rows, nil
}

func (m memoryRepo) LockByID(ctx context.Context, id uuid.UUID) (domaincampaign.Campaign, error) {
	return m.GetByID(ctx, id)
}

func (m memoryRepo) UpdateOptimistic(_ context.Context, c domaincampaign.Campaign, expectedVersion int) (domaincampaign.Campaign, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	current, ok := m.store.campaigns[c.ID]
	if !ok {
		return domaincampaign.Campaign{}, apperr.NotFound(campaignNotFoundMessage)
	}
	if current.Version != expectedVersion {
		return domaincampaign.Campaign{}, apperr.StaleVersion(staleVersionMessage)
	}
	for _, existing := range m.store.campaigns {
		if existing.ID != c.ID && existing.Code == c.Code {
			return domaincampaign.Campaign{}, apperr.Conflict(campaignConflictMessage)
		}
	}
	c.Version = expectedVersion + 1
	m.store.campaigns[c.ID] = c
	return c, nil
}

type memoryPackages struct{ store *MemoryStore }

func (m memoryPackages) FindByCode(_ context.Context, code domainpackaging.PackageCode) (domainpackaging.Package, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	for _, p := range m.store.packages {
		if p.Code == code {
			return p, nil
		}
	}
	return domainpackaging.Package{}, apperr.NotFound(packageNotFoundMessage)
}

func (m memoryPackages) FindByID(_ context.Context, id uuid.UUID) (domainpackaging.Package, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	p, ok := m.store.packages[id]
	if !ok {
		return domainpackaging.Package{}, apperr.NotFound(packageNotFoundMessage)
	}
	return p, nil
}

type memoryAssets struct{ store *MemoryStore }

func (m memoryAssets) FindAssetByID(_ context.Context, id uuid.UUID) (domainmedia.Asset, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	a, ok := m.store.assets[id]
	if !ok {
		return domainmedia.Asset{}, apperr.NotFound(assetNotFoundMessage)
	}
	return a, nil
}

type memoryUsers struct{ store *MemoryStore }

func (m memoryUsers) FindByID(_ context.Context, id uuid.UUID) (domainuser.User, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	u, ok := m.store.users[id]
	if !ok {
		return domainuser.User{}, apperr.NotFound("Kullanıcı bulunamadı.")
	}
	return u, nil
}

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

func uuidLess(a, b uuid.UUID) bool {
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

var (
	_ Repository    = memoryRepo{}
	_ PackageLookup = memoryPackages{}
	_ AssetLookup   = memoryAssets{}
	_ UserReader    = memoryUsers{}
)
