package packaging

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

// MemoryStore holds in-memory packaging state for unit tests.
type MemoryStore struct {
	mu   sync.Mutex
	txMu sync.Mutex

	packages    map[uuid.UUID]domainpackaging.Package
	assignments map[uuid.UUID]domainpackaging.AdvertPackageAssignment
	features    map[uuid.UUID]domainpackaging.AdvertFeatureActivation
	adverts     map[int64]domainadvert.Advert
	users       map[uuid.UUID]domainuser.User
}

// NewMemoryStore builds an empty packaging memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		packages:    map[uuid.UUID]domainpackaging.Package{},
		assignments: map[uuid.UUID]domainpackaging.AdvertPackageAssignment{},
		features:    map[uuid.UUID]domainpackaging.AdvertFeatureActivation{},
		adverts:     map[int64]domainadvert.Advert{},
		users:       map[uuid.UUID]domainuser.User{},
	}
}

func (s *MemoryStore) PutPackage(p domainpackaging.Package) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.packages[p.ID] = p
}

func (s *MemoryStore) PutAdvert(a domainadvert.Advert) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adverts[a.ID] = a
}

func (s *MemoryStore) PutUser(u domainuser.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[u.ID] = u
}

func (s *MemoryStore) Assignments() []domainpackaging.AdvertPackageAssignment {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domainpackaging.AdvertPackageAssignment, 0, len(s.assignments))
	for _, a := range s.assignments {
		out = append(out, a)
	}
	return out
}

func (s *MemoryStore) Features() []domainpackaging.AdvertFeatureActivation {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domainpackaging.AdvertFeatureActivation, 0, len(s.features))
	for _, f := range s.features {
		out = append(out, f)
	}
	return out
}

func (s *MemoryStore) Packages() PackageRepository { return memoryPackages{store: s} }
func (s *MemoryStore) AssignmentRepo() AssignmentRepository {
	return memoryAssignments{store: s}
}
func (s *MemoryStore) FeatureRepo() FeatureRepository { return memoryFeatures{store: s} }
func (s *MemoryStore) Adverts() AdvertReader          { return memoryAdverts{store: s} }
func (s *MemoryStore) Users() UserReader              { return memoryUsers{store: s} }

// NewMemoryService builds a packaging service backed by the store.
func NewMemoryService(store *MemoryStore, clock Clock) (*Service, error) {
	return NewService(Config{
		Packages:    store.Packages(),
		Assignments: store.AssignmentRepo(),
		Features:    store.FeatureRepo(),
		Adverts:     store.Adverts(),
		Users:       store.Users(),
		Clock:       clock,
	})
}

type memoryPackages struct{ store *MemoryStore }

func (m memoryPackages) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return m.store.AssignmentRepo().BeginTx(ctx)
}

func (m memoryPackages) WithTx(pgx.Tx) PackageRepository { return m }

func (m memoryPackages) FindByID(_ context.Context, id uuid.UUID) (domainpackaging.Package, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	p, ok := m.store.packages[id]
	if !ok {
		return domainpackaging.Package{}, apperr.NotFound(packageNotFoundMessage)
	}
	return p, nil
}

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

func (m memoryPackages) LockByCode(ctx context.Context, code domainpackaging.PackageCode) (domainpackaging.Package, error) {
	return m.FindByCode(ctx, code)
}

func (m memoryPackages) List(_ context.Context, includeInactive bool) ([]domainpackaging.Package, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	out := make([]domainpackaging.Package, 0, len(m.store.packages))
	for _, p := range m.store.packages {
		if !includeInactive && !p.IsActive {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].Code < out[j].Code
	})
	return out, nil
}

func (m memoryPackages) Create(_ context.Context, p domainpackaging.Package) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	for _, existing := range m.store.packages {
		if existing.Code == p.Code {
			return apperr.Conflict(packageCodeConflictMessage)
		}
		if existing.ID == p.ID {
			return apperr.Conflict(packageCodeConflictMessage)
		}
	}
	m.store.packages[p.ID] = p
	return nil
}

func (m memoryPackages) UpdateOptimistic(
	_ context.Context,
	p domainpackaging.Package,
	expectedVersion int,
) (domainpackaging.Package, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	cur, ok := m.store.packages[p.ID]
	if !ok {
		return domainpackaging.Package{}, apperr.NotFound(packageNotFoundMessage)
	}
	if cur.Version != expectedVersion {
		return domainpackaging.Package{}, apperr.StaleVersion(stalePackageVersionMessage)
	}
	p.Version = cur.Version + 1
	m.store.packages[p.ID] = p
	return p, nil
}

