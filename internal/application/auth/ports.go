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
	UpdatePasswordHash(ctx context.Context, userID uuid.UUID, passwordHash string, securityStamp uuid.UUID, now time.Time) error
	UpdateEmail(ctx context.Context, userID uuid.UUID, email, emailNormalized string, securityStamp uuid.UUID, now time.Time) error
	UpdateProfile(ctx context.Context, userID uuid.UUID, patch ProfilePatch, now time.Time) (domainuser.User, error)
}

// ProfilePatch is the set of user-editable profile fields for ACCOUNT-02.
type ProfilePatch struct {
	FirstName *string
	LastName  *string
	// PhoneSet is true when the request included the phone field (including JSON null).
	PhoneSet   bool
	PhoneValue *string
}

// SessionRepository persists sessions, one-time credentials, and security events.
type SessionRepository interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	WithTx(tx pgx.Tx) SessionRepository
	CreateSession(ctx context.Context, s domainauth.Session) error
	FindSessionByRefreshHashForUpdate(ctx context.Context, hash string) (domainauth.Session, error)
	FindSessionByID(ctx context.Context, id uuid.UUID) (domainauth.Session, error)
	FindSessionByIDForUpdate(ctx context.Context, id uuid.UUID) (domainauth.Session, error)
	ListSessionsByUserID(ctx context.Context, userID uuid.UUID, afterLastUsed *time.Time, afterID *uuid.UUID, limit int) ([]domainauth.Session, error)
	RevokeSession(ctx context.Context, id uuid.UUID, now time.Time, reason string, replacedBy *uuid.UUID) error
	RevokeSessionForUser(ctx context.Context, userID, sessionID uuid.UUID, now time.Time, reason string) (sess domainauth.Session, newlyRevoked bool, err error)
	RevokeAllSessionsForUser(ctx context.Context, userID uuid.UUID, now time.Time, reason string) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID, now time.Time, reason string) error
	CreateOneTimeCredential(ctx context.Context, c domainauth.OneTimeCredential) error
	FindOneTimeCredentialByHashForUpdate(ctx context.Context, tokenHash string) (domainauth.OneTimeCredential, error)
	ConsumeOneTimeCredential(ctx context.Context, id uuid.UUID, now time.Time) error
	InvalidateActiveOneTimeCredentials(ctx context.Context, userID uuid.UUID, purpose domainauth.OneTimePurpose, now time.Time) error
	InsertSecurityEvent(ctx context.Context, e domainauth.SecurityEvent) error
}

// UserRepositoryFactory builds a user repository for a transaction.
type UserRepositoryFactory func(tx pgx.Tx) UserRepository
