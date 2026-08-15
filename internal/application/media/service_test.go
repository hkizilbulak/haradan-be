package media_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

const (
	testUploadTTL   = 15 * time.Minute
	testMaxByteSize = int64(5 * 1024 * 1024)
)

func testAllowedContentTypes() []string { return []string{"image/jpeg", "image/png"} }

type fixture struct {
	svc      *appmedia.Service
	store    *appmedia.MemoryStore
	storage  *appmedia.FakeStorage
	clock    *testClock
	owner    uuid.UUID
	stranger uuid.UUID
}

// newFixture builds a fully configured service on the fake storage.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	clock := &testClock{now: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)}
	store := appmedia.NewMemoryStore()
	storage := appmedia.NewFakeStorage(clock)

	svc, err := appmedia.NewMemoryService(store, clock, appmedia.Config{
		Storage:             storage,
		Processor:           appmedia.FakeProcessor{},
		AllowedContentTypes: testAllowedContentTypes(),
		MaxByteSize:         testMaxByteSize,
		UploadURLTTL:        testUploadTTL,
	})
	if err != nil {
		t.Fatalf("new memory service: %v", err)
	}
	return &fixture{
		svc:      svc,
		store:    store,
		storage:  storage,
		clock:    clock,
		owner:    uuid.New(),
		stranger: uuid.New(),
	}
}

// newServiceWithConfig builds a service with an explicit config so the
// unconfigured paths can be exercised.
func newServiceWithConfig(t *testing.T, cfg appmedia.Config) (*appmedia.Service, *appmedia.MemoryStore) {
	t.Helper()
	clock := &testClock{now: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)}
	store := appmedia.NewMemoryStore()
	svc, err := appmedia.NewMemoryService(store, clock, cfg)
	if err != nil {
		t.Fatalf("new memory service: %v", err)
	}
	return svc, store
}

// seedAsset inserts an asset directly so relation rules can be tested without
// walking the whole upload flow. MASTER_READY assets carry every field the
// database CHECK for that lifecycle requires.
func (f *fixture) seedAsset(ownerID uuid.UUID, lifecycle domainmedia.AssetLifecycle) uuid.UUID {
	id := uuid.New()
	now := f.clock.Now()
	rawKey := domainmedia.RawObjectKey(id)
	asset := domainmedia.Asset{
		ID:                id,
		OwnerUserID:       ownerID,
		Provider:          domainmedia.ProviderB2,
		RawObjectKey:      &rawKey,
		LifecycleStatus:   lifecycle,
		TechnicalMetadata: domainmedia.EmptyMetadata(),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if lifecycle == domainmedia.AssetMasterReady {
		masterKey := domainmedia.MasterObjectKey(id)
		contentType := "image/png"
		size := int64(2048)
		edge := 100
		asset.MasterObjectKey = &masterKey
		asset.ContentType = &contentType
		asset.ByteSize = &size
		asset.WidthPx = &edge
		asset.HeightPx = &edge
	}
	f.store.PutAsset(asset)
	return id
}

func (f *fixture) seedAdvert(ownerID uuid.UUID, status string, mediaVersion int) uuid.UUID {
	id := uuid.New()
	f.store.PutAdvert(appmedia.MemoryAdvert{
		ID:           id,
		OwnerUserID:  ownerID,
		Status:       status,
		MediaVersion: mediaVersion,
	})
	return id
}

func requireCode(t *testing.T, err error, want apperr.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("want %s, got nil error", want)
	}
	ae, ok := apperr.As(err)
	if !ok {
		t.Fatalf("want typed %s, got %v", want, err)
	}
	if ae.Code != want {
		t.Fatalf("want %s, got %s (%v)", want, ae.Code, err)
	}
}

