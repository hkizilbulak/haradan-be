package notification

import (
	"context"
	"sort"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

// MemoryStore holds in-memory notification template state for unit tests.
type MemoryStore struct {
	mu   sync.Mutex
	txMu sync.Mutex

	templates map[domainnotification.TemplateEventType]domainnotification.NotificationTemplate
	users     map[uuid.UUID]domainuser.User
}

// NewMemoryStore builds an empty notification memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		templates: map[domainnotification.TemplateEventType]domainnotification.NotificationTemplate{},
		users:     map[uuid.UUID]domainuser.User{},
	}
}

func (s *MemoryStore) PutTemplate(t domainnotification.NotificationTemplate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.templates[t.EventType] = t
}

func (s *MemoryStore) PutUser(u domainuser.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[u.ID] = u
}

func (s *MemoryStore) Repo() Repository  { return memoryRepo{store: s} }
func (s *MemoryStore) Users() UserReader { return memoryUsers{store: s} }

// NewMemoryService builds a notification service backed by the store.
func NewMemoryService(store *MemoryStore, clock Clock) (*Service, error) {
	return NewService(Config{
		Repo:  store.Repo(),
		Users: store.Users(),
		Clock: clock,
	})
}

type memoryRepo struct{ store *MemoryStore }

func (m memoryRepo) BeginTx(context.Context) (pgx.Tx, error) {
	m.store.txMu.Lock()
	return &memoryTx{store: m.store}, nil
}

func (m memoryRepo) WithTx(pgx.Tx) Repository { return m }

func (m memoryRepo) ListTemplates(context.Context) ([]domainnotification.NotificationTemplate, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	out := make([]domainnotification.NotificationTemplate, 0, len(m.store.templates))
	for _, t := range m.store.templates {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].EventType < out[j].EventType
	})
	return out, nil
}

func (m memoryRepo) GetByEventType(_ context.Context, eventType domainnotification.TemplateEventType) (domainnotification.NotificationTemplate, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	t, ok := m.store.templates[eventType]
	if !ok {
		return domainnotification.NotificationTemplate{}, apperr.NotFound(templateNotFoundMessage)
	}
	return t, nil
}

func (m memoryRepo) LockByEventType(ctx context.Context, eventType domainnotification.TemplateEventType) (domainnotification.NotificationTemplate, error) {
	return m.GetByEventType(ctx, eventType)
}

func (m memoryRepo) UpdateOptimistic(
	_ context.Context,
	t domainnotification.NotificationTemplate,
	expectedVersion int,
) (domainnotification.NotificationTemplate, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	current, ok := m.store.templates[t.EventType]
	if !ok {
		return domainnotification.NotificationTemplate{}, apperr.NotFound(templateNotFoundMessage)
	}
	if current.Version != expectedVersion {
		return domainnotification.NotificationTemplate{}, apperr.StaleVersion(staleVersionMessage)
	}
	t.ID = current.ID
	t.EventType = current.EventType
	t.CreatedAt = current.CreatedAt
	t.Version = expectedVersion + 1
	m.store.templates[t.EventType] = t
	return t, nil
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

var (
	_ Repository = memoryRepo{}
	_ UserReader = memoryUsers{}
)
