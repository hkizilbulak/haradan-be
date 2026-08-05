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
	"net/url"
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

// Transform profile names come from the media decision document. Pixel sizes,
// crop behaviour and compression quality are configuration, not domain values,
// and are deliberately absent here.
const (
	ProfileDetail   = "DETAIL"
	ProfileHomepage = "HOMEPAGE"
	ProfileSearch   = "SEARCH"
)

// RequiredTransformProfiles lists the profiles generated from every canonical
// master. Each profile succeeds or fails independently.
func RequiredTransformProfiles() []string {
	return []string{ProfileDetail, ProfileHomepage, ProfileSearch}
}

// IsKnownTransformProfile reports whether profile is a phase-one profile name.
func IsKnownTransformProfile(profile string) bool {
	switch profile {
	case ProfileDetail, ProfileHomepage, ProfileSearch:
		return true
	}
	return false
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
	AdvertID     uuid.UUID
	AssetID      uuid.UUID
	DisplayOrder int
	IsCover      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// AdvertRef is the slice of an advert the media domain is allowed to read. The
// advert core aggregate owns everything else about that row.
type AdvertRef struct {
	ID           uuid.UUID
	Status       string
	MediaVersion int
	DeletedAt    *time.Time
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

	JobNotificationFanoutAdvancedAdvert JobType = "NOTIFICATION_FANOUT_ADVANCED_ADVERT"
	JobNotificationFanoutUrgentAdvert   JobType = "NOTIFICATION_FANOUT_URGENT_ADVERT"
	JobEmailSendAdvertNotificationChunk JobType = "EMAIL_SEND_ADVERT_NOTIFICATION_CHUNK"
	JobPackageExpiryReminderScan        JobType = "PACKAGE_EXPIRY_REMINDER_SCAN"
	JobEmailSendPackageExpiryReminder   JobType = "EMAIL_SEND_PACKAGE_EXPIRY_REMINDER"
)

// Valid reports whether t is a known background job type owned by the media package.
func (t JobType) Valid() bool {
	switch t {
	case JobValidateAndNormalize, JobGenerateVariant, JobDeleteObjects, JobReconcile,
		JobNotificationFanoutAdvancedAdvert, JobNotificationFanoutUrgentAdvert,
		JobEmailSendAdvertNotificationChunk, JobPackageExpiryReminderScan,
		JobEmailSendPackageExpiryReminder:
		return true
	}
	return false
}

// IsNotificationJob reports whether t is a notification/email fan-out job type.
func (t JobType) IsNotificationJob() bool {
	switch t {
	case JobNotificationFanoutAdvancedAdvert, JobNotificationFanoutUrgentAdvert,
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

// PublicURL joins a validated public media origin with a generated object key.
// Invalid origins and traversal-shaped keys are rejected rather than projected.
func PublicURL(baseURL, objectKey string) string {
	base := strings.TrimSpace(baseURL)
	key := strings.TrimSpace(objectKey)
	if base == "" || key == "" {
		return ""
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" ||
		u.RawQuery != "" || u.Fragment != "" || strings.HasPrefix(key, "/") ||
		strings.Contains(key, `\`) || strings.Contains(key, "..") {
		return ""
	}
	for _, segment := range strings.Split(key, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return ""
		}
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + key
	return u.String()
}
