package auth

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hkizilbulak/haradan-be/internal/platform/security/password"
	"github.com/hkizilbulak/haradan-be/internal/platform/security/token"
)

// NewMemoryServiceForTest builds an in-memory auth service for unit/HTTP tests.
func NewMemoryServiceForTest(t testing.TB) (*Service, *MemoryStore, *token.FixedClock) {
	t.Helper()
	return newMemoryServiceForTest(t, nil)
}

// NewMemoryServiceForTestWithEmail builds an in-memory auth service with a custom email sender.
func NewMemoryServiceForTestWithEmail(t testing.TB, email EmailSender) (*Service, *MemoryStore, *token.FixedClock) {
	t.Helper()
	return newMemoryServiceForTest(t, email)
}

func newMemoryServiceForTest(t testing.TB, email EmailSender) (*Service, *MemoryStore, *token.FixedClock) {
	t.Helper()
	store := newMemStore()
	clock := &token.FixedClock{T: time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)}
	hasher, err := password.NewHasher(password.TestParams())
	if err != nil {
		t.Fatal(err)
	}
	tok, err := token.NewManager(token.Config{
		JWTSecret:          "unit-test-jwt-secret-value",
		AccessTokenTTL:     15 * time.Minute,
		RefreshAbsoluteTTL: 24 * time.Hour,
		RefreshIdleTTL:     2 * time.Hour,
		Clock:              clock,
	})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewService(Config{
		Users:    memUsers{store: store},
		Sessions: memSessions{store: store},
		UserTx: func(_ pgx.Tx) UserRepository {
			return memUsers{store: store}
		},
		Hasher:            hasher,
		Tokens:            tok,
		Clock:             clock,
		EmailSender:       email,
		EmailVerifyTTL:    24 * time.Hour,
		DummyPasswordHash: password.DummyHash(hasher),
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, store, clock
}

// MemoryStore exposes in-memory auth state for assertions in tests.
type MemoryStore = memStore