func TestInitiateMediaUploadSuccess(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	declared := "image/jpeg"
	size := int64(1024)

	view, err := f.svc.InitiateMediaUpload(ctx, f.owner, appmedia.InitiateInput{
		DeclaredContentType: &declared,
		DeclaredByteSize:    &size,
	})
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if view.AssetID == uuid.Nil {
		t.Fatal("assetId must be set")
	}
	if view.Upload.Method != "PUT" {
		t.Fatalf("method=%q", view.Upload.Method)
	}
	if view.Upload.URL == "" {
		t.Fatal("upload url must be set")
	}
	if want := f.clock.Now().Add(testUploadTTL); !view.Upload.ExpiresAt.Equal(want) {
		t.Fatalf("expiresAt=%v want %v", view.Upload.ExpiresAt, want)
	}
	if view.Constraints.MaxByteSize != testMaxByteSize {
		t.Fatalf("maxByteSize=%d", view.Constraints.MaxByteSize)
	}
	if len(view.Constraints.AllowedContentTypes) != 2 {
		t.Fatalf("allowedContentTypes=%v", view.Constraints.AllowedContentTypes)
	}

	asset, ok := f.store.Asset(view.AssetID)
	if !ok {
		t.Fatal("asset must be persisted")
	}
	if asset.LifecycleStatus != domainmedia.AssetUploadPending {
		t.Fatalf("lifecycle=%s", asset.LifecycleStatus)
	}
	if asset.OwnerUserID != f.owner {
		t.Fatal("owner must come from the caller identity")
	}
	// The object key is system generated from the asset id, never from a client
	// file name.
	if asset.RawObjectKey == nil || *asset.RawObjectKey != "assets/"+view.AssetID.String()+"/raw" {
		t.Fatalf("rawObjectKey=%v", asset.RawObjectKey)
	}
	// The client MIME hint is not canonical, so it must not land in content_type.
	if asset.ContentType != nil {
		t.Fatalf("declared content type must not be trusted: %v", asset.ContentType)
	}

	body := []byte("fake-jpeg-bytes")
	if err := f.svc.PutMediaAssetContent(ctx, f.owner, view.AssetID, "image/jpeg", body); err != nil {
		t.Fatalf("put content: %v", err)
	}
	info, err := f.storage.HeadObject(ctx, domainmedia.RawObjectKey(view.AssetID))
	if err != nil || !info.Exists || info.ByteSize != int64(len(body)) {
		t.Fatalf("stored object info=%+v err=%v", info, err)
	}
}

func TestInitiateMediaUploadDependencyUnavailableWhenConfigUnset(t *testing.T) {
	cases := []struct {
		name string
		cfg  appmedia.Config
	}{
		{
			name: "no allowed content types",
			cfg: appmedia.Config{
				Storage:      appmedia.NewFakeStorage(nil),
				MaxByteSize:  testMaxByteSize,
				UploadURLTTL: testUploadTTL,
			},
		},
		{
			name: "blank allowed content types",
			cfg: appmedia.Config{
				Storage:             appmedia.NewFakeStorage(nil),
				AllowedContentTypes: []string{"  ", ""},
				MaxByteSize:         testMaxByteSize,
				UploadURLTTL:        testUploadTTL,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, store := newServiceWithConfig(t, tc.cfg)
			_, err := svc.InitiateMediaUpload(context.Background(), uuid.New(), appmedia.InitiateInput{})
			requireCode(t, err, apperr.CodeDependencyUnavailable)
			if jobs := store.Jobs(); len(jobs) != 0 {
				t.Fatalf("no job must be enqueued, got %d", len(jobs))
			}
		})
	}

	t.Run("zero max byte size uses security default", func(t *testing.T) {
		svc, _ := newServiceWithConfig(t, appmedia.Config{
			Storage:             appmedia.NewFakeStorage(nil),
			AllowedContentTypes: testAllowedContentTypes(),
			UploadURLTTL:        testUploadTTL,
		})
		view, err := svc.InitiateMediaUpload(context.Background(), uuid.New(), appmedia.InitiateInput{})
		if err != nil {
			t.Fatalf("initiate: %v", err)
		}
		if view.Constraints.MaxByteSize != domainmedia.MaxUploadBytes {
			t.Fatalf("MaxByteSize=%d want %d", view.Constraints.MaxByteSize, domainmedia.MaxUploadBytes)
		}
	})
}

