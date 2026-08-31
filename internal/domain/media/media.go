// Package media holds the shared media asset aggregate, its processing
// lifecycles and the advert relation rules.
//
// An asset is a physical file plus technical metadata. The advert relation is a
// separate concept that owns display order and the single cover flag. Raw
// upload, canonical master and per-profile variants are distinct objects with
// independent readiness: a MASTER_READY asset does not imply READY variants.
package media

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AssetLifecycle is the source-side processing state of an asset. Values match
// the hrd_media_assets.lifecycle_status CHECK constraint.
type AssetLifecycle string

const (
	AssetUploadPending     AssetLifecycle = "UPLOAD_PENDING"
	AssetUploaded          AssetLifecycle = "UPLOADED"
	AssetValidating        AssetLifecycle = "VALIDATING"
	AssetMasterReady       AssetLifecycle = "MASTER_READY"
	AssetValidationFailed  AssetLifecycle = "VALIDATION_FAILED"
	AssetCleanupCandidate  AssetLifecycle = "CLEANUP_CANDIDATE"
	AssetDeleting          AssetLifecycle = "DELETING"
	AssetPhysicallyDeleted AssetLifecycle = "PHYSICALLY_DELETED"
)

// Valid reports whether l is a known asset lifecycle value.
func (l AssetLifecycle) Valid() bool {
	switch l {
	case AssetUploadPending, AssetUploaded, AssetValidating, AssetMasterReady,
		AssetValidationFailed, AssetCleanupCandidate, AssetDeleting, AssetPhysicallyDeleted:
		return true
	}
	return false
}

// VariantLifecycle is the per-profile readiness of a derived variant. Values
// match the hrd_media_variants.lifecycle_status CHECK constraint.
type VariantLifecycle string

const (
	VariantPending           VariantLifecycle = "PENDING"
	VariantProcessing        VariantLifecycle = "PROCESSING"
	VariantReady             VariantLifecycle = "READY"
	VariantFailed            VariantLifecycle = "FAILED"
	VariantDeleting          VariantLifecycle = "DELETING"
	VariantPhysicallyDeleted VariantLifecycle = "PHYSICALLY_DELETED"
)

// Valid reports whether l is a known variant lifecycle value.
func (l VariantLifecycle) Valid() bool {
	switch l {
	case VariantPending, VariantProcessing, VariantReady, VariantFailed,
		VariantDeleting, VariantPhysicallyDeleted:
		return true
	}
	return false
}

// ProviderB2 is the storage provider recorded on an asset; it matches the
// hrd_media_assets.provider column default.
const ProviderB2 = "B2"

// Transform profile names come from the media decision document. Pixel sizes
// for advert variants are fixed production defaults; BANNER is compress-only
// and has no resize dimensions.
const (
	ProfileDetail   = "DETAIL"
	ProfileHomepage = "HOMEPAGE"
	ProfileSearch   = "SEARCH"
	ProfileBanner   = "BANNER"
)

// MaxUploadBytes is the server-side security ceiling for uploads (64 MiB).
const MaxUploadBytes int64 = 67108864

// Fixed advert transform dimensions (fit / aspect ratio preserved).
const (
	ProfileHomepageWidth  = 340
	ProfileHomepageHeight = 268
	ProfileDetailWidth    = 368
	ProfileDetailHeight   = 290
	ProfileSearchWidth    = 100
	ProfileSearchHeight   = 79
)

// ProfileDimensions holds width×height for a transform profile.
type ProfileDimensions struct {
	Width  int
	Height int
}

// DefaultProfileDimensions returns the fixed dimensions for known advert profiles.
func DefaultProfileDimensions(profile string) (ProfileDimensions, bool) {
	switch profile {
	case ProfileHomepage:
		return ProfileDimensions{Width: ProfileHomepageWidth, Height: ProfileHomepageHeight}, true
	case ProfileDetail:
		return ProfileDimensions{Width: ProfileDetailWidth, Height: ProfileDetailHeight}, true
	case ProfileSearch:
		return ProfileDimensions{Width: ProfileSearchWidth, Height: ProfileSearchHeight}, true
	default:
		return ProfileDimensions{}, false
	}
}

// RequiredTransformProfiles lists the profiles generated from every canonical
// master. Each profile succeeds or fails independently.
func RequiredTransformProfiles() []string {
	return []string{ProfileDetail, ProfileHomepage, ProfileSearch}
}

// IsKnownTransformProfile reports whether profile is a generated variant profile.
// BANNER is compress-only (no resize bounds); advert profiles use fit resize.
func IsKnownTransformProfile(profile string) bool {
	switch profile {
	case ProfileDetail, ProfileHomepage, ProfileSearch, ProfileBanner:
		return true
	}
	return false
}

// IsKnownDeliveryProfile reports whether profile may appear in a public delivery URL.
func IsKnownDeliveryProfile(profile string) bool {
	return IsKnownTransformProfile(profile)
}

