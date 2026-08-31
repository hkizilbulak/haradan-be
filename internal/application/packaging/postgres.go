package packaging

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	pgpackaging "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/packaging"
)

type pgPackageRepo struct{ *pgpackaging.Repository }

func (r pgPackageRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.Repository.BeginTx(ctx)
}

func (r pgPackageRepo) WithTx(tx pgx.Tx) PackageRepository {
	return pgPackageRepo{r.Repository.WithTx(tx)}
}

func (r pgPackageRepo) FindByID(ctx context.Context, id uuid.UUID) (domainpackaging.Package, error) {
	return r.FindPackageByID(ctx, id)
}

func (r pgPackageRepo) FindByCode(ctx context.Context, code domainpackaging.PackageCode) (domainpackaging.Package, error) {
	return r.FindPackageByCode(ctx, code)
}

func (r pgPackageRepo) LockByCode(ctx context.Context, code domainpackaging.PackageCode) (domainpackaging.Package, error) {
	return r.LockPackageByCode(ctx, code)
}

func (r pgPackageRepo) List(ctx context.Context, includeInactive bool) ([]domainpackaging.Package, error) {
	return r.ListPackages(ctx, includeInactive)
}

func (r pgPackageRepo) Create(ctx context.Context, p domainpackaging.Package) error {
	return r.CreatePackage(ctx, p)
}

func (r pgPackageRepo) UpdateOptimistic(
	ctx context.Context,
	p domainpackaging.Package,
	expectedVersion int,
) (domainpackaging.Package, error) {
	return r.UpdatePackageOptimistic(ctx, p, expectedVersion)
}

type pgAssignmentRepo struct{ *pgpackaging.Repository }

func (r pgAssignmentRepo) WithTx(tx pgx.Tx) AssignmentRepository {
	return pgAssignmentRepo{r.Repository.WithTx(tx)}
}

func (r pgAssignmentRepo) FindActiveByAdvertID(
	ctx context.Context,
	advertID int64,
) (domainpackaging.AdvertPackageAssignment, error) {
	return r.FindActiveAssignmentByAdvertID(ctx, advertID)
}

func (r pgAssignmentRepo) FindEffectiveActiveByAdvertID(
	ctx context.Context,
	advertID int64,
	at time.Time,
) (domainpackaging.AdvertPackageAssignment, error) {
	return r.FindEffectiveActiveAssignmentByAdvertID(ctx, advertID, at)
}

func (r pgAssignmentRepo) LockActiveByAdvertID(
	ctx context.Context,
	advertID int64,
) (domainpackaging.AdvertPackageAssignment, error) {
	return r.LockActiveAssignmentByAdvertID(ctx, advertID)
}

func (r pgAssignmentRepo) ListHistoryByAdvertID(
	ctx context.Context,
	advertID int64,
	afterAssignedAt *time.Time,
	afterID *uuid.UUID,
	limit int,
) ([]domainpackaging.AdvertPackageAssignment, error) {
	return r.ListAssignmentHistoryByAdvertID(ctx, advertID, afterAssignedAt, afterID, limit)
}

func (r pgAssignmentRepo) Create(ctx context.Context, a domainpackaging.AdvertPackageAssignment) error {
	return r.CreateAssignment(ctx, a)
}

func (r pgAssignmentRepo) MarkSuperseded(ctx context.Context, id uuid.UUID, supersededAt, updatedAt time.Time) error {
	return r.MarkAssignmentSuperseded(ctx, id, supersededAt, updatedAt)
}

func (r pgAssignmentRepo) MarkCancelled(
	ctx context.Context,
	id uuid.UUID,
	cancelledAt, updatedAt time.Time,
	reason *string,
) error {
	return r.MarkAssignmentCancelled(ctx, id, cancelledAt, updatedAt, reason)
}

type pgFeatureRepo struct{ *pgpackaging.Repository }

func (r pgFeatureRepo) WithTx(tx pgx.Tx) FeatureRepository {
	return pgFeatureRepo{r.Repository.WithTx(tx)}
}

func (r pgFeatureRepo) FindActiveByAdvertIDAndCode(
	ctx context.Context,
	advertID int64,
	code domainpackaging.FeatureCode,
) (domainpackaging.AdvertFeatureActivation, error) {
	return r.FindActiveFeatureByAdvertIDAndCode(ctx, advertID, code)
}

func (r pgFeatureRepo) LockActiveByAdvertIDAndCode(
	ctx context.Context,
	advertID int64,
	code domainpackaging.FeatureCode,
) (domainpackaging.AdvertFeatureActivation, error) {
	return r.LockActiveFeatureByAdvertIDAndCode(ctx, advertID, code)
}

func (r pgFeatureRepo) Create(ctx context.Context, a domainpackaging.AdvertFeatureActivation) error {
	return r.CreateFeatureActivation(ctx, a)
}

func (r pgFeatureRepo) DeactivateActive(
	ctx context.Context,
	advertID int64,
	code domainpackaging.FeatureCode,
	deactivatedAt time.Time,
	reason *string,
	updatedAt time.Time,
) (bool, error) {
	return r.DeactivateActiveFeature(ctx, advertID, code, deactivatedAt, reason, updatedAt)
}

func (r pgFeatureRepo) DeactivateActiveUrgentForPackage(
	ctx context.Context,
	packageID uuid.UUID,
	deactivatedAt time.Time,
	reason *string,
	updatedAt time.Time,
) (int64, error) {
	return r.Repository.DeactivateActiveUrgentForPackage(ctx, packageID, deactivatedAt, reason, updatedAt)
}

// NewPostgresService constructs a packaging Service backed by PostgreSQL.
func NewPostgresService(
	pool *pgxpool.Pool,
	adverts AdvertReader,
	users UserReader,
	clock Clock,
	notifications ...NotificationEmitter,
) (*Service, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	repo := pgpackaging.NewRepository(pool)
	var emitter NotificationEmitter
	if len(notifications) > 0 {
		emitter = notifications[0]
	}
	return NewService(Config{
		Packages:      pgPackageRepo{repo},
		Assignments:   pgAssignmentRepo{repo},
		Features:      pgFeatureRepo{repo},
		Adverts:       adverts,
		Users:         users,
		Clock:         clock,
		Notifications: emitter,
	})
}

var (
	_ PackageRepository    = pgPackageRepo{}
	_ AssignmentRepository = pgAssignmentRepo{}
	_ FeatureRepository    = pgFeatureRepo{}
)