func TestInitiateMediaUploadDependencyUnavailableWithUnconfiguredStorage(t *testing.T) {
	svc, _ := newServiceWithConfig(t, appmedia.Config{
		Storage:             appmedia.UnconfiguredStorage{},
		AllowedContentTypes: testAllowedContentTypes(),
		MaxByteSize:         testMaxByteSize,
		UploadURLTTL:        testUploadTTL,
	})
	_, err := svc.InitiateMediaUpload(context.Background(), uuid.New(), appmedia.InitiateInput{})
	requireCode(t, err, apperr.CodeDependencyUnavailable)
}

func TestInitiateMediaUploadRejectsUnsupportedDeclaredType(t *testing.T) {
	f := newFixture(t)
	declared := "image/svg+xml"
	_, err := f.svc.InitiateMediaUpload(context.Background(), f.owner, appmedia.InitiateInput{
		DeclaredContentType: &declared,
	})
	requireCode(t, err, apperr.CodeValidation)
}

func TestConfirmMediaUploadEnqueuesJobAndIsIdempotent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	initiated, err := f.svc.InitiateMediaUpload(ctx, f.owner, appmedia.InitiateInput{})
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	// Stand in for the client's direct upload to the quarantine object.
	rawKey := domainmedia.RawObjectKey(initiated.AssetID)
	if err := f.storage.PutObject(ctx, rawKey, "image/png", appmedia.FakeImageBytes()); err != nil {
		t.Fatalf("seed raw object: %v", err)
	}

	view, err := f.svc.ConfirmMediaUpload(ctx, f.owner, initiated.AssetID)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if view.LifecycleStatus != domainmedia.AssetUploaded {
		t.Fatalf("lifecycle=%s", view.LifecycleStatus)
	}
	jobs := f.store.JobsByType(domainmedia.JobValidateAndNormalize)
	if len(jobs) != 1 {
		t.Fatalf("want 1 validate job, got %d", len(jobs))
	}
	if jobs[0].DeduplicationKey == nil ||
		*jobs[0].DeduplicationKey != domainmedia.ValidateJobDedupKey(initiated.AssetID) {
		t.Fatalf("dedupKey=%v", jobs[0].DeduplicationKey)
	}
	if jobs[0].Status != domainmedia.JobQueued || jobs[0].MaxAttempts < 1 {
		t.Fatalf("job=%+v", jobs[0])
	}

	// A repeated completion finds the existing state and must not duplicate work.
	second, err := f.svc.ConfirmMediaUpload(ctx, f.owner, initiated.AssetID)
	if err != nil {
		t.Fatalf("second confirm: %v", err)
	}
	if second.LifecycleStatus != domainmedia.AssetUploaded {
		t.Fatalf("second lifecycle=%s", second.LifecycleStatus)
	}
	if jobs := f.store.JobsByType(domainmedia.JobValidateAndNormalize); len(jobs) != 1 {
		t.Fatalf("double confirm must not enqueue a second job, got %d", len(jobs))
	}
}

func TestConfirmMediaUploadWithoutObjectIsInvalidState(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	initiated, err := f.svc.InitiateMediaUpload(ctx, f.owner, appmedia.InitiateInput{})
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	_, err = f.svc.ConfirmMediaUpload(ctx, f.owner, initiated.AssetID)
	requireCode(t, err, apperr.CodeInvalidState)
	if jobs := f.store.Jobs(); len(jobs) != 0 {
		t.Fatalf("no job must be enqueued, got %d", len(jobs))
	}
}

