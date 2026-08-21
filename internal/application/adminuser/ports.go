package adminuser

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

// Repository groups the transactional user, session, and audit queries needed
// by ADMIN-USER operations.
type Repository interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	WithTx(tx pgx.Tx) Repository

	ListUsers(ctx context.Context, status *domainuser.Status, role *domainuser.Role, query string, afterCreated *time.Time, afterID *uuid.UUID, limit int) ([]domainuser.User, error)
	FindUser(ctx context.Context, userID uuid.UUID) (domainuser.User, error)
	FindUserForUpdate(ctx context.Context, userID uuid.UUID) (domainuser.User, error)
	GetDetail(ctx context.Context, userID uuid.UUID, now time.Time) (Detail, error)
	ActiveSessionCount(ctx context.Context, userID uuid.UUID, now time.Time) (int, error)
	UpdateRole(ctx context.Context, userID uuid.UUID, role domainuser.Role, securityStamp uuid.UUID, now time.Time) (domainuser.User, error)
	UpdateStatus(ctx context.Context, userID uuid.UUID, status domainuser.Status, securityStamp uuid.UUID, now time.Time) (domainuser.User, error)
	RevokeAllSessions(ctx context.Context, userID uuid.UUID, now time.Time, reason string) error
	InsertSecurityEvent(ctx context.Context, event domainauth.SecurityEvent) error
	ListSecurityEvents(ctx context.Context, userID uuid.UUID, eventType *domainauth.SecurityEventType, afterCreated *time.Time, afterID *uuid.UUID, limit int) ([]domainauth.SecurityEvent, error)

	CreateUser(ctx context.Context, user domainuser.User) error
	UpdateProfile(ctx context.Context, userID uuid.UUID, firstName, lastName string, phone *string, now time.Time) (domainuser.User, error)
	UpdateEmail(ctx context.Context, userID uuid.UUID, email, emailNormalized string, securityStamp uuid.UUID, now time.Time) (domainuser.User, error)
	FindUserByNormalizedEmail(ctx context.Context, normalized string) (domainuser.User, error)
	CountActiveAdmins(ctx context.Context) (int, error)
	// LockActiveAdminGuard serializes last-active-admin checks inside the current TX
	// (e.g. pg_advisory_xact_lock). Must be called before CountActiveAdmins on demotion.
	LockActiveAdminGuard(ctx context.Context) error
	InvalidateActiveOneTimeCredentials(ctx context.Context, userID uuid.UUID, purpose domainauth.OneTimePurpose, now time.Time) error
	CreateOneTimeCredential(ctx context.Context, cred domainauth.OneTimeCredential) error
}

// PasswordHasher hashes passwords for admin-created accounts (random secret).
type PasswordHasher interface {
	Hash(password string) (string, error)
}

// EmailSender delivers password-setup and email-change verification emails.
type EmailSender interface {
	SendPasswordReset(ctx context.Context, toEmail, plaintextToken, fullName string) error
	SendRegistrationVerification(ctx context.Context, toEmail, plaintextToken, fullName string) error
}
