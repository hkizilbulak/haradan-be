// Package media implements the MEDIA-01..07 owner use cases and the media
// worker steps behind them.
//
// Two invariants shape every projection in this file: object keys never reach a
// client, and a MASTER_READY asset says nothing about variant readiness. READY
// variants project PublicDeliveryURL relative paths (never object keys).
package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

// defaultJobMaxAttempts is an engineering default for the durable job retry
// budget, not a product decision; hrd_background_jobs.max_attempts must be > 0.
const defaultJobMaxAttempts = 5

// Service implements the owner-facing media use cases.
type Service struct {
	repo      Repository
	storage   Storage
	processor ImageProcessor
	clock     Clock

	allowedContentTypes []string
	maxByteSize         int64
	uploadURLTTL        time.Duration
}

// Config wires media application dependencies. Storage and Processor default to
// their unconfigured implementations so a half-wired process fails loudly with
// DEPENDENCY_UNAVAILABLE instead of pretending uploads work.
type Config struct {
	Repo      Repository
	Storage   Storage
	Processor ImageProcessor
	Clock     Clock

	// AllowedContentTypes and MaxByteSize come from configuration. While either
	// is unset, upload initiation reports the dependency as unavailable.
	AllowedContentTypes []string
	MaxByteSize         int64
	UploadURLTTL        time.Duration
}

// NewService constructs the media application service.
func NewService(cfg Config) (*Service, error) {
	if cfg.Repo == nil {
		return nil, fmt.Errorf("media service repository is required")
	}
	clock := cfg.Clock
	if clock == nil {
		clock = systemClock{}
	}
	storage := cfg.Storage
	if storage == nil {
		storage = UnconfiguredStorage{}
	}
	processor := cfg.Processor
	if processor == nil {
		processor = UnconfiguredImageProcessor{}
	}
	return &Service{
		repo:                cfg.Repo,
		storage:             storage,
		processor:           processor,
		clock:               clock,
		allowedContentTypes: normalizeContentTypes(cfg.AllowedContentTypes),
		maxByteSize:         cfg.MaxByteSize,
		uploadURLTTL:        cfg.UploadURLTTL,
	}, nil
}

// InitiateInput is MEDIA-01 input. Both hints are advisory: the client MIME type
// and size are never treated as canonical.
type InitiateInput struct {
	DeclaredContentType *string
	DeclaredByteSize    *int64
}

