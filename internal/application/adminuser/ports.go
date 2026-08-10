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
}
