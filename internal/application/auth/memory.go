package auth

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

type memStore struct {
	mu         sync.Mutex
	txMu       sync.Mutex // serializes BeginTx..Commit like a coarse row lock for tests
	users      map[uuid.UUID]domainuser.User
	byEmail    map[string]uuid.UUID
	sessions   map[uuid.UUID]domainauth.Session
	byRefresh  map[string]uuid.UUID
	otc        []domainauth.OneTimeCredential
	events     []domainauth.SecurityEvent
	failCreate bool
	failTx     bool
}

func newMemStore() *memStore {
	return &memStore{
		users:     map[uuid.UUID]domainuser.User{},
		byEmail:   map[string]uuid.UUID{},
		sessions:  map[uuid.UUID]domainauth.Session{},
		byRefresh: map[string]uuid.UUID{},
	}
}

type memTx struct {
	store *memStore
	once  sync.Once
}

func (t *memTx) release() {
	t.once.Do(func() { t.store.txMu.Unlock() })
}

func (t *memTx) Begin(context.Context) (pgx.Tx, error) { return t, nil }
func (t *memTx) BeginFunc(ctx context.Context, fn func(pgx.Tx) error) error {
	return fn(t)
}
func (t *memTx) Commit(context.Context) error {
	t.release()
	return nil
}
func (t *memTx) Rollback(context.Context) error {
	t.release()
	return nil
}
func (t *memTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *memTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (t *memTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (t *memTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *memTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (t *memTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (t *memTx) QueryRow(context.Context, string, ...any) pgx.Row        { return nil }
func (t *memTx) Conn() *pgx.Conn                                         { return nil }

type memUsers struct{ store *memStore }

func (m memUsers) FindByNormalizedEmail(_ context.Context, emailNormalized string) (domainuser.User, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	id, ok := m.store.byEmail[emailNormalized]
	if !ok {
		return domainuser.User{}, apperr.NotFound("user not found")
	}
	return m.store.users[id], nil
}
func (m memUsers) FindByID(_ context.Context, id uuid.UUID) (domainuser.User, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	u, ok := m.store.users[id]
	if !ok {
		return domainuser.User{}, apperr.NotFound("user not found")
	}
	return u, nil
}
func (m memUsers) FindByIDForUpdate(ctx context.Context, id uuid.UUID) (domainuser.User, error) {
	return m.FindByID(ctx, id)
}
func (m memUsers) Create(_ context.Context, u domainuser.User) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	if m.store.failCreate {
		return apperr.Internal(nil)
	}
	if _, ok := m.store.byEmail[u.EmailNormalized]; ok {
		return apperr.Conflict("email already registered")
	}
	m.store.users[u.ID] = u
	m.store.byEmail[u.EmailNormalized] = u.ID
	return nil
}
func (m memUsers) RecordFailedLogin(_ context.Context, userID uuid.UUID, now time.Time) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	u := m.store.users[userID]
	u.FailedLoginCount++
	u.UpdatedAt = now
	m.store.users[userID] = u
	return nil
}
func (m memUsers) ResetFailedLogin(_ context.Context, userID uuid.UUID, now time.Time) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	u := m.store.users[userID]
	u.FailedLoginCount = 0
	u.LockedUntil = nil
	u.UpdatedAt = now
	m.store.users[userID] = u
	return nil
}
func (m memUsers) MarkEmailVerified(_ context.Context, userID uuid.UUID, verifiedAt time.Time) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	u, ok := m.store.users[userID]
	if !ok {
		return apperr.NotFound("user not found")
	}
	if u.EmailVerifiedAt == nil {
		u.EmailVerifiedAt = &verifiedAt
	}
	u.UpdatedAt = verifiedAt
	m.store.users[userID] = u
	return nil
}

type memSessions struct{ store *memStore }