func TestConfirmMediaUploadRejectsOversizedStoredObject(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	initiated, err := f.svc.InitiateMediaUpload(ctx, f.owner, appmedia.InitiateInput{})
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	oversized := make([]byte, testMaxByteSize+1)
	copy(oversized, appmedia.FakeImageBytes())
	if err := f.storage.PutObject(ctx, domainmedia.RawObjectKey(initiated.AssetID), "image/png", oversized); err != nil {
		t.Fatalf("seed oversized object: %v", err)
	}

	_, err = f.svc.ConfirmMediaUpload(ctx, f.owner, initiated.AssetID)
	requireCode(t, err, apperr.CodeValidation)
	if jobs := f.store.Jobs(); len(jobs) != 0 {
		t.Fatalf("oversized confirm must not enqueue a job, got %d", len(jobs))
	}
	asset, ok := f.store.Asset(initiated.AssetID)
	if !ok {
		t.Fatal("asset missing after rejected confirm")
	}
	if asset.LifecycleStatus != domainmedia.AssetUploadPending {
		t.Fatalf("lifecycle=%s, want UPLOAD_PENDING", asset.LifecycleStatus)
	}
}

func TestConfirmMediaUploadRejectsDisallowedStoredContentType(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	initiated, err := f.svc.InitiateMediaUpload(ctx, f.owner, appmedia.InitiateInput{})
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if err := f.storage.PutObject(ctx, domainmedia.RawObjectKey(initiated.AssetID), "application/pdf", appmedia.FakeImageBytes()); err != nil {
		t.Fatalf("seed disallowed object: %v", err)
	}

	_, err = f.svc.ConfirmMediaUpload(ctx, f.owner, initiated.AssetID)
	requireCode(t, err, apperr.CodeValidation)
	if jobs := f.store.Jobs(); len(jobs) != 0 {
		t.Fatalf("disallowed content type must not enqueue a job, got %d", len(jobs))
	}
}

func TestConfirmMediaUploadWrongOwnerNotFound(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	initiated, err := f.svc.InitiateMediaUpload(ctx, f.owner, appmedia.InitiateInput{})
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if err := f.storage.PutObject(ctx, domainmedia.RawObjectKey(initiated.AssetID), "image/png", appmedia.FakeImageBytes()); err != nil {
		t.Fatalf("seed raw object: %v", err)
	}

	_, err = f.svc.ConfirmMediaUpload(ctx, f.stranger, initiated.AssetID)
	requireCode(t, err, apperr.CodeNotFound)
}

func TestConfirmMediaUploadUnconfiguredStorageDependencyUnavailable(t *testing.T) {
	svc, store := newServiceWithConfig(t, appmedia.Config{
		Storage:             appmedia.UnconfiguredStorage{},
		AllowedContentTypes: testAllowedContentTypes(),
		MaxByteSize:         testMaxByteSize,
		UploadURLTTL:        testUploadTTL,
	})
	owner := uuid.New()
	assetID := uuid.New()
	rawKey := domainmedia.RawObjectKey(assetID)
	store.PutAsset(domainmedia.Asset{
		ID:                assetID,
		OwnerUserID:       owner,
		Provider:          domainmedia.ProviderB2,
		RawObjectKey:      &rawKey,
		LifecycleStatus:   domainmedia.AssetUploadPending,
		TechnicalMetadata: domainmedia.EmptyMetadata(),
	})

	_, err := svc.ConfirmMediaUpload(context.Background(), owner, assetID)
	requireCode(t, err, apperr.CodeDependencyUnavailable)
}

