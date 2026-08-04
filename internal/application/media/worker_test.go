package media_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

// worker builds a worker sharing the fixture's store, fake storage and fake
// processor.
func (f *fixture) worker(t *testing.T) *appmedia.Worker {
	t.Helper()
	w, err := appmedia.NewMemoryWorker(f.store, f.clock, appmedia.WorkerConfig{
		Storage:   f.storage,
		Processor: appmedia.FakeProcessor{},
	})
	if err != nil {
		t.Fatalf("new memory worker: %v", err)
	}
	return w
}

// seedUploadedAsset seeds an UPLOADED asset whose raw object already exists in
// the fake store, which is the state a confirmed upload leaves behind.
func (f *fixture) seedUploadedAsset(t *testing.T, raw []byte) uuid.UUID {
	t.Helper()
	assetID := f.seedAsset(f.owner, domainmedia.AssetUploaded)
	if err := f.storage.PutObject(context.Background(), domainmedia.RawObjectKey(assetID), "image/png", raw); err != nil {
		t.Fatalf("seed raw object: %v", err)
	}
	return assetID
}

func TestWorkerValidateAndNormalizeReachesMasterReadyAndEnqueuesVariantJobs(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	w := f.worker(t)
	assetID := f.seedUploadedAsset(t, appmedia.FakeImageBytes())

	if err := w.ProcessValidateAndNormalize(ctx, assetID); err != nil {
		t.Fatalf("validate and normalize: %v", err)
	}

	asset, ok := f.store.Asset(assetID)
	if !ok {
		t.Fatal("asset must exist")
	}
	if asset.LifecycleStatus != domainmedia.AssetMasterReady {
		t.Fatalf("lifecycle=%s", asset.LifecycleStatus)
	}
	// Every field the MASTER_READY database CHECK demands must be present.
	if asset.MasterObjectKey == nil || asset.ContentType == nil ||
		asset.ByteSize == nil || asset.WidthPx == nil || asset.HeightPx == nil {
		t.Fatalf("master ready fields incomplete: %+v", asset)
	}
	if *asset.MasterObjectKey != domainmedia.MasterObjectKey(assetID) {
		t.Fatalf("masterObjectKey=%s", *asset.MasterObjectKey)
	}
	if *asset.ByteSize <= 0 || *asset.WidthPx <= 0 || *asset.HeightPx <= 0 {
		t.Fatalf("byteSize=%d width=%d height=%d", *asset.ByteSize, *asset.WidthPx, *asset.HeightPx)
	}
	if !f.storage.Has(domainmedia.MasterObjectKey(assetID)) {
		t.Fatal("canonical master object must be written")
	}
	// The raw upload survives this step; its retention is a separate decision.
	if !f.storage.Has(domainmedia.RawObjectKey(assetID)) {
		t.Fatal("raw object must not be removed here")
	}

	// A ready master implies pending, not ready, variants.
	variants := f.store.Variants(assetID)
	if len(variants) != len(domainmedia.RequiredTransformProfiles()) {
		t.Fatalf("want %d variants, got %d", len(domainmedia.RequiredTransformProfiles()), len(variants))
	}
	for _, v := range variants {
		if v.LifecycleStatus != domainmedia.VariantPending {
			t.Fatalf("variant %s lifecycle=%s", v.TransformProfile, v.LifecycleStatus)
		}
		if v.ObjectKey != nil {
			t.Fatalf("pending variant must have no object key: %v", v.ObjectKey)
		}
	}

	jobs := f.store.JobsByType(domainmedia.JobGenerateVariant)
	if len(jobs) != 3 {
		t.Fatalf("want 3 variant jobs, got %d", len(jobs))
	}
	wantKeys := map[string]bool{}
	for _, profile := range domainmedia.RequiredTransformProfiles() {
		wantKeys[domainmedia.VariantJobDedupKey(assetID, profile)] = true
	}
	for _, job := range jobs {
		if job.DeduplicationKey == nil || !wantKeys[*job.DeduplicationKey] {
			t.Fatalf("unexpected dedup key %v", job.DeduplicationKey)
		}
		delete(wantKeys, *job.DeduplicationKey)
	}
	if len(wantKeys) != 0 {
		t.Fatalf("missing variant jobs for %v", wantKeys)
	}

	// A redelivered job must not duplicate variants or jobs.
	if err := w.ProcessValidateAndNormalize(ctx, assetID); err != nil {
		t.Fatalf("second validate run: %v", err)
	}
	if jobs := f.store.JobsByType(domainmedia.JobGenerateVariant); len(jobs) != 3 {
		t.Fatalf("variant jobs duplicated: %d", len(jobs))
	}
	if variants := f.store.Variants(assetID); len(variants) != 3 {
		t.Fatalf("variants duplicated: %d", len(variants))
	}
}

