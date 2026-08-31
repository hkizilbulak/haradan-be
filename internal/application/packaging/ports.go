package packaging

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

// Clock supplies UTC instants for assignment and activation timestamps.
type Clock interface {
	Now() time.Time
}

// PackageRepository persists the package catalog.
type PackageRepository interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	WithTx(tx pgx.Tx) PackageRepository

	FindByID(ctx context.Context, id uuid.UUID) (domainpackaging.Package, error)
	FindByCode(ctx context.Context, code domainpackaging.PackageCode) (domainpackaging.Package, error)
	LockByCode(ctx context.Context, code domainpackaging.PackageCode) (domainpackaging.Package, error)
	List(ctx context.Context, includeInactive bool) ([]domainpackaging.Package, error)
	Create(ctx context.Context, p domainpackaging.Package) error
	UpdateOptimistic(ctx context.Context, p domainpackaging.Package, expectedVersion int) (domainpackaging.Package, error)
}

// AssignmentRepository persists advert package assignment history.
type AssignmentRepository interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	WithTx(tx pgx.Tx) AssignmentRepository

	// FindActiveByAdvertID returns the status=ACTIVE row (time window ignored).
	FindActiveByAdvertID(ctx context.Context, advertID int64) (domainpackaging.AdvertPackageAssignment, error)
	// FindEffectiveActiveByAdvertID returns the ACTIVE row covering instant at.
	FindEffectiveActiveByAdvertID(ctx context.Context, advertID int64, at time.Time) (domainpackaging.AdvertPackageAssignment, error)
	LockActiveByAdvertID(ctx context.Context, advertID int64) (domainpackaging.AdvertPackageAssignment, error)
	ListHistoryByAdvertID(
		ctx context.Context,
		advertID int64,
		afterAssignedAt *time.Time,
		afterID *uuid.UUID,
		limit int,
	) ([]domainpackaging.AdvertPackageAssignment, error)
	Create(ctx context.Context, a domainpackaging.AdvertPackageAssignment) error
	MarkSuperseded(ctx context.Context, id uuid.UUID, supersededAt, updatedAt time.Time) error
	MarkCancelled(ctx context.Context, id uuid.UUID, cancelledAt, updatedAt time.Time, reason *string) error
}

// FeatureRepository persists advert feature activations (URGENT).
type FeatureRepository interface {
	WithTx(tx pgx.Tx) FeatureRepository

	FindActiveByAdvertIDAndCode(
		ctx context.Context,
		advertID int64,
		code domainpackaging.FeatureCode,
	) (domainpackaging.AdvertFeatureActivation, error)
	LockActiveByAdvertIDAndCode(
		ctx context.Context,
		advertID int64,
		code domainpackaging.FeatureCode,
	) (domainpackaging.AdvertFeatureActivation, error)
	FindLatestActivationVersion(
		ctx context.Context,
		advertID int64,
		code domainpackaging.FeatureCode,
	) (int, error)
	Create(ctx context.Context, a domainpackaging.AdvertFeatureActivation) error
	DeactivateActive(
		ctx context.Context,
		advertID int64,
		code domainpackaging.FeatureCode,
		deactivatedAt time.Time,
		reason *string,
		updatedAt time.Time,
	) (bool, error)
	DeactivateActiveUrgentForPackage(
		ctx context.Context,
		packageID uuid.UUID,
		deactivatedAt time.Time,
		reason *string,
		updatedAt time.Time,
	) (int64, error)
}

// AdvertReader loads adverts for package/URGENT authorization and locks.
type AdvertReader interface {
	FindByID(ctx context.Context, advertID int64) (domainadvert.Advert, error)
	FindByIDForUpdate(ctx context.Context, advertID int64) (domainadvert.Advert, error)
}

// UserReader loads actor accounts for role checks.
type UserReader interface {
	FindByID(ctx context.Context, id uuid.UUID) (domainuser.User, error)
}