type memoryAssignments struct{ store *MemoryStore }

func (m memoryAssignments) BeginTx(context.Context) (pgx.Tx, error) {
	m.store.txMu.Lock()
	return &memoryTx{store: m.store}, nil
}

func (m memoryAssignments) WithTx(pgx.Tx) AssignmentRepository { return m }

func (m memoryAssignments) FindActiveByAdvertID(_ context.Context, advertID int64) (domainpackaging.AdvertPackageAssignment, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	return m.findActiveLocked(advertID)
}

func (m memoryAssignments) FindEffectiveActiveByAdvertID(
	_ context.Context,
	advertID int64,
	at time.Time,
) (domainpackaging.AdvertPackageAssignment, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	a, err := m.findActiveLocked(advertID)
	if err != nil {
		return domainpackaging.AdvertPackageAssignment{}, err
	}
	if !a.IsEffectiveAt(at) {
		return domainpackaging.AdvertPackageAssignment{}, apperr.NotFound(assignmentNotFoundMessage)
	}
	return a, nil
}

func (m memoryAssignments) LockActiveByAdvertID(ctx context.Context, advertID int64) (domainpackaging.AdvertPackageAssignment, error) {
	return m.FindActiveByAdvertID(ctx, advertID)
}

func (m memoryAssignments) findActiveLocked(advertID int64) (domainpackaging.AdvertPackageAssignment, error) {
	for _, a := range m.store.assignments {
		if a.AdvertID == advertID && a.Status == domainpackaging.AssignmentStatusActive {
			return a, nil
		}
	}
	return domainpackaging.AdvertPackageAssignment{}, apperr.NotFound(assignmentNotFoundMessage)
}

func (m memoryAssignments) ListHistoryByAdvertID(
	_ context.Context,
	advertID int64,
	afterAssignedAt *time.Time,
	afterID *uuid.UUID,
	limit int,
) ([]domainpackaging.AdvertPackageAssignment, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	rows := make([]domainpackaging.AdvertPackageAssignment, 0)
	for _, a := range m.store.assignments {
		if a.AdvertID != advertID {
			continue
		}
		if afterAssignedAt != nil && afterID != nil {
			if a.AssignedAt.After(*afterAssignedAt) {
				continue
			}
			if a.AssignedAt.Equal(*afterAssignedAt) && !uuidLess(a.ID, *afterID) {
				continue
			}
		}
		rows = append(rows, a)
	}
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].AssignedAt.Equal(rows[j].AssignedAt) {
			return rows[i].AssignedAt.After(rows[j].AssignedAt)
		}
		return uuidLess(rows[j].ID, rows[i].ID)
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (m memoryAssignments) Create(_ context.Context, a domainpackaging.AdvertPackageAssignment) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	if a.Status == domainpackaging.AssignmentStatusActive {
		for _, existing := range m.store.assignments {
			if existing.AdvertID == a.AdvertID && existing.Status == domainpackaging.AssignmentStatusActive {
				return apperr.Conflict(assignmentConflictMessage)
			}
		}
	}
	m.store.assignments[a.ID] = a
	return nil
}

func (m memoryAssignments) MarkSuperseded(_ context.Context, id uuid.UUID, supersededAt, updatedAt time.Time) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	a, ok := m.store.assignments[id]
	if !ok {
		return apperr.NotFound(assignmentNotFoundMessage)
	}
	a.Status = domainpackaging.AssignmentStatusSuperseded
	a.SupersededAt = &supersededAt
	a.UpdatedAt = updatedAt
	a.Version++
	m.store.assignments[id] = a
	return nil
}

func (m memoryAssignments) MarkCancelled(
	_ context.Context,
	id uuid.UUID,
	cancelledAt, updatedAt time.Time,
	reason *string,
) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	a, ok := m.store.assignments[id]
	if !ok || a.Status != domainpackaging.AssignmentStatusActive {
		return apperr.NotFound(assignmentNotFoundMessage)
	}
	a.Status = domainpackaging.AssignmentStatusCancelled
	a.CancelledAt = &cancelledAt
	if reason != nil {
		a.Reason = reason
	}
	a.UpdatedAt = updatedAt
	a.Version++
	m.store.assignments[id] = a
	return nil
}

type memoryFeatures struct{ store *MemoryStore }

func (m memoryFeatures) WithTx(pgx.Tx) FeatureRepository { return m }