func TestWorkerGenerateVariantMarksVariantReady(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	w := f.worker(t)
	assetID := f.seedUploadedAsset(t, appmedia.FakeImageBytes())
	if err := w.ProcessValidateAndNormalize(ctx, assetID); err != nil {
		t.Fatalf("validate and normalize: %v", err)
	}

	for _, profile := range domainmedia.RequiredTransformProfiles() {
		if err := w.ProcessGenerateVariant(ctx, assetID, profile); err != nil {
			t.Fatalf("generate %s: %v", profile, err)
		}
	}

	for _, v := range f.store.Variants(assetID) {
		if v.LifecycleStatus != domainmedia.VariantReady {
			t.Fatalf("variant %s lifecycle=%s", v.TransformProfile, v.LifecycleStatus)
		}
		// Every field the READY variant CHECK demands must be present.
		if v.ObjectKey == nil || v.ContentType == nil || v.ByteSize == nil ||
			v.WidthPx == nil || v.HeightPx == nil {
			t.Fatalf("ready variant fields incomplete: %+v", v)
		}
		if *v.ObjectKey != domainmedia.VariantObjectKey(assetID, v.TransformProfile) {
			t.Fatalf("objectKey=%s", *v.ObjectKey)
		}
		if !f.storage.Has(*v.ObjectKey) {
			t.Fatalf("variant object %s must be written", *v.ObjectKey)
		}
	}

	// Re-running a ready profile is a no-op, so no duplicate object is produced.
	if err := w.ProcessGenerateVariant(ctx, assetID, domainmedia.ProfileDetail); err != nil {
		t.Fatalf("repeat generate: %v", err)
	}
}

func TestWorkerGenerateVariantFailsOneProfileOnly(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	w := f.worker(t)
	assetID := f.seedUploadedAsset(t, appmedia.FakeImageBytes())
	if err := w.ProcessValidateAndNormalize(ctx, assetID); err != nil {
		t.Fatalf("validate and normalize: %v", err)
	}
	if err := w.ProcessGenerateVariant(ctx, assetID, domainmedia.ProfileDetail); err != nil {
		t.Fatalf("generate detail: %v", err)
	}

	// Corrupt only the master input the fake processor sees, then drive a single
	// profile: the already ready one must be untouched.
	if err := f.storage.PutObject(ctx, domainmedia.MasterObjectKey(assetID), "image/png", []byte("not-an-image")); err != nil {
		t.Fatalf("overwrite master: %v", err)
	}
	if err := w.ProcessGenerateVariant(ctx, assetID, domainmedia.ProfileSearch); err != nil {
		t.Fatalf("generate search: %v", err)
	}

	for _, v := range f.store.Variants(assetID) {
		switch v.TransformProfile {
		case domainmedia.ProfileDetail:
			if v.LifecycleStatus != domainmedia.VariantReady {
				t.Fatalf("DETAIL must stay READY, got %s", v.LifecycleStatus)
			}
		case domainmedia.ProfileSearch:
			if v.LifecycleStatus != domainmedia.VariantFailed {
				t.Fatalf("SEARCH must be FAILED, got %s", v.LifecycleStatus)
			}
			if v.FailureReason == nil || *v.FailureReason == "" {
				t.Fatal("FAILED variant needs a non-empty reason")
			}
		default:
			if v.LifecycleStatus != domainmedia.VariantPending {
				t.Fatalf("HOMEPAGE must stay PENDING, got %s", v.LifecycleStatus)
			}
		}
	}
}

