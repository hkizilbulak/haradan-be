package campaign

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domaincampaign "github.com/hkizilbulak/haradan-be/internal/domain/campaign"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

// Clock supplies UTC instants for campaign timestamps.
type Clock interface {
	Now() time.Time
}

// PackageLookup resolves package catalog rows by code or id.
type PackageLookup interface {
	FindByCode(ctx context.Context, code domainpackaging.PackageCode) (domainpackaging.Package, error)
	FindByID(ctx context.Context, id uuid.UUID) (domainpackaging.Package, error)
}

// AssetLookup resolves media assets by id.
type AssetLookup interface {
	FindAssetByID(ctx context.Context, id uuid.UUID) (domainmedia.Asset, error)
}

// UserReader loads actor accounts for role checks.
type UserReader interface {
	FindByID(ctx context.Context, id uuid.UUID) (domainuser.User, error)
}

// ListFilter holds optional list predicates and keyset cursor.
type ListFilter struct {
	EventType       *domaincampaign.CampaignEventType
	IsActive        *bool
	SourcePackageID *uuid.UUID
	TargetPackageID *uuid.UUID
	AfterCreatedAt  *time.Time
	AfterID         *uuid.UUID
	Limit           int
}

// Repository persists campaigns.
type Repository interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	WithTx(tx pgx.Tx) Repository

	Create(ctx context.Context, c domaincampaign.Campaign) error
	GetByID(ctx context.Context, id uuid.UUID) (domaincampaign.Campaign, error)
	List(ctx context.Context, f ListFilter) ([]domaincampaign.Campaign, error)
	LockByID(ctx context.Context, id uuid.UUID) (domaincampaign.Campaign, error)
	UpdateOptimistic(ctx context.Context, c domaincampaign.Campaign, expectedVersion int) (domaincampaign.Campaign, error)
}