func (m memSessions) BeginTx(_ context.Context) (pgx.Tx, error) {
	if m.store.failTx {
		return nil, apperr.Internal(nil)
	}
	m.store.txMu.Lock()
	return &memTx{store: m.store}, nil
}
func (m memSessions) WithTx(_ pgx.Tx) SessionRepository { return m }
func (m memSessions) CreateSession(_ context.Context, s domainauth.Session) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	m.store.sessions[s.ID] = s
	m.store.byRefresh[s.RefreshTokenHash] = s.ID
	return nil
}
func (m memSessions) FindSessionByRefreshHashForUpdate(_ context.Context, hash string) (domainauth.Session, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	id, ok := m.store.byRefresh[hash]
	if !ok {
		return domainauth.Session{}, apperr.NotFound("session not found")
	}
	return m.store.sessions[id], nil
}
func (m memSessions) FindSessionByID(_ context.Context, id uuid.UUID) (domainauth.Session, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	s, ok := m.store.sessions[id]
	if !ok {
		return domainauth.Session{}, apperr.NotFound("session not found")
	}
	return s, nil
}
func (m memSessions) FindSessionByIDForUpdate(ctx context.Context, id uuid.UUID) (domainauth.Session, error) {
	return m.FindSessionByID(ctx, id)
}
func (m memSessions) RevokeSession(_ context.Context, id uuid.UUID, now time.Time, reason string, replacedBy *uuid.UUID) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	s := m.store.sessions[id]
	if s.RevokedAt == nil {
		s.RevokedAt = &now
		s.RevokeReason = &reason
	}
	if replacedBy != nil && s.ReplacedBySessionID == nil {
		s.ReplacedBySessionID = replacedBy
	}
	m.store.sessions[id] = s
	return nil
}
func (m memSessions) RevokeFamily(_ context.Context, familyID uuid.UUID, now time.Time, reason string) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	for id, s := range m.store.sessions {
		if s.FamilyID == familyID && s.RevokedAt == nil {
			s.RevokedAt = &now
			s.RevokeReason = &reason
			m.store.sessions[id] = s
		}
	}
	return nil
}
func (m memSessions) CreateOneTimeCredential(_ context.Context, c domainauth.OneTimeCredential) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	for _, existing := range m.store.otc {
		if existing.TokenHash == c.TokenHash {
			return apperr.Conflict("one-time credential conflict")
		}
		if existing.UserID == c.UserID && existing.Purpose == c.Purpose && existing.IsActive() {
			return apperr.Conflict("one-time credential conflict")
		}
	}
	m.store.otc = append(m.store.otc, c)
	return nil
}
func (m memSessions) FindOneTimeCredentialByHashForUpdate(_ context.Context, tokenHash string) (domainauth.OneTimeCredential, error) {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	for _, c := range m.store.otc {
		if c.TokenHash == tokenHash {
			return c, nil
		}
	}
	return domainauth.OneTimeCredential{}, apperr.NotFound("credential not found")
}
func (m memSessions) ConsumeOneTimeCredential(_ context.Context, id uuid.UUID, now time.Time) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	for i := range m.store.otc {
		c := m.store.otc[i]
		if c.ID != id {
			continue
		}
		if !c.IsActive() {
			return apperr.TokenAlreadyUsed("Doğrulama jetonu zaten kullanılmış.")
		}
		c.ConsumedAt = &now
		m.store.otc[i] = c
		return nil
	}
	return apperr.TokenAlreadyUsed("Doğrulama jetonu zaten kullanılmış.")
}
func (m memSessions) InvalidateActiveOneTimeCredentials(_ context.Context, userID uuid.UUID, purpose domainauth.OneTimePurpose, now time.Time) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	for i := range m.store.otc {
		c := m.store.otc[i]
		if c.UserID == userID && c.Purpose == purpose && c.ConsumedAt == nil && c.InvalidatedAt == nil {
			c.InvalidatedAt = &now
			m.store.otc[i] = c
		}
	}
	return nil
}
func (m memSessions) InsertSecurityEvent(_ context.Context, e domainauth.SecurityEvent) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	m.store.events = append(m.store.events, e)
	return nil
}
