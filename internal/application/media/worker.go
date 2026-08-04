package media

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

// Worker implements MEDIA-WORKER-01 and MEDIA-WORKER-02: the steps that turn an
// untrusted raw upload into a safe canonical master and then into per-profile
// variants.
//
// Both steps are idempotent. An already ready asset or variant is left alone, so
// a redelivered job never produces a duplicate object. A permanent failure is
// recorded on the row instead of being surfaced as a retryable error; a
// transient dependency failure is returned so the caller can retry.
type Worker struct {
	repo      Repository
	storage   Storage
	processor ImageProcessor
	clock     Clock
}

// WorkerConfig wires media worker dependencies.
type WorkerConfig struct {
	Repo      Repository
	Storage   Storage
	Processor ImageProcessor
	Clock     Clock
}

// NewWorker constructs the media worker service.
func NewWorker(cfg WorkerConfig) (*Worker, error) {
	if cfg.Repo == nil {
		return nil, fmt.Errorf("media worker repository is required")
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
	return &Worker{repo: cfg.Repo, storage: storage, processor: processor, clock: clock}, nil
}

// ProcessValidateAndNormalize implements MEDIA-WORKER-01. It decodes and
// normalizes the raw upload, writes the canonical master, and in one transaction
// marks the asset MASTER_READY while enqueuing one variant job per required
// profile. The master being ready does not make any variant ready.
func (w *Worker) ProcessValidateAndNormalize(ctx context.Context, assetID uuid.UUID) error {
	asset, err := w.repo.FindAssetByID(ctx, assetID)
	if err != nil {
		return err
	}
	switch asset.LifecycleStatus {
	case domainmedia.AssetMasterReady:
		// Already normalized: nothing to redo.
		return nil
	case domainmedia.AssetUploaded, domainmedia.AssetValidating:
		// proceed
	default:
		return apperr.InvalidState("Görsel doğrulama için uygun durumda değil.")
	}

	now := w.clock.Now()
	if asset.LifecycleStatus == domainmedia.AssetUploaded {
		if asset, err = w.repo.SetAssetValidating(ctx, assetID, now); err != nil {
			return err
		}
	}

	raw, _, err := w.storage.GetObject(ctx, domainmedia.RawObjectKey(assetID))
	if err != nil {
		return err
	}

	hints := decodeAssetHints(asset.TechnicalMetadata)
	processed, err := w.processor.ValidateAndNormalize(ctx, raw, hints.DeclaredContentType)
	if err != nil {
		if !isPermanentProcessingFailure(err) {
			return err
		}
		// A broken or unsupported file is terminal for this asset, not a retry.
		_, failErr := w.repo.SetAssetValidationFailed(ctx, assetID, failureReason(err), now)
		return failErr
	}

	masterKey := domainmedia.MasterObjectKey(assetID)
	if err := w.storage.PutObject(ctx, masterKey, processed.ContentType, processed.Bytes); err != nil {
		return err
	}

	return withTx(ctx, w.repo, func(ctx context.Context, repo Repository) error {
		if _, err := repo.SetAssetMasterReady(
			ctx, assetID, masterKey, processed.ContentType,
			int64(len(processed.Bytes)), processed.Width, processed.Height, now,
		); err != nil {
			return err
		}
		for _, profile := range domainmedia.RequiredTransformProfiles() {
			if _, err := repo.UpsertPendingVariant(ctx, domainmedia.Variant{
				ID:                uuid.New(),
				AssetID:           assetID,
				TransformProfile:  profile,
				LifecycleStatus:   domainmedia.VariantPending,
				TechnicalMetadata: domainmedia.EmptyMetadata(),
				CreatedAt:         now,
				UpdatedAt:         now,
			}); err != nil {
				return err
			}
			if err := enqueueIgnoringDuplicate(ctx, repo, variantJob(assetID, profile, now)); err != nil {
				return err
			}
		}
		return nil
	})
}

// ProcessGenerateVariant implements MEDIA-WORKER-02. Variants are always derived
// from the canonical master, never from the raw upload, and each profile
// succeeds or fails on its own.
func (w *Worker) ProcessGenerateVariant(ctx context.Context, assetID uuid.UUID, profile string) error {
	if !domainmedia.IsKnownTransformProfile(profile) {
		return apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   "transformProfile",
			Message: "Bilinmeyen transform profili.",
		})
	}
	asset, err := w.repo.FindAssetByID(ctx, assetID)
	if err != nil {
		return err
	}
	// An outdated worker result must not overwrite a newer state.
	if asset.LifecycleStatus != domainmedia.AssetMasterReady {
		return apperr.InvalidState("Görsel varyantı için canonical master hazır değil.")
	}
	if asset.MasterObjectKey == nil {
		return apperr.Internal(fmt.Errorf("asset %s is MASTER_READY without a master object key", assetID))
	}

	existing, err := w.repo.ListVariantsByAsset(ctx, assetID)
	if err != nil {
		return err
	}
	found := false
	for _, v := range existing {
		if v.TransformProfile != profile {
			continue
		}
		found = true
		if v.LifecycleStatus == domainmedia.VariantReady {
			// The same master and profile are never processed twice.
			return nil
		}
	}

	now := w.clock.Now()
	if !found {
		if _, err := w.repo.UpsertPendingVariant(ctx, domainmedia.Variant{
			ID:                uuid.New(),
			AssetID:           assetID,
			TransformProfile:  profile,
			LifecycleStatus:   domainmedia.VariantPending,
			TechnicalMetadata: domainmedia.EmptyMetadata(),
			CreatedAt:         now,
			UpdatedAt:         now,
		}); err != nil {
			return err
		}
	}

	master, _, err := w.storage.GetObject(ctx, *asset.MasterObjectKey)
	if err != nil {
		return err
	}
	processed, err := w.processor.GenerateVariant(ctx, master, profile)
	if err != nil {
		if !isPermanentProcessingFailure(err) {
			// A transient compression outage leaves the variant retryable; an
			// uncompressed file is never published as a fallback.
			return err
		}
		_, failErr := w.repo.MarkVariantFailed(ctx, assetID, profile, failureReason(err), now)
		return failErr
	}

	variantKey := domainmedia.VariantObjectKey(assetID, profile)
	if err := w.storage.PutObject(ctx, variantKey, processed.ContentType, processed.Bytes); err != nil {
		return err
	}
	_, err = w.repo.MarkVariantReady(
		ctx, assetID, profile, variantKey, processed.ContentType,
		int64(len(processed.Bytes)), processed.Width, processed.Height, now,
	)
	return err
}

// isPermanentProcessingFailure separates a permanently invalid file from a
// temporarily unavailable dependency. Only the former is recorded as a terminal
// failure on the row.
func isPermanentProcessingFailure(err error) bool {
	ae, ok := apperr.As(err)
	if !ok {
		return false
	}
	switch ae.Kind {
	case apperr.KindValidation, apperr.KindBadRequest:
		return true
	}
	return false
}

// failureReason returns a non-empty reason, which both failure CHECK constraints
// require.
func failureReason(err error) string {
	if ae, ok := apperr.As(err); ok && ae.Message != "" {
		return ae.Message
	}
	return "Görsel işlenemedi."
}