func TestAdvertMediaAttachReorderCoverDetachLifecycle(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	advertID := f.seedAdvert(f.owner, "DRAFT", 1)
	assetA := f.seedAsset(f.owner, domainmedia.AssetMasterReady)
	assetB := f.seedAsset(f.owner, domainmedia.AssetMasterReady)
	assetC := f.seedAsset(f.owner, domainmedia.AssetUploaded)

	// The first attached image with a ready master becomes the cover.
	view, err := f.svc.AttachMediaToAdvert(ctx, f.owner, advertID, assetA, nil, 1)
	if err != nil {
		t.Fatalf("attach A: %v", err)
	}
	if view.MediaVersion != 2 {
		t.Fatalf("mediaVersion=%d", view.MediaVersion)
	}
	if len(view.Items) != 1 || view.Items[0].AssetID != assetA || !view.Items[0].IsCover {
		t.Fatalf("items=%+v", view.Items)
	}
	if view.Items[0].DisplayOrder != 0 {
		t.Fatalf("displayOrder=%d", view.Items[0].DisplayOrder)
	}

	if view, err = f.svc.AttachMediaToAdvert(ctx, f.owner, advertID, assetB, nil, 2); err != nil {
		t.Fatalf("attach B: %v", err)
	}
	if view.MediaVersion != 3 {
		t.Fatalf("mediaVersion=%d", view.MediaVersion)
	}
	if view, err = f.svc.AttachMediaToAdvert(ctx, f.owner, advertID, assetC, nil, 3); err != nil {
		t.Fatalf("attach C: %v", err)
	}
	if len(view.Items) != 3 {
		t.Fatalf("items=%+v", view.Items)
	}
	coverCount := 0
	for _, item := range view.Items {
		if item.IsCover {
			coverCount++
		}
	}
	if coverCount != 1 {
		t.Fatalf("exactly one cover expected, got %d", coverCount)
	}

	// A duplicate attach is a conflict, not a silent no-op.
	_, err = f.svc.AttachMediaToAdvert(ctx, f.owner, advertID, assetA, nil, 4)
	requireCode(t, err, apperr.CodeConflict)

	reordered, err := f.svc.ReorderAdvertMedia(ctx, f.owner, advertID, []uuid.UUID{assetC, assetA, assetB}, 4)
	if err != nil {
		t.Fatalf("reorder: %v", err)
	}
	if reordered.MediaVersion != 5 {
		t.Fatalf("mediaVersion=%d", reordered.MediaVersion)
	}
	wantOrder := []uuid.UUID{assetC, assetA, assetB}
	for i, item := range reordered.Items {
		if item.AssetID != wantOrder[i] || item.DisplayOrder != i {
			t.Fatalf("items=%+v", reordered.Items)
		}
	}

	covered, err := f.svc.SetAdvertCover(ctx, f.owner, advertID, assetC, 5)
	if err != nil {
		t.Fatalf("set cover: %v", err)
	}
	if covered.MediaVersion != 6 {
		t.Fatalf("mediaVersion=%d", covered.MediaVersion)
	}
	for _, item := range covered.Items {
		if item.IsCover != (item.AssetID == assetC) {
			t.Fatalf("cover must be exactly asset C: %+v", covered.Items)
		}
	}

	// Setting the same cover again is idempotent and does not bump the version.
	same, err := f.svc.SetAdvertCover(ctx, f.owner, advertID, assetC, 6)
	if err != nil {
		t.Fatalf("repeat set cover: %v", err)
	}
	if same.MediaVersion != 6 {
		t.Fatalf("repeat set cover must not bump: %d", same.MediaVersion)
	}

	// Detaching the cover promotes the first remaining relation in display order
	// whose asset has a ready master, which skips the UPLOADED asset C position.
	detached, err := f.svc.DetachMediaFromAdvert(ctx, f.owner, advertID, assetC, 6)
	if err != nil {
		t.Fatalf("detach cover: %v", err)
	}
	if detached.MediaVersion != 7 {
		t.Fatalf("mediaVersion=%d", detached.MediaVersion)
	}
	if len(detached.Items) != 2 {
		t.Fatalf("items=%+v", detached.Items)
	}
	for _, item := range detached.Items {
		if item.IsCover != (item.AssetID == assetA) {
			t.Fatalf("cover must be promoted to asset A: %+v", detached.Items)
		}
	}

	// Detaching again is idempotent and leaves the version alone.
	again, err := f.svc.DetachMediaFromAdvert(ctx, f.owner, advertID, assetC, 7)
	if err != nil {
		t.Fatalf("repeat detach: %v", err)
	}
	if again.MediaVersion != 7 || len(again.Items) != 2 {
		t.Fatalf("repeat detach changed state: %+v", again)
	}
}

