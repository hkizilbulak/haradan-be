package packaging

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// NotificationEmitter emits packaging notification events inside caller transactions.
type NotificationEmitter interface {
	OnAdvancedAssignedWhilePublished(ctx context.Context, tx pgx.Tx, advertID, assignmentID uuid.UUID) error
	OnUrgentActivated(ctx context.Context, tx pgx.Tx, advertID, assignmentID uuid.UUID, activationVersion int) error
}