func TestWorkerValidateRecordsPermanentFailureForUnreadableFile(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	w := f.worker(t)
	assetID := f.seedUploadedAsset(t, []byte("this is not an image"))

	if err := w.ProcessValidateAndNormalize(ctx, assetID); err != nil {
		t.Fatalf("validate and normalize: %v", err)
	}

	asset, _ := f.store.Asset(assetID)
	if asset.LifecycleStatus != domainmedia.AssetValidationFailed {
		t.Fatalf("lifecycle=%s", asset.LifecycleStatus)
	}
	if asset.FailureReason == nil || *asset.FailureReason == "" {
		t.Fatal("VALIDATION_FAILED needs a non-empty reason")
	}
	if f.storage.Has(domainmedia.MasterObjectKey(assetID)) {
		t.Fatal("no master object may be written for an invalid file")
	}
	if jobs := f.store.JobsByType(domainmedia.JobGenerateVariant); len(jobs) != 0 {
		t.Fatalf("no variant job may be enqueued, got %d", len(jobs))
	}
}

func TestWorkerValidateReportsUnconfiguredDependencies(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	assetID := f.seedUploadedAsset(t, appmedia.FakeImageBytes())

	t.Run("storage", func(t *testing.T) {
		w, err := appmedia.NewMemoryWorker(f.store, f.clock, appmedia.WorkerConfig{
			Storage:   appmedia.UnconfiguredStorage{},
			Processor: appmedia.FakeProcessor{},
		})
		if err != nil {
			t.Fatalf("new worker: %v", err)
		}
		requireCode(t, w.ProcessValidateAndNormalize(ctx, assetID), apperr.CodeDependencyUnavailable)
	})

	t.Run("processor", func(t *testing.T) {
		w, err := appmedia.NewMemoryWorker(f.store, f.clock, appmedia.WorkerConfig{
			Storage:   f.storage,
			Processor: appmedia.UnconfiguredImageProcessor{},
		})
		if err != nil {
			t.Fatalf("new worker: %v", err)
		}
		requireCode(t, w.ProcessValidateAndNormalize(ctx, assetID), apperr.CodeDependencyUnavailable)
		// A temporary outage must not be recorded as a permanent file failure.
		asset, _ := f.store.Asset(assetID)
		if asset.LifecycleStatus == domainmedia.AssetValidationFailed {
			t.Fatal("a transient dependency failure must stay retryable")
		}
	})
}

func TestWorkerGenerateVariantRequiresReadyMaster(t *testing.T) {
	f := newFixture(t)
	w := f.worker(t)
	assetID := f.seedAsset(f.owner, domainmedia.AssetUploaded)

	err := w.ProcessGenerateVariant(context.Background(), assetID, domainmedia.ProfileDetail)
	requireCode(t, err, apperr.CodeInvalidState)
}

func TestWorkerGenerateVariantRejectsUnknownProfile(t *testing.T) {
	f := newFixture(t)
	w := f.worker(t)
	assetID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)

	err := w.ProcessGenerateVariant(context.Background(), assetID, "THUMBNAIL")
	requireCode(t, err, apperr.CodeValidation)
}

func TestWorkerValidateRejectsUnknownAsset(t *testing.T) {
	f := newFixture(t)
	w := f.worker(t)
	err := w.ProcessValidateAndNormalize(context.Background(), uuid.New())
	requireCode(t, err, apperr.CodeNotFound)
}
