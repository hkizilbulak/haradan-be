package paytr

import (
	"context"

	"github.com/google/uuid"

	apppackaging "github.com/hkizilbulak/haradan-be/internal/application/packaging"
	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

// PackageLookup adapts packaging.Service to PackageCatalog.
type PackageLookup struct {
	Svc interface {
		LookupPackageByCode(ctx context.Context, code domainpackaging.PackageCode) (domainpackaging.Package, error)
	}
}

func (p PackageLookup) FindByCode(ctx context.Context, code domainpackaging.PackageCode) (domainpackaging.Package, error) {
	return p.Svc.LookupPackageByCode(ctx, code)
}

// AdvertRepo adapts advert persistence to AdvertAccess.
type AdvertRepo struct {
	Repo interface {
		FindByIDForOwner(ctx context.Context, ownerID uuid.UUID, advertID int64) (domainadvert.Advert, error)
		FindByID(ctx context.Context, advertID int64) (domainadvert.Advert, error)
	}
}

func (a AdvertRepo) FindByIDForOwner(ctx context.Context, ownerID uuid.UUID, advertID int64) (domainadvert.Advert, error) {
	return a.Repo.FindByIDForOwner(ctx, ownerID, advertID)
}

func (a AdvertRepo) FindByID(ctx context.Context, advertID int64) (domainadvert.Advert, error) {
	return a.Repo.FindByID(ctx, advertID)
}

// UserRepo adapts user persistence to UserAccess.
type UserRepo struct {
	Repo interface {
		FindByID(ctx context.Context, id uuid.UUID) (domainuser.User, error)
	}
}

func (u UserRepo) FindByID(ctx context.Context, id uuid.UUID) (domainuser.User, error) {
	return u.Repo.FindByID(ctx, id)
}

// PackagingBridge adapts packaging.Service to PackagingAssigner.
type PackagingBridge struct {
	Svc interface {
		AssignAdvertPackage(ctx context.Context, in apppackaging.AssignAdvertPackageInput) (apppackaging.AssignmentView, error)
	}
}

func (p PackagingBridge) AssignAdvertPackage(ctx context.Context, in apppackaging.AssignAdvertPackageInput) (apppackaging.AssignmentView, error) {
	return p.Svc.AssignAdvertPackage(ctx, in)
}

// AdvertBridge adapts advert.Service to AdvertSubmitter.
type AdvertBridge struct {
	Svc interface {
		SubmitAdvertForReview(ctx context.Context, ownerID uuid.UUID, advertID int64, expectedVersion int) (domainadvert.OwnerView, error)
		ResubmitAdvertForReview(ctx context.Context, ownerID uuid.UUID, advertID int64, expectedVersion int) (domainadvert.OwnerView, error)
	}
}

func (a AdvertBridge) SubmitAdvertForReview(ctx context.Context, ownerID uuid.UUID, advertID int64, expectedVersion int) (domainadvert.OwnerView, error) {
	return a.Svc.SubmitAdvertForReview(ctx, ownerID, advertID, expectedVersion)
}

func (a AdvertBridge) ResubmitAdvertForReview(ctx context.Context, ownerID uuid.UUID, advertID int64, expectedVersion int) (domainadvert.OwnerView, error) {
	return a.Svc.ResubmitAdvertForReview(ctx, ownerID, advertID, expectedVersion)
}
