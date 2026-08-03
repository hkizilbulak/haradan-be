package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

// UserRepository persists users.
type UserRepository interface {
	FindByNormalizedEmail(ctx context.Context, emailNormalized string) (domainuser.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (domainuser.User, error)
	FindByIDForUpdate(ctx context.Context, id uuid.UUID) (domainuser.User, error)
	Create(ctx context.Context, u domainuser.User) error
	RecordFailedLogin(ctx context.Context, userID uuid.UUID, now time.Time) error
	ResetFailedLogin(ctx context.Context, userID uuid.UUID, now time.Time) error
	MarkEmailVerified(ctx context.Context, userID uuid.UUID, verifiedAt time.Time) error
}

// SessionRepository persists sessions, one-time credentials, and security events.
type SessionRepository interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	WithTx(tx pgx.Tx) SessionRepository
	CreateSession(ctx context.Context, s domainauth.Session) error
	FindSessionByRefreshHashForUpdate(ctx context.Context, hash string) (domainauth.Session, error)
	FindSessionByID(ctx context.Context, id uuid.UUID) (domainauth.Session, error)
	FindSessionByIDForUpdate(ctx context.Context, id uuid.UUID) (domainauth.Session, error)
	RevokeSession(ctx context.Context, id uuid.UUID, now time.Time, reason string, replacedBy *uuid.UUID) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID, now time.Time, reason string) error
	CreateOneTimeCredential(ctx context.Context, c domainauth.OneTimeCredential) error
	FindOneTimeCredentialByHashForUpdate(ctx context.Context, tokenHash string) (domainauth.OneTimeCredential, error)
	ConsumeOneTimeCredential(ctx context.Context, id uuid.UUID, now time.Time) error
	InvalidateActiveOneTimeCredentials(ctx context.Context, userID uuid.UUID, purpose domainauth.OneTimePurpose, now time.Time) error
	InsertSecurityEvent(ctx context.Context, e domainauth.SecurityEvent) error
}

// UserRepositoryFactory builds a user repository for a transaction.
type UserRepositoryFactory func(tx pgx.Tx) UserRepository