func TestAdvertMediaStaleMediaVersion(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	advertID := f.seedAdvert(f.owner, "DRAFT", 3)
	assetID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)
	otherID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)
	if _, err := f.svc.AttachMediaToAdvert(ctx, f.owner, advertID, assetID, nil, 3); err != nil {
		t.Fatalf("attach: %v", err)
	}

	t.Run("attach", func(t *testing.T) {
		_, err := f.svc.AttachMediaToAdvert(ctx, f.owner, advertID, otherID, nil, 3)
		requireCode(t, err, apperr.CodeStaleVersion)
	})
	t.Run("reorder", func(t *testing.T) {
		_, err := f.svc.ReorderAdvertMedia(ctx, f.owner, advertID, []uuid.UUID{assetID}, 3)
		requireCode(t, err, apperr.CodeStaleVersion)
	})
	t.Run("cover", func(t *testing.T) {
		_, err := f.svc.SetAdvertCover(ctx, f.owner, advertID, assetID, 3)
		requireCode(t, err, apperr.CodeStaleVersion)
	})
	t.Run("detach", func(t *testing.T) {
		_, err := f.svc.DetachMediaFromAdvert(ctx, f.owner, advertID, assetID, 3)
		requireCode(t, err, apperr.CodeStaleVersion)
	})
}

func TestAttachCrossUserAssetOrAdvertNotFound(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	t.Run("foreign asset", func(t *testing.T) {
		advertID := f.seedAdvert(f.owner, "DRAFT", 1)
		foreignAsset := f.seedAsset(f.stranger, domainmedia.AssetMasterReady)
		_, err := f.svc.AttachMediaToAdvert(ctx, f.owner, advertID, foreignAsset, nil, 1)
		requireCode(t, err, apperr.CodeNotFound)
	})

	t.Run("foreign advert", func(t *testing.T) {
		foreignAdvert := f.seedAdvert(f.stranger, "DRAFT", 1)
		assetID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)
		_, err := f.svc.AttachMediaToAdvert(ctx, f.owner, foreignAdvert, assetID, nil, 1)
		requireCode(t, err, apperr.CodeNotFound)
	})

	t.Run("foreign advert and asset", func(t *testing.T) {
		foreignAdvert := f.seedAdvert(f.stranger, "DRAFT", 1)
		foreignAsset := f.seedAsset(f.stranger, domainmedia.AssetMasterReady)
		_, err := f.svc.AttachMediaToAdvert(ctx, f.owner, foreignAdvert, foreignAsset, nil, 1)
		requireCode(t, err, apperr.CodeNotFound)
	})
}

func TestMediaMutationsRejectedOutsideEditableStatuses(t *testing.T) {
	closedStatuses := []string{"PENDING_REVIEW", "PUBLISHED", "REJECTED", "SUSPENDED", "SOLD", "ARCHIVED"}
	for _, status := range closedStatuses {
		t.Run(status, func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()
			advertID := f.seedAdvert(f.owner, status, 1)
			assetID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)

			_, err := f.svc.AttachMediaToAdvert(ctx, f.owner, advertID, assetID, nil, 1)
			requireCode(t, err, apperr.CodeInvalidState)

			_, err = f.svc.ReorderAdvertMedia(ctx, f.owner, advertID, []uuid.UUID{assetID}, 1)
			requireCode(t, err, apperr.CodeInvalidState)

			_, err = f.svc.SetAdvertCover(ctx, f.owner, advertID, assetID, 1)
			requireCode(t, err, apperr.CodeInvalidState)

			_, err = f.svc.DetachMediaFromAdvert(ctx, f.owner, advertID, assetID, 1)
			requireCode(t, err, apperr.CodeInvalidState)
		})
	}
}