// UploadAuthView is the client-safe half of an upload grant: no object key and
// no permanent provider credential.
type UploadAuthView struct {
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	ExpiresAt time.Time         `json:"expiresAt"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// UploadConstraintsView tells the client what the server will accept.
type UploadConstraintsView struct {
	AllowedContentTypes []string `json:"allowedContentTypes"`
	MaxByteSize         int64    `json:"maxByteSize"`
	RequiredHeaders     []string `json:"requiredHeaders"`
}

// InitiateView is MEDIA-01 output.
type InitiateView struct {
	AssetID     uuid.UUID             `json:"assetId"`
	Upload      UploadAuthView        `json:"upload"`
	Constraints UploadConstraintsView `json:"constraints"`
}

// VariantStatusView is the per-profile readiness of one variant. PublicURL is
// the same-origin delivery path for READY variants; object keys never appear.
type VariantStatusView struct {
	TransformProfile string                       `json:"transformProfile"`
	LifecycleStatus  domainmedia.VariantLifecycle `json:"lifecycleStatus"`
	PublicURL        *string                      `json:"publicUrl,omitempty"`
	Usage            *string                      `json:"usage,omitempty"`
}

// ProcessingView is MEDIA-02/MEDIA-03 output: lifecycle plus per-variant state,
// never raw or master object keys.
type ProcessingView struct {
	AssetID         uuid.UUID                  `json:"assetId"`
	LifecycleStatus domainmedia.AssetLifecycle `json:"lifecycleStatus"`
	FailureCode     *string                    `json:"failureCode,omitempty"`
	FailureMessage  *string                    `json:"failureMessage,omitempty"`
	Variants        []VariantStatusView        `json:"variants"`
}

// RelationItemView is one advert media relation as its owner sees it.
type RelationItemView struct {
	AssetID         uuid.UUID                  `json:"assetId"`
	DisplayOrder    int                        `json:"displayOrder"`
	IsCover         bool                       `json:"isCover"`
	LifecycleStatus domainmedia.AssetLifecycle `json:"lifecycleStatus"`
}

// OwnerMediaView is MEDIA-04..07 output: the whole relation list plus the new
// media version the client must send back next time.
type OwnerMediaView struct {
	AdvertID     uuid.UUID          `json:"advertId"`
	MediaVersion int                `json:"mediaVersion"`
	Items        []RelationItemView `json:"items"`
}

// InitiateMediaUpload implements MEDIA-01. It creates an UPLOAD_PENDING asset
// with a system-generated object key and returns a short-lived direct upload
// authorization. Two calls intentionally create two assets: there is no client
// idempotency key, and abandoned assets are cleaned up later.
func (s *Service) InitiateMediaUpload(
	ctx context.Context,
	ownerID uuid.UUID,
	in InitiateInput,
) (InitiateView, error) {
	maxBytes := effectiveMaxByteSize(s.maxByteSize)
	if len(s.allowedContentTypes) == 0 {
		return InitiateView{}, apperr.DependencyUnavailable(mediaNotConfiguredMessage)
	}
	declaredType, err := validateDeclaredContentType(s.allowedContentTypes, in.DeclaredContentType)
	if err != nil {
		return InitiateView{}, err
	}
	if err := validateDeclaredByteSize(maxBytes, in.DeclaredByteSize); err != nil {
		return InitiateView{}, err
	}

	now := s.clock.Now()
	assetID := uuid.New()
	rawKey := domainmedia.RawObjectKey(assetID)

	// The declared hints are kept as metadata only. content_type and the pixel
	// columns stay NULL until the worker derives them from the real bytes.
	asset := domainmedia.Asset{
		ID:                assetID,
		OwnerUserID:       ownerID,
		Provider:          domainmedia.ProviderB2,
		RawObjectKey:      &rawKey,
		LifecycleStatus:   domainmedia.AssetUploadPending,
		TechnicalMetadata: encodeAssetHints(assetHints{DeclaredContentType: declaredType, DeclaredByteSize: in.DeclaredByteSize}),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	// A single insert is already atomic; the provider authorization is issued
	// afterwards, outside any transaction. If it fails the asset simply stays
	// UPLOAD_PENDING and the client may initiate again.
	if err := s.repo.CreateAsset(ctx, asset); err != nil {
		return InitiateView{}, err
	}

	auth, err := s.storage.CreateUploadAuthorization(ctx, rawKey, declaredType, maxBytes, s.uploadURLTTL)
	if err != nil {
		return InitiateView{}, err
	}

	return InitiateView{
		AssetID: assetID,
		Upload: UploadAuthView{
			Method:    auth.Method,
			URL:       auth.URL,
			ExpiresAt: auth.ExpiresAt,
			Headers:   auth.Headers,
		},
		Constraints: UploadConstraintsView{
			AllowedContentTypes: append([]string(nil), s.allowedContentTypes...),
			MaxByteSize:         maxBytes,
			RequiredHeaders:     headerNames(auth.Headers),
		},
	}, nil
}

// ConfirmMediaUpload implements MEDIA-02. The provider HEAD check runs outside
// the transaction; the short transaction then moves the asset to UPLOADED and
// enqueues the validate job together, so an UPLOADED asset is never left without
// a job. Repeating the call is idempotent.
func (s *Service) ConfirmMediaUpload(ctx context.Context, ownerID, assetID uuid.UUID) (ProcessingView, error) {
	if err := requireAssetID(assetID); err != nil {
		return ProcessingView{}, err
	}
	current, err := s.repo.FindAssetByIDForOwner(ctx, ownerID, assetID)
	if err != nil {
		return ProcessingView{}, err
	}
	switch current.LifecycleStatus {
	case domainmedia.AssetUploadPending:
		// proceed
	case domainmedia.AssetUploaded, domainmedia.AssetValidating, domainmedia.AssetMasterReady:
		// Already confirmed: report the current state without a second write.
		return s.processingView(ctx, current)
	default:
		return ProcessingView{}, apperr.InvalidState(assetNotConfirmableMessage)
	}

	// Without MIME limits we cannot validate provider metadata; fail closed
	// rather than accepting an object under invented ceilings.
	if len(s.allowedContentTypes) == 0 {
		return ProcessingView{}, apperr.DependencyUnavailable(mediaNotConfiguredMessage)
	}

	rawKey := domainmedia.RawObjectKey(assetID)
	info, err := s.storage.HeadObject(ctx, rawKey)
	if err != nil {
		return ProcessingView{}, err
	}
	if err := validateStoredObject(s.allowedContentTypes, s.maxByteSize, info); err != nil {
		return ProcessingView{}, err
	}

	now := s.clock.Now()
	var confirmed domainmedia.Asset
	err = s.withTx(ctx, func(ctx context.Context, repo Repository) error {
		locked, err := repo.FindAssetByIDForOwnerForUpdate(ctx, ownerID, assetID)
		if err != nil {
			return err
		}
		switch locked.LifecycleStatus {
		case domainmedia.AssetUploadPending:
			// proceed
		case domainmedia.AssetUploaded, domainmedia.AssetValidating, domainmedia.AssetMasterReady:
			confirmed = locked
			return nil
		default:
			return apperr.InvalidState(assetNotConfirmableMessage)
		}

		confirmed, err = repo.SetAssetUploaded(ctx, assetID, rawKey, now)
		if err != nil {
			return err
		}
		return enqueueIgnoringDuplicate(ctx, repo, validateJob(assetID, now))
	})
	if err != nil {
		return ProcessingView{}, err
	}
	return s.processingView(ctx, confirmed)
}

// GetMediaProcessingStatus implements MEDIA-03.
func (s *Service) GetMediaProcessingStatus(ctx context.Context, ownerID, assetID uuid.UUID) (ProcessingView, error) {
	if err := requireAssetID(assetID); err != nil {
		return ProcessingView{}, err
	}
	asset, err := s.repo.FindAssetByIDForOwner(ctx, ownerID, assetID)
	if err != nil {
		return ProcessingView{}, err
	}
	return s.processingView(ctx, asset)
}

// AttachMediaToAdvert implements MEDIA-04. A still-processing asset may be
// attached; submit-time validation is what demands READY variants.
func (s *Service) AttachMediaToAdvert(
	ctx context.Context,
	ownerID, advertID, assetID uuid.UUID,
	displayOrder *int,
	expectedMediaVersion int,
) (OwnerMediaView, error) {
	if err := requireAssetID(assetID); err != nil {
		return OwnerMediaView{}, err
	}
	if err := requireExpectedMediaVersion(expectedMediaVersion); err != nil {
		return OwnerMediaView{}, err
	}
	if err := validateDisplayOrder(displayOrder); err != nil {
		return OwnerMediaView{}, err
	}

	now := s.clock.Now()
	var view OwnerMediaView
	err := s.withTx(ctx, func(ctx context.Context, repo Repository) error {
		if err := guardAdvertForMediaEdit(ctx, repo, ownerID, advertID, expectedMediaVersion); err != nil {
			return err
		}
		// The asset is locked and its lifecycle re-checked inside the same
		// transaction, so a concurrent cleanup cannot slip underneath.
		asset, err := repo.FindAssetByIDForOwnerForUpdate(ctx, ownerID, assetID)
		if err != nil {
			return err
		}
		if !domainmedia.IsAttachableAssetLifecycle(asset.LifecycleStatus) {
			return apperr.InvalidState(assetNotAttachableMessage)
		}

		rows, err := repo.ListAdvertMediaByAdvert(ctx, advertID)
		if err != nil {
			return err
		}
		hasCover := false
		nextOrder := 0
		for _, row := range rows {
			if row.Relation.AssetID == assetID {
				return apperr.Conflict(assetAlreadyAttached)
			}
			if row.Relation.IsCover {
				hasCover = true
			}
			if row.Relation.DisplayOrder >= nextOrder {
				nextOrder = row.Relation.DisplayOrder + 1
			}
		}
		order := nextOrder
		if displayOrder != nil {
			order = *displayOrder
			for _, row := range rows {
				if row.Relation.DisplayOrder == order {
					return apperr.Conflict(displayOrderTakenMessage)
				}
			}
		}

		// The first attached image whose master is ready becomes the cover; the
		// owner may change it later.
		isCover := !hasCover && asset.LifecycleStatus == domainmedia.AssetMasterReady
		if err := repo.AttachAdvertMedia(ctx, domainmedia.AdvertMediaRelation{
			ID:           uuid.New(),
			AdvertID:     advertID,
			AssetID:      assetID,
			DisplayOrder: order,
			IsCover:      isCover,
			CreatedAt:    now,
			UpdatedAt:    now,
		}); err != nil {
			return err
		}
		view, err = bumpAndProject(ctx, repo, ownerID, advertID, expectedMediaVersion, now)
		return err
	})
	if err != nil {
		return OwnerMediaView{}, err
	}
	return view, nil
}

// DetachMediaFromAdvert implements MEDIA-05. Detaching only breaks the relation:
// it is not an asset deletion and triggers no immediate object removal.
func (s *Service) DetachMediaFromAdvert(
	ctx context.Context,
	ownerID, advertID, assetID uuid.UUID,
	expectedMediaVersion int,
) (OwnerMediaView, error) {
	if err := requireAssetID(assetID); err != nil {
		return OwnerMediaView{}, err
	}
	if err := requireExpectedMediaVersion(expectedMediaVersion); err != nil {
		return OwnerMediaView{}, err
	}

	now := s.clock.Now()
	var view OwnerMediaView
	err := s.withTx(ctx, func(ctx context.Context, repo Repository) error {
		advert, err := guardAdvertForMediaEditRef(ctx, repo, ownerID, advertID, expectedMediaVersion)
		if err != nil {
			return err
		}
		removed, wasCover, err := repo.DetachAdvertMedia(ctx, advertID, assetID)
		if err != nil {
			return err
		}
		if !removed {
			// Already detached: idempotent success with no second write.
			view, err = s.projectAdvertMedia(ctx, repo, advertID, advert.MediaVersion)
			return err
		}
		if wasCover {
			if err := promoteCover(ctx, repo, advertID, now); err != nil {
				return err
			}
		}
		view, err = bumpAndProject(ctx, repo, ownerID, advertID, expectedMediaVersion, now)
		return err
	})
	if err != nil {
		return OwnerMediaView{}, err
	}
	return view, nil
}

// ReorderAdvertMedia implements MEDIA-06. The ordering must cover exactly the
// attached assets, and it is written in two phases so the unique
// (advert_id, display_order) index is never violated mid-update.
func (s *Service) ReorderAdvertMedia(
	ctx context.Context,
	ownerID, advertID uuid.UUID,
	orderedAssetIDs []uuid.UUID,
	expectedMediaVersion int,
) (OwnerMediaView, error) {
	if err := requireExpectedMediaVersion(expectedMediaVersion); err != nil {
		return OwnerMediaView{}, err
	}
	if err := validateOrderedAssetIDs(orderedAssetIDs); err != nil {
		return OwnerMediaView{}, err
	}

	now := s.clock.Now()
	var view OwnerMediaView
	err := s.withTx(ctx, func(ctx context.Context, repo Repository) error {
		if err := guardAdvertForMediaEdit(ctx, repo, ownerID, advertID, expectedMediaVersion); err != nil {
			return err
		}
		rows, err := repo.ListAdvertMediaByAdvert(ctx, advertID)
		if err != nil {
			return err
		}
		if err := requireExactRelationSet(rows, orderedAssetIDs); err != nil {
			return err
		}

		// The table forbids negative display_order, so the temporary window sits
		// above every existing order and above the final range instead.
		offset := len(orderedAssetIDs)
		for _, row := range rows {
			if row.Relation.DisplayOrder >= offset {
				offset = row.Relation.DisplayOrder + 1
			}
		}
		for i, id := range orderedAssetIDs {
			if err := repo.UpdateAdvertMediaDisplayOrder(ctx, advertID, id, offset+i, now); err != nil {
				return err
			}
		}
		for i, id := range orderedAssetIDs {
			if err := repo.UpdateAdvertMediaDisplayOrder(ctx, advertID, id, i, now); err != nil {
				return err
			}
		}
		view, err = bumpAndProject(ctx, repo, ownerID, advertID, expectedMediaVersion, now)
		return err
	})
	if err != nil {
		return OwnerMediaView{}, err
	}
	return view, nil
}

// SetAdvertCover implements MEDIA-07. The old cover is cleared and the new one
// set inside a single transaction, which the one-cover partial unique index
// requires.
func (s *Service) SetAdvertCover(
	ctx context.Context,
	ownerID, advertID, assetID uuid.UUID,
	expectedMediaVersion int,
) (OwnerMediaView, error) {
	if err := requireAssetID(assetID); err != nil {
		return OwnerMediaView{}, err
	}
	if err := requireExpectedMediaVersion(expectedMediaVersion); err != nil {
		return OwnerMediaView{}, err
	}

	now := s.clock.Now()
	var view OwnerMediaView
	err := s.withTx(ctx, func(ctx context.Context, repo Repository) error {
		advert, err := guardAdvertForMediaEditRef(ctx, repo, ownerID, advertID, expectedMediaVersion)
		if err != nil {
			return err
		}
		rows, err := repo.ListAdvertMediaByAdvert(ctx, advertID)
		if err != nil {
			return err
		}
		attached := false
		alreadyCover := false
		for _, row := range rows {
			if row.Relation.AssetID == assetID {
				attached = true
				alreadyCover = row.Relation.IsCover
				break
			}
		}
		if !attached {
			return apperr.InvalidState(assetNotAttachedMessage)
		}
		if alreadyCover {
			// Already the cover: idempotent success with no second write.
			view, err = s.projectAdvertMedia(ctx, repo, advertID, advert.MediaVersion)
			return err
		}
		if err := repo.ClearAdvertCover(ctx, advertID, now); err != nil {
			return err
		}
		if err := repo.SetAdvertCover(ctx, advertID, assetID, now); err != nil {
			return err
		}
		view, err = bumpAndProject(ctx, repo, ownerID, advertID, expectedMediaVersion, now)
		return err
	})
	if err != nil {
		return OwnerMediaView{}, err
	}
	return view, nil
}

// promoteCover picks the first remaining relation in display order whose asset
// has a ready master. Variant readiness is per profile and may still be pending,
// so MASTER_READY is the strongest signal available at this point. When nothing
// qualifies the advert simply stays without a cover.
func promoteCover(ctx context.Context, repo Repository, advertID uuid.UUID, now time.Time) error {
	rows, err := repo.ListAdvertMediaByAdvert(ctx, advertID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.AssetLifecycle == domainmedia.AssetMasterReady {
			return repo.SetAdvertCover(ctx, advertID, row.Relation.AssetID, now)
		}
	}
	return nil
}

// guardAdvertForMediaEdit rejects a foreign, deleted, locked-down or stale
// advert before any media row is touched.
func guardAdvertForMediaEdit(
	ctx context.Context,
	repo Repository,
	ownerID, advertID uuid.UUID,
	expectedMediaVersion int,
) error {
	_, err := guardAdvertForMediaEditRef(ctx, repo, ownerID, advertID, expectedMediaVersion)
	return err
}

func guardAdvertForMediaEditRef(
	ctx context.Context,
	repo Repository,
	ownerID, advertID uuid.UUID,
	expectedMediaVersion int,
) (AdvertRef, error) {
	advert, err := repo.FindOwnerAdvertForUpdate(ctx, ownerID, advertID)
	if err != nil {
		return AdvertRef{}, err
	}
	if advert.IsDeleted() {
		return AdvertRef{}, apperr.InvalidState(deletedAdvertMessage)
	}
	if !domainmedia.AdvertEditableForMedia(advert.Status) {
		return AdvertRef{}, apperr.InvalidState(advertNotEditableMessage)
	}
	if advert.MediaVersion != expectedMediaVersion {
		return AdvertRef{}, apperr.StaleVersion(staleMediaVersionMessage)
	}
	return advert, nil
}

func bumpAndProject(
	ctx context.Context,
	repo Repository,
	ownerID, advertID uuid.UUID,
	expectedMediaVersion int,
	now time.Time,
) (OwnerMediaView, error) {
	newVersion, err := repo.BumpAdvertMediaVersion(ctx, ownerID, advertID, expectedMediaVersion, now)
	if err != nil {
		return OwnerMediaView{}, err
	}
	rows, err := repo.ListAdvertMediaByAdvert(ctx, advertID)
	if err != nil {
		return OwnerMediaView{}, err
	}
	return ownerMediaView(advertID, newVersion, rows), nil
}

func (s *Service) projectAdvertMedia(
	ctx context.Context,
	repo Repository,
	advertID uuid.UUID,
	mediaVersion int,
) (OwnerMediaView, error) {
	rows, err := repo.ListAdvertMediaByAdvert(ctx, advertID)
	if err != nil {
		return OwnerMediaView{}, err
	}
	return ownerMediaView(advertID, mediaVersion, rows), nil
}

func ownerMediaView(advertID uuid.UUID, mediaVersion int, rows []RelationRow) OwnerMediaView {
	items := make([]RelationItemView, 0, len(rows))
	for _, row := range rows {
		items = append(items, RelationItemView{
			AssetID:         row.Relation.AssetID,
			DisplayOrder:    row.Relation.DisplayOrder,
			IsCover:         row.Relation.IsCover,
			LifecycleStatus: row.AssetLifecycle,
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].DisplayOrder < items[j].DisplayOrder })
	return OwnerMediaView{AdvertID: advertID, MediaVersion: mediaVersion, Items: items}
}

// processingView projects an asset and its variants. No object key is copied
// into the result. READY variants expose PublicDeliveryURL relative paths.
func (s *Service) processingView(ctx context.Context, asset domainmedia.Asset) (ProcessingView, error) {
	variants, err := s.repo.ListVariantsByAsset(ctx, asset.ID)
	if err != nil {
		return ProcessingView{}, err
	}
	items := make([]VariantStatusView, 0, len(variants))
	for _, v := range variants {
		item := VariantStatusView{
			TransformProfile: v.TransformProfile,
			LifecycleStatus:  v.LifecycleStatus,
		}
		if v.LifecycleStatus == domainmedia.VariantReady && domainmedia.IsKnownDeliveryProfile(v.TransformProfile) {
			url := domainmedia.PublicDeliveryURL(asset.ID, v.TransformProfile)
			if url != "" {
				item.PublicURL = &url
			}
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].TransformProfile < items[j].TransformProfile })
	return ProcessingView{
		AssetID:         asset.ID,
		LifecycleStatus: asset.LifecycleStatus,
		FailureMessage:  asset.FailureReason,
		Variants:        items,
	}, nil
}

// assetHints keeps the non-canonical client declarations out of the trusted
// columns while staying available to the worker.
type assetHints struct {
	DeclaredContentType string `json:"declaredContentType,omitempty"`
	DeclaredByteSize    *int64 `json:"declaredByteSize,omitempty"`
}

func encodeAssetHints(h assetHints) json.RawMessage {
	raw, err := json.Marshal(h)
	if err != nil || len(raw) == 0 {
		return domainmedia.EmptyMetadata()
	}
	return raw
}

func decodeAssetHints(raw json.RawMessage) assetHints {
	var h assetHints
	if len(raw) == 0 {
		return h
	}
	if err := json.Unmarshal(raw, &h); err != nil {
		return assetHints{}
	}
	return h
}

func headerNames(headers map[string]string) []string {
	out := make([]string, 0, len(headers))
	for name := range headers {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// validateJob builds the deduplicated MEDIA_VALIDATE_AND_NORMALIZE job for one
// asset.
func validateJob(assetID uuid.UUID, now time.Time) domainmedia.BackgroundJob {
	key := domainmedia.ValidateJobDedupKey(assetID)
	return domainmedia.BackgroundJob{
		ID:               uuid.New(),
		JobType:          domainmedia.JobValidateAndNormalize,
		Status:           domainmedia.JobQueued,
		Payload:          jobPayload(assetID, ""),
		DeduplicationKey: &key,
		MaxAttempts:      defaultJobMaxAttempts,
		AvailableAt:      now,
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// variantJob builds the deduplicated MEDIA_GENERATE_VARIANT job for one
// (asset, profile) pair, so a retry never produces a duplicate variant.
func variantJob(assetID uuid.UUID, profile string, now time.Time) domainmedia.BackgroundJob {
	key := domainmedia.VariantJobDedupKey(assetID, profile)
	return domainmedia.BackgroundJob{
		ID:               uuid.New(),
		JobType:          domainmedia.JobGenerateVariant,
		Status:           domainmedia.JobQueued,
		Payload:          jobPayload(assetID, profile),
		DeduplicationKey: &key,
		MaxAttempts:      defaultJobMaxAttempts,
		AvailableAt:      now,
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func jobPayload(assetID uuid.UUID, profile string) json.RawMessage {
	payload := struct {
		AssetID          string `json:"assetId"`
		TransformProfile string `json:"transformProfile,omitempty"`
	}{AssetID: assetID.String(), TransformProfile: profile}
	raw, err := json.Marshal(payload)
	if err != nil {
		return domainmedia.EmptyMetadata()
	}
	return raw
}

// enqueueIgnoringDuplicate treats a dedup-key collision as success: the job the
// caller wanted is already queued.
func enqueueIgnoringDuplicate(ctx context.Context, repo Repository, job domainmedia.BackgroundJob) error {
	err := repo.EnqueueJob(ctx, job)
	if err == nil {
		return nil
	}
	if ae, ok := apperr.As(err); ok && ae.Code == apperr.CodeConflict {
		return nil
	}
	return err
}

func (s *Service) withTx(ctx context.Context, fn func(context.Context, Repository) error) error {
	return withTx(ctx, s.repo, fn)
}

func withTx(ctx context.Context, repo Repository, fn func(context.Context, Repository) error) error {
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(ctx, repo.WithTx(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		if errors.Is(err, pgx.ErrTxClosed) {
			return apperr.Internal(err)
		}
		return apperr.Internal(fmt.Errorf("commit media tx: %w", err))
	}
	return nil
}