// IsSoftDeletedAssetLifecycle reports whether the asset must not be publicly served.
func IsSoftDeletedAssetLifecycle(l AssetLifecycle) bool {
	switch l {
	case AssetCleanupCandidate, AssetDeleting, AssetPhysicallyDeleted:
		return true
	}
	return false
}

// IsOwnedVariantObjectKey reports whether objectKey is the Haradan-owned variant
// key for assetID/profile (never raw or master).
func IsOwnedVariantObjectKey(assetID uuid.UUID, profile, objectKey string) bool {
	if assetID == uuid.Nil || !IsKnownDeliveryProfile(profile) {
		return false
	}
	want := VariantObjectKey(assetID, profile)
	key := strings.TrimSpace(objectKey)
	return key == want
}

// GeneratedTransformProfiles lists every variant enqueued after MASTER_READY:
// advert fit profiles plus the compress-only BANNER profile.
func GeneratedTransformProfiles() []string {
	return []string{ProfileDetail, ProfileHomepage, ProfileSearch, ProfileBanner}
}

// RawObjectKey returns the quarantine key for the client-uploaded file. Object
// keys are system generated from the asset id; the user file name is never used.
func RawObjectKey(assetID uuid.UUID) string {
	return "assets/" + assetID.String() + "/raw"
}

// MasterObjectKey returns the key of the safe canonical master object.
func MasterObjectKey(assetID uuid.UUID) string {
	return "assets/" + assetID.String() + "/master"
}

// VariantObjectKey returns the key of a derived variant object.
func VariantObjectKey(assetID uuid.UUID, profile string) string {
	return "assets/" + assetID.String() + "/variants/" + profile
}

