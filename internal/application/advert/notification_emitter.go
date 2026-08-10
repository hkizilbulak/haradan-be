package advert

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// NotificationEmitter emits advert lifecycle notification events inside caller transactions.
type NotificationEmitter interface {
	OnAdvertPublished(ctx context.Context, tx pgx.Tx, advertID uuid.UUID) error
}
