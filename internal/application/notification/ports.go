package notification

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

// Clock supplies UTC instants for template timestamps.
type Clock interface {
	Now() time.Time
}

// UserReader loads actor accounts for role checks.
type UserReader interface {
	FindByID(ctx context.Context, id uuid.UUID) (domainuser.User, error)
}

// Repository persists notification templates.
type Repository interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	WithTx(tx pgx.Tx) Repository

	ListTemplates(ctx context.Context) ([]domainnotification.NotificationTemplate, error)
	GetByEventType(ctx context.Context, eventType domainnotification.TemplateEventType) (domainnotification.NotificationTemplate, error)
	LockByEventType(ctx context.Context, eventType domainnotification.TemplateEventType) (domainnotification.NotificationTemplate, error)
	UpdateOptimistic(ctx context.Context, t domainnotification.NotificationTemplate, expectedVersion int) (domainnotification.NotificationTemplate, error)
}
