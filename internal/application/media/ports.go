package media

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

// ObjectInfo is the provider's own view of a stored object. Client-declared
// metadata is never canonical; only what the provider reports counts.
type ObjectInfo struct {
	ContentType string
	ByteSize    int64
	Exists      bool
}

// UploadAuth is a short-lived, single-object upload grant. ObjectKey stays on
// the server side of this struct: it is needed to verify completion later but is
// never projected into a client response.
type UploadAuth struct {
	Method    string
	URL       string
	ExpiresAt time.Time
	Headers   map[string]string
	ObjectKey string
}

// Storage abstracts the object store behind the media pipeline. No permanent
// provider credential ever leaves this port, and the domain never learns which
// provider is behind it.
type Storage interface {
	// CreateUploadAuthorization grants short-lived write access to exactly one
	// object key.
	CreateUploadAuthorization(
		ctx context.Context,
		objectKey string,
		contentType string,
		maxBytes int64,
		ttl time.Duration,
	) (UploadAuth, error)

	// HeadObject reports what the provider actually stored under objectKey.
	HeadObject(ctx context.Context, objectKey string) (ObjectInfo, error)

	// PutObject writes a server-generated object (canonical master or variant).
	PutObject(ctx context.Context, objectKey string, contentType string, body []byte) error

	// GetObject reads an object back for processing.
	GetObject(ctx context.Context, objectKey string) ([]byte, string, error)
}

// ProcessedImage is the output of a processing step: the decoded, normalized
// bytes plus the dimensions the caller must persist.
type ProcessedImage struct {
	ContentType string
	Bytes       []byte
	Width       int
	Height      int
}

// ImageProcessor abstracts decode, normalization and variant generation. The
// compression provider sits behind this port, never in the service or handler.
type ImageProcessor interface {
	// ValidateAndNormalize turns an untrusted raw upload into the safe canonical
	// master. The declared type is a hint only.
	ValidateAndNormalize(ctx context.Context, raw []byte, declaredType string) (ProcessedImage, error)

	// GenerateVariant derives one transform profile from the canonical master.
	GenerateVariant(ctx context.Context, master []byte, profile string) (ProcessedImage, error)
}

// AdvertRef and RelationRow are aliases of the domain projections. They are
// aliases rather than local structs so the postgres repository can satisfy this
// interface without importing the application package.
type (
	AdvertRef   = domainmedia.AdvertRef
	RelationRow = domainmedia.RelationWithAsset
)

