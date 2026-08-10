package banner

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domainbanner "github.com/hkizilbulak/haradan-be/internal/domain/banner"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

type Clock interface{ Now() time.Time }

type UserReader interface {
	FindByID(context.Context, uuid.UUID) (domainuser.User, error)
}

// MediaReader keeps asset and variant validation in the application layer.
type MediaReader interface {
	FindAssetByID(context.Context, uuid.UUID) (domainmedia.Asset, error)
	ListVariantsByAsset(context.Context, uuid.UUID) ([]domainmedia.Variant, error)
}

type ListFilter struct {
	Placement *domainbanner.Placement
	Status    *domainbanner.Status
	Limit     int
}

type Repository interface {
	BeginTx(context.Context) (pgx.Tx, error)
	WithTx(pgx.Tx) Repository
	Create(context.Context, domainbanner.Banner) error
	GetByID(context.Context, uuid.UUID) (domainbanner.Banner, error)
	LockByID(context.Context, uuid.UUID) (domainbanner.Banner, error)
	List(context.Context, ListFilter) ([]domainbanner.Banner, error)
	ListActive(context.Context, domainbanner.Placement) ([]domainbanner.Banner, error)
	UpdateOptimistic(context.Context, domainbanner.Banner, int) (domainbanner.Banner, error)
}