// Asset mirrors hrd_media_assets. Object keys stay internal and are never part
// of a client projection.
type Asset struct {
	ID                uuid.UUID
	OwnerUserID       uuid.UUID
	Provider          string
	RawObjectKey      *string
	MasterObjectKey   *string
	ContentType       *string
	ByteSize          *int64
	ChecksumSHA256    *string
	WidthPx           *int
	HeightPx          *int
	LifecycleStatus   AssetLifecycle
	TechnicalMetadata json.RawMessage
	FailureReason     *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Variant mirrors hrd_media_variants: one row per (asset, transform profile).
type Variant struct {
	ID                uuid.UUID
	AssetID           uuid.UUID
	TransformProfile  string
	ObjectKey         *string
	LifecycleStatus   VariantLifecycle
	WidthPx           *int
	HeightPx          *int
	ByteSize          *int64
	ContentType       *string
	FailureReason     *string
	TechnicalMetadata json.RawMessage
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// AdvertMediaRelation mirrors hrd_advert_media: the use of an asset inside one
// advert. Display order and the cover flag belong here, not to the asset.
type AdvertMediaRelation struct {
	ID           uuid.UUID
	AdvertID     int64
	AssetID      uuid.UUID
	DisplayOrder int
	IsCover      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// AdvertRef is the slice of an advert the media domain is allowed to read. The
// advert core aggregate owns everything else about that row.
type AdvertRef struct {
	ID           int64
	Status       string
	MediaVersion int
	DeletedAt    *time.Time
}

// AdvertMediaAccess is the advert attachment slice used for public delivery
// authorization (owner, status, soft-delete).
type AdvertMediaAccess struct {
	OwnerUserID uuid.UUID
	Status      string
	DeletedAt   *time.Time
}

// BannerMediaAccess is the banner attachment slice used for public delivery
// authorization. Status mirrors hrd_banners.status (ACTIVE/INACTIVE).
type BannerMediaAccess struct {
	Status string
}

// IsDeleted reports whether the advert is soft-deleted.
func (a AdvertRef) IsDeleted() bool { return a.DeletedAt != nil }

// RelationWithAsset is an advert media relation together with the lifecycle of
// the asset behind it, which is what both the owner projection and automatic
// cover promotion need.
type RelationWithAsset struct {
	Relation       AdvertMediaRelation
	AssetLifecycle AssetLifecycle
}

// JobType is a hrd_background_jobs.job_type value owned by the media domain.
type JobType string

const (
	JobValidateAndNormalize JobType = "MEDIA_VALIDATE_AND_NORMALIZE"
	JobGenerateVariant      JobType = "MEDIA_GENERATE_VARIANT"
	JobDeleteObjects        JobType = "MEDIA_DELETE_OBJECTS"
	JobReconcile            JobType = "MEDIA_RECONCILE"

	JobNotificationFanoutPackageAdvert  JobType = "NOTIFICATION_FANOUT_PACKAGE_ADVERT"
	JobNotificationFanoutAdvancedAdvert JobType = "NOTIFICATION_FANOUT_ADVANCED_ADVERT" // historical
	JobNotificationFanoutUrgentAdvert   JobType = "NOTIFICATION_FANOUT_URGENT_ADVERT"
	JobEmailSendAdvertNotificationChunk JobType = "EMAIL_SEND_ADVERT_NOTIFICATION_CHUNK"
	JobPackageExpiryReminderScan        JobType = "PACKAGE_EXPIRY_REMINDER_SCAN"
	JobEmailSendPackageExpiryReminder   JobType = "EMAIL_SEND_PACKAGE_EXPIRY_REMINDER"
)

// Valid reports whether t is a known background job type owned by the media package.
func (t JobType) Valid() bool {
	switch t {
	case JobValidateAndNormalize, JobGenerateVariant, JobDeleteObjects, JobReconcile,
		JobNotificationFanoutPackageAdvert, JobNotificationFanoutAdvancedAdvert,
		JobNotificationFanoutUrgentAdvert,
		JobEmailSendAdvertNotificationChunk, JobPackageExpiryReminderScan,
		JobEmailSendPackageExpiryReminder:
		return true
	}
	return false
}

// IsNotificationJob reports whether t is a notification/email fan-out job type.
func (t JobType) IsNotificationJob() bool {
	switch t {
	case JobNotificationFanoutPackageAdvert, JobNotificationFanoutAdvancedAdvert,
		JobNotificationFanoutUrgentAdvert,
		JobEmailSendAdvertNotificationChunk, JobPackageExpiryReminderScan,
		JobEmailSendPackageExpiryReminder:
		return true
	}
	return false
}

// JobStatus is a hrd_background_jobs.status value.
type JobStatus string

const (
	JobQueued    JobStatus = "QUEUED"
	JobLeased    JobStatus = "LEASED"
	JobSucceeded JobStatus = "SUCCEEDED"
	JobFailed    JobStatus = "FAILED"
	JobCancelled JobStatus = "CANCELLED"
	JobDead      JobStatus = "DEAD"
)

// Valid reports whether s is a known job status.
func (s JobStatus) Valid() bool {
	switch s {
	case JobQueued, JobLeased, JobSucceeded, JobFailed, JobCancelled, JobDead:
		return true
	}
	return false
}

// BackgroundJob mirrors the hrd_background_jobs essentials a media job needs.
// tjk_sync_run_id is absent on purpose: the table CHECK requires it to be NULL
// for every non-TJK job type.
type BackgroundJob struct {
	ID                uuid.UUID
	JobType           JobType
	Status            JobStatus
	Payload           json.RawMessage
	DeduplicationKey  *string
	AttemptCount      int
	MaxAttempts       int
	AvailableAt       time.Time
	LeasedUntil       *time.Time
	LeaseOwner        *string
	LastError         *string
	CancelRequestedAt *time.Time
	Version           int
	CreatedAt         time.Time
	UpdatedAt         time.Time
	CompletedAt       *time.Time
}

// ValidateJobDedupKey is the deterministic dedup key for the validate job of one
// asset, so a repeated upload completion cannot enqueue a duplicate.
func ValidateJobDedupKey(assetID uuid.UUID) string {
	return string(JobValidateAndNormalize) + ":" + assetID.String()
}

// VariantJobDedupKey is the deterministic dedup key for one (asset, profile)
// variant job; the same master and profile are never processed twice.
func VariantJobDedupKey(assetID uuid.UUID, profile string) string {
	return string(JobGenerateVariant) + ":" + assetID.String() + ":" + profile
}

// AdvertEditableForMedia reports whether the owner may add, remove, reorder or
// re-cover advert media in the given advert status. Phase one allows media edits
// in DRAFT and CHANGES_REQUESTED only.
func AdvertEditableForMedia(status string) bool {
	return status == "DRAFT" || status == "CHANGES_REQUESTED"
}

// AttachableAssetLifecycles lists the lifecycles an asset may have when it is
// attached to an advert. A still-processing asset may be attached; submit-time
// validation is what requires READY variants.
func AttachableAssetLifecycles() []AssetLifecycle {
	return []AssetLifecycle{AssetUploadPending, AssetUploaded, AssetValidating, AssetMasterReady}
}

// IsAttachableAssetLifecycle reports whether an asset in lifecycle l may be
// attached to an advert.
func IsAttachableAssetLifecycle(l AssetLifecycle) bool {
	switch l {
	case AssetUploadPending, AssetUploaded, AssetValidating, AssetMasterReady:
		return true
	}
	return false
}

// EmptyMetadata returns the canonical empty technical metadata object.
func EmptyMetadata() json.RawMessage { return json.RawMessage(`{}`) }

// PublicDeliveryURL returns the stable same-origin media delivery path for an
// asset profile. Object keys are never part of the public URL.
func PublicDeliveryURL(assetID uuid.UUID, profile string) string {
	profile = strings.TrimSpace(profile)
	if assetID == uuid.Nil || profile == "" {
		return ""
	}
	return "/v1/media/" + assetID.String() + "/" + profile
}
