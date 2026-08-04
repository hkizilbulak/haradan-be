package campaign

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domaincampaign "github.com/hkizilbulak/haradan-be/internal/domain/campaign"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	pgcampaign "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/campaign"
	pgpackaging "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/packaging"
)

type pgRepo struct{ *pgcampaign.Repository }

func (r pgRepo) WithTx(tx pgx.Tx) Repository {
	return pgRepo{r.Repository.WithTx(tx)}
}

func (r pgRepo) Create(ctx context.Context, c domaincampaign.Campaign) error {
	return r.Repository.Create(ctx, c)
}

func (r pgRepo) GetByID(ctx context.Context, id uuid.UUID) (domaincampaign.Campaign, error) {
	return r.Repository.GetByID(ctx, id)
}

func (r pgRepo) List(ctx context.Context, f ListFilter) ([]domaincampaign.Campaign, error) {
	return r.Repository.List(ctx, pgcampaign.ListFilter{
		EventType:       f.EventType,
		IsActive:        f.IsActive,
		SourcePackageID: f.SourcePackageID,
		TargetPackageID: f.TargetPackageID,
		AfterCreatedAt:  f.AfterCreatedAt,
		AfterID:         f.AfterID,
		Limit:           f.Limit,
	})
}

func (r pgRepo) LockByID(ctx context.Context, id uuid.UUID) (domaincampaign.Campaign, error) {
	return r.Repository.LockByID(ctx, id)
}

func (r pgRepo) UpdateOptimistic(
	ctx context.Context,
	c domaincampaign.Campaign,
	expectedVersion int,
) (domaincampaign.Campaign, error) {
	return r.Repository.UpdateOptimistic(ctx, c, expectedVersion)
}

// NewPostgresService constructs a campaign Service backed by PostgreSQL.
func NewPostgresService(
	pool *pgxpool.Pool,
	packages PackageLookup,
	assets AssetLookup,
	users UserReader,
	clock Clock,
) (*Service, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	return NewService(Config{
		Repo:     pgRepo{pgcampaign.NewRepository(pool)},
		Packages: packages,
		Assets:   assets,
		Users:    users,
		Clock:    clock,
	})
}

// NewPostgresPackageLookup adapts the packaging postgres repository to PackageLookup.
func NewPostgresPackageLookup(pool *pgxpool.Pool) PackageLookup {
	return pgPackageLookup{pgpackaging.NewRepository(pool)}
}

type pgPackageLookup struct{ *pgpackaging.Repository }

func (r pgPackageLookup) FindByCode(ctx context.Context, code domainpackaging.PackageCode) (domainpackaging.Package, error) {
	return r.FindPackageByCode(ctx, code)
}

func (r pgPackageLookup) FindByID(ctx context.Context, id uuid.UUID) (domainpackaging.Package, error) {
	return r.FindPackageByID(ctx, id)
}

var (
	_ Repository    = pgRepo{}
	_ PackageLookup = pgPackageLookup{}
)