func TestAttachToChangesRequestedAdvertAllowed(t *testing.T) {
	f := newFixture(t)
	advertID := f.seedAdvert(f.owner, "CHANGES_REQUESTED", 1)
	assetID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)

	view, err := f.svc.AttachMediaToAdvert(context.Background(), f.owner, advertID, assetID, nil, 1)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if view.MediaVersion != 2 || len(view.Items) != 1 {
		t.Fatalf("view=%+v", view)
	}
}

func TestAttachSoftDeletedAdvertInvalidState(t *testing.T) {
	f := newFixture(t)
	deletedAt := f.clock.Now()
	advertID := uuid.New()
	f.store.PutAdvert(appmedia.MemoryAdvert{
		ID:           advertID,
		OwnerUserID:  f.owner,
		Status:       "DRAFT",
		MediaVersion: 1,
		DeletedAt:    &deletedAt,
	})
	assetID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)

	_, err := f.svc.AttachMediaToAdvert(context.Background(), f.owner, advertID, assetID, nil, 1)
	requireCode(t, err, apperr.CodeInvalidState)
}

func TestAttachRejectsNonAttachableAssetLifecycles(t *testing.T) {
	rejected := []domainmedia.AssetLifecycle{
		domainmedia.AssetValidationFailed,
		domainmedia.AssetCleanupCandidate,
		domainmedia.AssetDeleting,
		domainmedia.AssetPhysicallyDeleted,
	}
	for _, lifecycle := range rejected {
		t.Run(string(lifecycle), func(t *testing.T) {
			f := newFixture(t)
			advertID := f.seedAdvert(f.owner, "DRAFT", 1)
			assetID := f.seedAsset(f.owner, lifecycle)
			_, err := f.svc.AttachMediaToAdvert(context.Background(), f.owner, advertID, assetID, nil, 1)
			requireCode(t, err, apperr.CodeInvalidState)
		})
	}
}

func TestAttachAcceptsProcessingAssetLifecycles(t *testing.T) {
	for _, lifecycle := range domainmedia.AttachableAssetLifecycles() {
		t.Run(string(lifecycle), func(t *testing.T) {
			f := newFixture(t)
			advertID := f.seedAdvert(f.owner, "DRAFT", 1)
			assetID := f.seedAsset(f.owner, lifecycle)
			view, err := f.svc.AttachMediaToAdvert(context.Background(), f.owner, advertID, assetID, nil, 1)
			if err != nil {
				t.Fatalf("attach %s: %v", lifecycle, err)
			}
			// Only a ready master may become the automatic cover.
			if view.Items[0].IsCover != (lifecycle == domainmedia.AssetMasterReady) {
				t.Fatalf("isCover=%v for %s", view.Items[0].IsCover, lifecycle)
			}
		})
	}
}

func TestReorderRequiresExactAttachedSet(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	advertID := f.seedAdvert(f.owner, "DRAFT", 1)
	assetA := f.seedAsset(f.owner, domainmedia.AssetMasterReady)
	assetB := f.seedAsset(f.owner, domainmedia.AssetUploaded)
	detached := f.seedAsset(f.owner, domainmedia.AssetUploaded)
	if _, err := f.svc.AttachMediaToAdvert(ctx, f.owner, advertID, assetA, nil, 1); err != nil {
		t.Fatalf("attach A: %v", err)
	}
	if _, err := f.svc.AttachMediaToAdvert(ctx, f.owner, advertID, assetB, nil, 2); err != nil {
		t.Fatalf("attach B: %v", err)
	}

	cases := map[string][]uuid.UUID{
		"missing one":     {assetA},
		"unknown asset":   {assetA, assetB, detached},
		"duplicate asset": {assetA, assetA},
		"empty":           {},
	}
	for name, ids := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := f.svc.ReorderAdvertMedia(ctx, f.owner, advertID, ids, 3)
			requireCode(t, err, apperr.CodeValidation)
		})
	}
}

func TestSetCoverRequiresAttachedAsset(t *testing.T) {
	f := newFixture(t)
	advertID := f.seedAdvert(f.owner, "DRAFT", 1)
	assetID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)

	_, err := f.svc.SetAdvertCover(context.Background(), f.owner, advertID, assetID, 1)
	requireCode(t, err, apperr.CodeInvalidState)
}