// Repository persists media assets, variants, advert relations and the durable
// media jobs.
//
// Every owner-scoped read/write is filtered by owner id, so another user's asset
// or advert is indistinguishable from a missing one (NOT_FOUND, never a leak).
type Repository interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	WithTx(tx pgx.Tx) Repository

	CreateAsset(ctx context.Context, a domainmedia.Asset) error
	FindAssetByIDForOwner(ctx context.Context, ownerID, assetID uuid.UUID) (domainmedia.Asset, error)
	FindAssetByIDForOwnerForUpdate(ctx context.Context, ownerID, assetID uuid.UUID) (domainmedia.Asset, error)

	// FindAssetByID and FindAssetByIDForUpdate are worker-side reads: a job
	// carries an asset id and has no session owner to scope by.
	FindAssetByID(ctx context.Context, assetID uuid.UUID) (domainmedia.Asset, error)
	FindAssetByIDForUpdate(ctx context.Context, assetID uuid.UUID) (domainmedia.Asset, error)

	// UpdateAssetLifecycle moves an asset between two non-terminal lifecycles.
	UpdateAssetLifecycle(
		ctx context.Context,
		assetID uuid.UUID,
		from, to domainmedia.AssetLifecycle,
		now time.Time,
	) (domainmedia.Asset, error)

	SetAssetUploaded(
		ctx context.Context,
		assetID uuid.UUID,
		rawObjectKey string,
		now time.Time,
	) (domainmedia.Asset, error)

	SetAssetValidating(ctx context.Context, assetID uuid.UUID, now time.Time) (domainmedia.Asset, error)

	// SetAssetMasterReady writes every field the MASTER_READY table CHECK
	// requires in one statement.
	SetAssetMasterReady(
		ctx context.Context,
		assetID uuid.UUID,
		masterObjectKey string,
		contentType string,
		byteSize int64,
		width, height int,
		now time.Time,
	) (domainmedia.Asset, error)

	SetAssetValidationFailed(
		ctx context.Context,
		assetID uuid.UUID,
		reason string,
		now time.Time,
	) (domainmedia.Asset, error)

	UpsertPendingVariant(ctx context.Context, v domainmedia.Variant) (domainmedia.Variant, error)
	ListVariantsByAsset(ctx context.Context, assetID uuid.UUID) ([]domainmedia.Variant, error)

	MarkVariantReady(
		ctx context.Context,
		assetID uuid.UUID,
		profile string,
		objectKey string,
		contentType string,
		byteSize int64,
		width, height int,
		now time.Time,
	) (domainmedia.Variant, error)

	MarkVariantFailed(
		ctx context.Context,
		assetID uuid.UUID,
		profile string,
		reason string,
		now time.Time,
	) (domainmedia.Variant, error)

	ListAdvertMediaByAdvert(ctx context.Context, advertID uuid.UUID) ([]RelationRow, error)
	CountAdvertMediaByAdvert(ctx context.Context, advertID uuid.UUID) (int, error)
	AttachAdvertMedia(ctx context.Context, rel domainmedia.AdvertMediaRelation) error

	// DetachAdvertMedia removes the relation and reports whether a row was
	// removed and whether that row was the cover.
	DetachAdvertMedia(ctx context.Context, advertID, assetID uuid.UUID) (removed bool, wasCover bool, err error)

	// UpdateAdvertMediaDisplayOrder rewrites one relation's order. Reorder runs
	// it twice per row (temporary orders first) to stay inside the unique
	// (advert_id, display_order) index.
	UpdateAdvertMediaDisplayOrder(
		ctx context.Context,
		advertID, assetID uuid.UUID,
		displayOrder int,
		now time.Time,
	) error

	ClearAdvertCover(ctx context.Context, advertID uuid.UUID, now time.Time) error
	SetAdvertCover(ctx context.Context, advertID, assetID uuid.UUID, now time.Time) error

	FindOwnerAdvertForUpdate(ctx context.Context, ownerID, advertID uuid.UUID) (AdvertRef, error)

	// BumpAdvertMediaVersion increments media_version under an optimistic guard
	// and returns the new value.
	BumpAdvertMediaVersion(
		ctx context.Context,
		ownerID, advertID uuid.UUID,
		expectedMediaVersion int,
		now time.Time,
	) (int, error)

	// EnqueueJob inserts a durable job. A duplicate deduplication key is a
	// CONFLICT, which callers treat as "already enqueued".
	EnqueueJob(ctx context.Context, job domainmedia.BackgroundJob) error
	FindJobByDedupKey(ctx context.Context, key string) (domainmedia.BackgroundJob, error)
}

// Clock provides the current time.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

const storageNotConfiguredMessage = "Görsel yükleme servisi şu anda kullanılamıyor."

// UnconfiguredStorage is the production default until a real provider adapter is
// wired. Every call reports the dependency as unavailable, so no code path can
// silently behave as if uploads worked.
type UnconfiguredStorage struct{}

// CreateUploadAuthorization reports the storage dependency as unavailable.
func (UnconfiguredStorage) CreateUploadAuthorization(
	context.Context, string, string, int64, time.Duration,
) (UploadAuth, error) {
	return UploadAuth{}, apperr.DependencyUnavailable(storageNotConfiguredMessage)
}

// HeadObject reports the storage dependency as unavailable.
func (UnconfiguredStorage) HeadObject(context.Context, string) (ObjectInfo, error) {
	return ObjectInfo{}, apperr.DependencyUnavailable(storageNotConfiguredMessage)
}

// PutObject reports the storage dependency as unavailable.
func (UnconfiguredStorage) PutObject(context.Context, string, string, []byte) error {
	return apperr.DependencyUnavailable(storageNotConfiguredMessage)
}

// GetObject reports the storage dependency as unavailable.
func (UnconfiguredStorage) GetObject(context.Context, string) ([]byte, string, error) {
	return nil, "", apperr.DependencyUnavailable(storageNotConfiguredMessage)
}

const processorNotConfiguredMessage = "Görsel işleme servisi şu anda kullanılamıyor."

// UnconfiguredImageProcessor is the production default until a real processing
// adapter is wired.
type UnconfiguredImageProcessor struct{}

// ValidateAndNormalize reports the processing dependency as unavailable.
func (UnconfiguredImageProcessor) ValidateAndNormalize(
	context.Context, []byte, string,
) (ProcessedImage, error) {
	return ProcessedImage{}, apperr.DependencyUnavailable(processorNotConfiguredMessage)
}

// GenerateVariant reports the processing dependency as unavailable.
func (UnconfiguredImageProcessor) GenerateVariant(
	context.Context, []byte, string,
) (ProcessedImage, error) {
	return ProcessedImage{}, apperr.DependencyUnavailable(processorNotConfiguredMessage)
}

var (
	_ Storage        = UnconfiguredStorage{}
	_ ImageProcessor = UnconfiguredImageProcessor{}
)