func (m memoryFeatures) FindActiveByAdvertIDAndCode(
	_ context.Context,
	advertID int64,
	code domainpackaging.FeatureCode,
) (domainpackaging.AdvertFeatureActivation, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	return m.findActiveLocked(advertID, code)
}

func (m memoryFeatures) LockActiveByAdvertIDAndCode(
	ctx context.Context,
	advertID int64,
	code domainpackaging.FeatureCode,
) (domainpackaging.AdvertFeatureActivation, error) {
	return m.FindActiveByAdvertIDAndCode(ctx, advertID, code)
}

func (m memoryFeatures) findActiveLocked(
	advertID int64,
	code domainpackaging.FeatureCode,
) (domainpackaging.AdvertFeatureActivation, error) {
	for _, f := range m.store.features {
		if f.AdvertID == advertID && f.FeatureCode == code && f.Status == domainpackaging.FeatureActivationStatusActive {
			return f, nil
		}
	}
	return domainpackaging.AdvertFeatureActivation{}, apperr.NotFound(activationNotFoundMessage)
}

func (m memoryFeatures) FindLatestActivationVersion(
	_ context.Context,
	advertID int64,
	code domainpackaging.FeatureCode,
) (int, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	latest := 0
	for _, f := range m.store.features {
		if f.AdvertID == advertID && f.FeatureCode == code && f.ActivationVersion > latest {
			latest = f.ActivationVersion
		}
	}
	return latest, nil
}

func (m memoryFeatures) Create(_ context.Context, a domainpackaging.AdvertFeatureActivation) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	if a.Status == domainpackaging.FeatureActivationStatusActive {
		for _, existing := range m.store.features {
			if existing.AdvertID == a.AdvertID &&
				existing.FeatureCode == a.FeatureCode &&
				existing.Status == domainpackaging.FeatureActivationStatusActive {
				return apperr.Conflict(activationConflictMessage)
			}
		}
	}
	m.store.features[a.ID] = a
	return nil
}

func (m memoryFeatures) DeactivateActive(
	_ context.Context,
	advertID int64,
	code domainpackaging.FeatureCode,
	deactivatedAt time.Time,
	reason *string,
	updatedAt time.Time,
) (bool, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	for id, f := range m.store.features {
		if f.AdvertID == advertID && f.FeatureCode == code && f.Status == domainpackaging.FeatureActivationStatusActive {
			f.Status = domainpackaging.FeatureActivationStatusDeactivated
			f.DeactivatedAt = &deactivatedAt
			f.Reason = reason
			f.UpdatedAt = updatedAt
			m.store.features[id] = f
			return true, nil
		}
	}
	return false, nil
}

func (m memoryFeatures) DeactivateActiveUrgentForPackage(
	_ context.Context,
	packageID uuid.UUID,
	deactivatedAt time.Time,
	reason *string,
	updatedAt time.Time,
) (int64, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	activeAssignmentIDs := map[uuid.UUID]struct{}{}
	for _, a := range m.store.assignments {
		if a.PackageID == packageID && a.Status == domainpackaging.AssignmentStatusActive {
			activeAssignmentIDs[a.ID] = struct{}{}
		}
	}
	var n int64
	for id, f := range m.store.features {
		if f.FeatureCode != domainpackaging.FeatureCodeUrgent {
			continue
		}
		if f.Status != domainpackaging.FeatureActivationStatusActive {
			continue
		}
		if _, ok := activeAssignmentIDs[f.PackageAssignmentID]; !ok {
			continue
		}
		f.Status = domainpackaging.FeatureActivationStatusDeactivated
		f.DeactivatedAt = &deactivatedAt
		f.Reason = reason
		f.UpdatedAt = updatedAt
		m.store.features[id] = f
		n++
	}
	return n, nil
}

type memoryAdverts struct{ store *MemoryStore }

func (m memoryAdverts) FindByID(_ context.Context, advertID int64) (domainadvert.Advert, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	a, ok := m.store.adverts[advertID]
	if !ok || a.IsDeleted() {
		return domainadvert.Advert{}, apperr.NotFound(advertNotFoundMessage)
	}
	return a, nil
}

func (m memoryAdverts) FindByIDForUpdate(ctx context.Context, advertID int64) (domainadvert.Advert, error) {
	return m.FindByID(ctx, advertID)
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

const (
	activationNotFoundMessage = "Aktif URGENT aktivasyonu bulunamadı."
	assignmentConflictMessage = "Paket ataması aynı anda başka bir işlem tarafından güncellendi."
	activationConflictMessage = "URGENT aktivasyonu aynı anda başka bir işlem tarafından güncellendi."
)