func TestGetMediaProcessingStatusOwnerScoped(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	assetID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)
	f.store.PutVariant(readyVariant(assetID, domainmedia.ProfileDetail))

	view, err := f.svc.GetMediaProcessingStatus(ctx, f.owner, assetID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if view.LifecycleStatus != domainmedia.AssetMasterReady || len(view.Variants) != 1 {
		t.Fatalf("view=%+v", view)
	}
	// A ready variant projects the same-origin delivery path (never an object key).
	if view.Variants[0].PublicURL == nil {
		t.Fatal("publicUrl must be set for READY variants")
	}
	want := domainmedia.PublicDeliveryURL(assetID, domainmedia.ProfileDetail)
	if *view.Variants[0].PublicURL != want {
		t.Fatalf("publicUrl=%q want %q", *view.Variants[0].PublicURL, want)
	}

	_, err = f.svc.GetMediaProcessingStatus(ctx, f.stranger, assetID)
	requireCode(t, err, apperr.CodeNotFound)
}

// TestViewsNeverMarshalObjectKeys is the guard for the rule that storage layout
// never reaches a client: no view may serialize a raw, master or variant key.
func TestViewsNeverMarshalObjectKeys(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	advertID := f.seedAdvert(f.owner, "DRAFT", 1)
	assetID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)
	for _, profile := range domainmedia.RequiredTransformProfiles() {
		f.store.PutVariant(readyVariant(assetID, profile))
	}
	ownerView, err := f.svc.AttachMediaToAdvert(ctx, f.owner, advertID, assetID, nil, 1)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	processingView, err := f.svc.GetMediaProcessingStatus(ctx, f.owner, assetID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	forbidden := []string{
		"assets/" + assetID.String(),
		"/raw",
		"/master",
		"/variants/",
		"objectKey",
		"rawObjectKey",
		"masterObjectKey",
	}
	for name, view := range map[string]any{"ownerView": ownerView, "processingView": processingView} {
		raw, err := json.Marshal(view)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		encoded := string(raw)
		for _, needle := range forbidden {
			if strings.Contains(encoded, needle) {
				t.Fatalf("%s leaks %q: %s", name, needle, encoded)
			}
		}
	}
	for _, v := range processingView.Variants {
		if v.LifecycleStatus != domainmedia.VariantReady {
			continue
		}
		if v.PublicURL == nil || *v.PublicURL == "" {
			t.Fatal("READY variant must expose publicUrl")
		}
		if strings.Contains(*v.PublicURL, "assets/") || strings.Contains(*v.PublicURL, "/raw") {
			t.Fatalf("publicUrl leaked object key shape: %s", *v.PublicURL)
		}
	}
}

// TestInitiateViewNeverMarshalsObjectKey covers the upload grant separately: the
// object key is needed server-side but must not be projected.
func TestInitiateViewNeverMarshalsObjectKey(t *testing.T) {
	f := newFixture(t)
	view, err := f.svc.InitiateMediaUpload(context.Background(), f.owner, appmedia.InitiateInput{})
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "objectKey") {
		t.Fatalf("initiate view leaks the object key field: %s", raw)
	}
}

func readyVariant(assetID uuid.UUID, profile string) domainmedia.Variant {
	objectKey := domainmedia.VariantObjectKey(assetID, profile)
	contentType := "image/png"
	size := int64(512)
	edge := 50
	return domainmedia.Variant{
		ID:                uuid.New(),
		AssetID:           assetID,
		TransformProfile:  profile,
		ObjectKey:         &objectKey,
		LifecycleStatus:   domainmedia.VariantReady,
		WidthPx:           &edge,
		HeightPx:          &edge,
		ByteSize:          &size,
		ContentType:       &contentType,
		TechnicalMetadata: domainmedia.EmptyMetadata(),
	}
}
