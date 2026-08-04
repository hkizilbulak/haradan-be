package media

// This file holds in-memory test doubles: a store-backed Repository plus a fake
// storage and a fake image processor. They exist so the media use cases can be
// unit tested without a database, an object store or a compression provider.
//
// None of them may be wired into a running process: production builds use
// NewPostgresService together with a real Storage/ImageProcessor adapter, and
// until such an adapter exists the unconfigured implementations in ports.go keep
// the failure honest.

import (
	"bytes"
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

const memoryStateChangedMessage = "Görsel durumu değişti; tekrar deneyin."

// MemoryAdvert is the advert state the media store needs to answer ownership,
// status and media version questions.
type MemoryAdvert struct {
	ID           uuid.UUID
	OwnerUserID  uuid.UUID
	Status       string
	MediaVersion int
	DeletedAt    *time.Time
}

// MemoryStore holds in-memory media state for tests.
type MemoryStore struct {
	mu   sync.Mutex
	txMu sync.Mutex // serializes BeginTx..Commit like a coarse row lock

	assets    map[uuid.UUID]domainmedia.Asset
	variants  map[uuid.UUID]map[string]domainmedia.Variant
	relations map[uuid.UUID][]domainmedia.AdvertMediaRelation
	adverts   map[uuid.UUID]MemoryAdvert
	jobs      []domainmedia.BackgroundJob
	dedup     map[string]struct{}
}

// NewMemoryStore builds an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		assets:    map[uuid.UUID]domainmedia.Asset{},
		variants:  map[uuid.UUID]map[string]domainmedia.Variant{},
		relations: map[uuid.UUID][]domainmedia.AdvertMediaRelation{},
		adverts:   map[uuid.UUID]MemoryAdvert{},
		dedup:     map[string]struct{}{},
	}
}

// PutAsset seeds or replaces an asset.
func (s *MemoryStore) PutAsset(a domainmedia.Asset) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.assets[a.ID] = a
}

// Asset returns a seeded asset.
func (s *MemoryStore) Asset(id uuid.UUID) (domainmedia.Asset, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.assets[id]
	return a, ok
}

// PutAdvert seeds or replaces the advert state media rules depend on.
func (s *MemoryStore) PutAdvert(a MemoryAdvert) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adverts[a.ID] = a
}

// Advert returns a seeded advert.
func (s *MemoryStore) Advert(id uuid.UUID) (MemoryAdvert, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.adverts[id]
	return a, ok
}

// PutVariant seeds or replaces a variant.
func (s *MemoryStore) PutVariant(v domainmedia.Variant) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.putVariantLocked(v)
}

func (s *MemoryStore) putVariantLocked(v domainmedia.Variant) {
	byProfile, ok := s.variants[v.AssetID]
	if !ok {
		byProfile = map[string]domainmedia.Variant{}
		s.variants[v.AssetID] = byProfile
	}
	byProfile[v.TransformProfile] = v
}

// Variants returns an asset's variants ordered by transform profile.
func (s *MemoryStore) Variants(assetID uuid.UUID) []domainmedia.Variant {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.variantsLocked(assetID)
}

func (s *MemoryStore) variantsLocked(assetID uuid.UUID) []domainmedia.Variant {
	out := make([]domainmedia.Variant, 0, len(s.variants[assetID]))
	for _, v := range s.variants[assetID] {
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TransformProfile < out[j].TransformProfile })
	return out
}

// Relations returns an advert's media relations ordered by display order.
func (s *MemoryStore) Relations(advertID uuid.UUID) []domainmedia.AdvertMediaRelation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.relationsLocked(advertID)
}

func (s *MemoryStore) relationsLocked(advertID uuid.UUID) []domainmedia.AdvertMediaRelation {
	out := append([]domainmedia.AdvertMediaRelation(nil), s.relations[advertID]...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].DisplayOrder < out[j].DisplayOrder })
	return out
}

// Jobs returns the enqueued jobs in insertion order.
func (s *MemoryStore) Jobs() []domainmedia.BackgroundJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domainmedia.BackgroundJob(nil), s.jobs...)
}

// JobsByType returns the enqueued jobs of one type in insertion order.
func (s *MemoryStore) JobsByType(jobType domainmedia.JobType) []domainmedia.BackgroundJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domainmedia.BackgroundJob
	for _, job := range s.jobs {
		if job.JobType == jobType {
			out = append(out, job)
		}
	}
	return out
}

// Repo returns the Repository view of the store.
func (s *MemoryStore) Repo() Repository { return MemoryRepository{store: s} }

// NewMemoryService builds a media service backed by the store. Storage and
// Processor stay as supplied, so a test can pass a fake or deliberately leave
// them unconfigured.
func NewMemoryService(store *MemoryStore, clock Clock, cfg Config) (*Service, error) {
	cfg.Repo = store.Repo()
	if cfg.Clock == nil {
		cfg.Clock = clock
	}
	return NewService(cfg)
}

// NewMemoryWorker builds a media worker backed by the store.
func NewMemoryWorker(store *MemoryStore, clock Clock, cfg WorkerConfig) (*Worker, error) {
	cfg.Repo = store.Repo()
	if cfg.Clock == nil {
		cfg.Clock = clock
	}
	return NewWorker(cfg)
}

// MemoryRepository implements Repository against a MemoryStore.
type MemoryRepository struct {
	store *MemoryStore
}

// BeginTx starts a fake transaction that serializes concurrent callers.
func (r MemoryRepository) BeginTx(context.Context) (pgx.Tx, error) {
	r.store.txMu.Lock()
	return &memoryTx{store: r.store}, nil
}

// WithTx returns the same store-backed repository.
func (r MemoryRepository) WithTx(pgx.Tx) Repository { return r }

// CreateAsset inserts a new asset.
func (r MemoryRepository) CreateAsset(_ context.Context, a domainmedia.Asset) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if _, ok := r.store.assets[a.ID]; ok {
		return apperr.Conflict("asset already exists")
	}
	r.store.assets[a.ID] = a
	return nil
}

// FindAssetByIDForOwner returns an owner-scoped asset.
func (r MemoryRepository) FindAssetByIDForOwner(_ context.Context, ownerID, assetID uuid.UUID) (domainmedia.Asset, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	a, ok := r.store.assets[assetID]
	if !ok || a.OwnerUserID != ownerID {
		return domainmedia.Asset{}, apperr.NotFound(assetNotFoundMessage)
	}
	return a, nil
}

// FindAssetByIDForOwnerForUpdate behaves like FindAssetByIDForOwner; the fake
// transaction already serializes writers.
func (r MemoryRepository) FindAssetByIDForOwnerForUpdate(ctx context.Context, ownerID, assetID uuid.UUID) (domainmedia.Asset, error) {
	return r.FindAssetByIDForOwner(ctx, ownerID, assetID)
}

// FindAssetByID returns an asset without owner scoping, for worker steps.
func (r MemoryRepository) FindAssetByID(_ context.Context, assetID uuid.UUID) (domainmedia.Asset, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	a, ok := r.store.assets[assetID]
	if !ok {
		return domainmedia.Asset{}, apperr.NotFound(assetNotFoundMessage)
	}
	return a, nil
}

// FindAssetByIDForUpdate behaves like FindAssetByID.
func (r MemoryRepository) FindAssetByIDForUpdate(ctx context.Context, assetID uuid.UUID) (domainmedia.Asset, error) {
	return r.FindAssetByID(ctx, assetID)
}

// UpdateAssetLifecycle moves an asset between two lifecycles when it still holds
// the expected one.
func (r MemoryRepository) UpdateAssetLifecycle(
	_ context.Context,
	assetID uuid.UUID,
	from, to domainmedia.AssetLifecycle,
	now time.Time,
) (domainmedia.Asset, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	a, err := r.assetLocked(assetID)
	if err != nil {
		return domainmedia.Asset{}, err
	}
	if a.LifecycleStatus != from {
		return domainmedia.Asset{}, apperr.InvalidState(memoryStateChangedMessage)
	}
	a.LifecycleStatus = to
	a.UpdatedAt = now
	r.store.assets[assetID] = a
	return a, nil
}

// SetAssetUploaded moves UPLOAD_PENDING to UPLOADED and stores the raw key.
func (r MemoryRepository) SetAssetUploaded(
	_ context.Context,
	assetID uuid.UUID,
	rawObjectKey string,
	now time.Time,
) (domainmedia.Asset, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	a, err := r.assetLocked(assetID)
	if err != nil {
		return domainmedia.Asset{}, err
	}
	if a.LifecycleStatus != domainmedia.AssetUploadPending {
		return domainmedia.Asset{}, apperr.InvalidState(memoryStateChangedMessage)
	}
	key := rawObjectKey
	a.RawObjectKey = &key
	a.LifecycleStatus = domainmedia.AssetUploaded
	a.UpdatedAt = now
	r.store.assets[assetID] = a
	return a, nil
}

// SetAssetValidating moves UPLOADED to VALIDATING.
func (r MemoryRepository) SetAssetValidating(ctx context.Context, assetID uuid.UUID, now time.Time) (domainmedia.Asset, error) {
	return r.UpdateAssetLifecycle(ctx, assetID, domainmedia.AssetUploaded, domainmedia.AssetValidating, now)
}

// SetAssetMasterReady writes the master key and every field MASTER_READY needs.
func (r MemoryRepository) SetAssetMasterReady(
	_ context.Context,
	assetID uuid.UUID,
	masterObjectKey string,
	contentType string,
	byteSize int64,
	width, height int,
	now time.Time,
) (domainmedia.Asset, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	a, err := r.assetLocked(assetID)
	if err != nil {
		return domainmedia.Asset{}, err
	}
	if a.LifecycleStatus != domainmedia.AssetUploaded && a.LifecycleStatus != domainmedia.AssetValidating {
		return domainmedia.Asset{}, apperr.InvalidState(memoryStateChangedMessage)
	}
	key := masterObjectKey
	ct := contentType
	size := byteSize
	w := width
	h := height
	a.MasterObjectKey = &key
	a.ContentType = &ct
	a.ByteSize = &size
	a.WidthPx = &w
	a.HeightPx = &h
	a.LifecycleStatus = domainmedia.AssetMasterReady
	a.FailureReason = nil
	a.UpdatedAt = now
	r.store.assets[assetID] = a
	return a, nil
}

// SetAssetValidationFailed records a terminal validation failure.
func (r MemoryRepository) SetAssetValidationFailed(
	_ context.Context,
	assetID uuid.UUID,
	reason string,
	now time.Time,
) (domainmedia.Asset, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	a, err := r.assetLocked(assetID)
	if err != nil {
		return domainmedia.Asset{}, err
	}
	failure := reason
	a.FailureReason = &failure
	a.LifecycleStatus = domainmedia.AssetValidationFailed
	a.UpdatedAt = now
	r.store.assets[assetID] = a
	return a, nil
}

// UpsertPendingVariant inserts a PENDING variant row, keeping an existing one so
// the same master and profile are never duplicated.
func (r MemoryRepository) UpsertPendingVariant(_ context.Context, v domainmedia.Variant) (domainmedia.Variant, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if existing, ok := r.store.variants[v.AssetID][v.TransformProfile]; ok {
		return existing, nil
	}
	r.store.putVariantLocked(v)
	return v, nil
}

// ListVariantsByAsset returns the variants of one asset ordered by profile.
func (r MemoryRepository) ListVariantsByAsset(_ context.Context, assetID uuid.UUID) ([]domainmedia.Variant, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	return r.store.variantsLocked(assetID), nil
}

// MarkVariantReady writes every field the READY variant CHECK needs.
func (r MemoryRepository) MarkVariantReady(
	_ context.Context,
	assetID uuid.UUID,
	profile string,
	objectKey string,
	contentType string,
	byteSize int64,
	width, height int,
	now time.Time,
) (domainmedia.Variant, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	v, ok := r.store.variants[assetID][profile]
	if !ok {
		return domainmedia.Variant{}, apperr.NotFound("Görsel varyantı bulunamadı.")
	}
	key := objectKey
	ct := contentType
	size := byteSize
	w := width
	h := height
	v.ObjectKey = &key
	v.ContentType = &ct
	v.ByteSize = &size
	v.WidthPx = &w
	v.HeightPx = &h
	v.LifecycleStatus = domainmedia.VariantReady
	v.FailureReason = nil
	v.UpdatedAt = now
	r.store.putVariantLocked(v)
	return v, nil
}

// MarkVariantFailed records a per-profile failure without touching the others.
func (r MemoryRepository) MarkVariantFailed(
	_ context.Context,
	assetID uuid.UUID,
	profile string,
	reason string,
	now time.Time,
) (domainmedia.Variant, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	v, ok := r.store.variants[assetID][profile]
	if !ok {
		return domainmedia.Variant{}, apperr.NotFound("Görsel varyantı bulunamadı.")
	}
	failure := reason
	v.FailureReason = &failure
	v.LifecycleStatus = domainmedia.VariantFailed
	v.UpdatedAt = now
	r.store.putVariantLocked(v)
	return v, nil
}

// ListAdvertMediaByAdvert returns the relations joined with asset lifecycle.
func (r MemoryRepository) ListAdvertMediaByAdvert(_ context.Context, advertID uuid.UUID) ([]RelationRow, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	relations := r.store.relationsLocked(advertID)
	out := make([]RelationRow, 0, len(relations))
	for _, rel := range relations {
		asset, ok := r.store.assets[rel.AssetID]
		if !ok {
			return nil, apperr.NotFound(assetNotFoundMessage)
		}
		out = append(out, RelationRow{Relation: rel, AssetLifecycle: asset.LifecycleStatus})
	}
	return out, nil
}

// CountAdvertMediaByAdvert counts the relations of one advert.
func (r MemoryRepository) CountAdvertMediaByAdvert(_ context.Context, advertID uuid.UUID) (int, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	return len(r.store.relations[advertID]), nil
}

// AttachAdvertMedia inserts a relation, rejecting a duplicate asset or a taken
// display order like the table's unique constraints do.
func (r MemoryRepository) AttachAdvertMedia(_ context.Context, rel domainmedia.AdvertMediaRelation) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	for _, existing := range r.store.relations[rel.AdvertID] {
		if existing.AssetID == rel.AssetID {
			return apperr.Conflict(assetAlreadyAttached)
		}
		if existing.DisplayOrder == rel.DisplayOrder {
			return apperr.Conflict(displayOrderTakenMessage)
		}
		if rel.IsCover && existing.IsCover {
			return apperr.Conflict("İlanda zaten bir kapak görseli var.")
		}
	}
	r.store.relations[rel.AdvertID] = append(r.store.relations[rel.AdvertID], rel)
	return nil
}

// DetachAdvertMedia removes a relation and reports whether it was the cover.
func (r MemoryRepository) DetachAdvertMedia(_ context.Context, advertID, assetID uuid.UUID) (bool, bool, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	relations := r.store.relations[advertID]
	for i, rel := range relations {
		if rel.AssetID != assetID {
			continue
		}
		r.store.relations[advertID] = append(relations[:i:i], relations[i+1:]...)
		return true, rel.IsCover, nil
	}
	return false, false, nil
}

// UpdateAdvertMediaDisplayOrder rewrites one relation's display order.
func (r MemoryRepository) UpdateAdvertMediaDisplayOrder(
	_ context.Context,
	advertID, assetID uuid.UUID,
	displayOrder int,
	now time.Time,
) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	relations := r.store.relations[advertID]
	target := -1
	for i, rel := range relations {
		if rel.AssetID == assetID {
			target = i
			continue
		}
		if rel.DisplayOrder == displayOrder {
			return apperr.Conflict(displayOrderTakenMessage)
		}
	}
	if target < 0 {
		return apperr.NotFound(assetNotAttachedMessage)
	}
	relations[target].DisplayOrder = displayOrder
	relations[target].UpdatedAt = now
	return nil
}

// ClearAdvertCover unsets the cover flag on every relation of one advert.
func (r MemoryRepository) ClearAdvertCover(_ context.Context, advertID uuid.UUID, now time.Time) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	relations := r.store.relations[advertID]
	for i := range relations {
		if relations[i].IsCover {
			relations[i].IsCover = false
			relations[i].UpdatedAt = now
		}
	}
	return nil
}

// SetAdvertCover flags one relation as the cover.
func (r MemoryRepository) SetAdvertCover(_ context.Context, advertID, assetID uuid.UUID, now time.Time) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	relations := r.store.relations[advertID]
	target := -1
	for i, rel := range relations {
		if rel.AssetID == assetID {
			target = i
			continue
		}
		if rel.IsCover {
			return apperr.Conflict("İlanda zaten bir kapak görseli var.")
		}
	}
	if target < 0 {
		return apperr.NotFound(assetNotAttachedMessage)
	}
	relations[target].IsCover = true
	relations[target].UpdatedAt = now
	return nil
}

// FindOwnerAdvertForUpdate returns the owner-scoped advert media slice.
func (r MemoryRepository) FindOwnerAdvertForUpdate(_ context.Context, ownerID, advertID uuid.UUID) (AdvertRef, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	a, ok := r.store.adverts[advertID]
	if !ok || a.OwnerUserID != ownerID {
		return AdvertRef{}, apperr.NotFound(advertNotFoundMessage)
	}
	return AdvertRef{ID: a.ID, Status: a.Status, MediaVersion: a.MediaVersion, DeletedAt: a.DeletedAt}, nil
}

// BumpAdvertMediaVersion increments media_version under an optimistic guard.
func (r MemoryRepository) BumpAdvertMediaVersion(
	_ context.Context,
	ownerID, advertID uuid.UUID,
	expectedMediaVersion int,
	now time.Time,
) (int, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	a, ok := r.store.adverts[advertID]
	if !ok || a.OwnerUserID != ownerID {
		return 0, apperr.NotFound(advertNotFoundMessage)
	}
	if a.DeletedAt != nil ||
		a.MediaVersion != expectedMediaVersion ||
		!domainmedia.AdvertEditableForMedia(a.Status) {
		return 0, apperr.StaleVersion(staleMediaVersionMessage)
	}
	a.MediaVersion++
	r.store.adverts[advertID] = a
	_ = now
	return a.MediaVersion, nil
}

// EnqueueJob inserts a durable job, rejecting a duplicate dedup key.
func (r MemoryRepository) EnqueueJob(_ context.Context, job domainmedia.BackgroundJob) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if job.DeduplicationKey != nil {
		if _, dup := r.store.dedup[*job.DeduplicationKey]; dup {
			return apperr.Conflict("İş kaydı zaten kuyruğa alındı.")
		}
		r.store.dedup[*job.DeduplicationKey] = struct{}{}
	}
	r.store.jobs = append(r.store.jobs, job)
	return nil
}

// FindJobByDedupKey returns the job carrying a dedup key.
func (r MemoryRepository) FindJobByDedupKey(_ context.Context, key string) (domainmedia.BackgroundJob, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	for _, job := range r.store.jobs {
		if job.DeduplicationKey != nil && *job.DeduplicationKey == key {
			return job, nil
		}
	}
	return domainmedia.BackgroundJob{}, apperr.NotFound("İş kaydı bulunamadı.")
}

func (r MemoryRepository) assetLocked(assetID uuid.UUID) (domainmedia.Asset, error) {
	a, ok := r.store.assets[assetID]
	if !ok {
		return domainmedia.Asset{}, apperr.NotFound(assetNotFoundMessage)
	}
	return a, nil
}

// memoryTx is a no-op pgx.Tx used by the in-memory repository.
type memoryTx struct {
	store *MemoryStore
	once  sync.Once
}

func (t *memoryTx) release() {
	t.once.Do(func() { t.store.txMu.Unlock() })
}

func (t *memoryTx) Begin(context.Context) (pgx.Tx, error) { return t, nil }
func (t *memoryTx) Commit(context.Context) error {
	t.release()
	return nil
}
func (t *memoryTx) Rollback(context.Context) error {
	t.release()
	return nil
}
func (t *memoryTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *memoryTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (t *memoryTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (t *memoryTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *memoryTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (t *memoryTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (t *memoryTx) QueryRow(context.Context, string, ...any) pgx.Row        { return nil }
func (t *memoryTx) Conn() *pgx.Conn                                         { return nil }

// fakeUploadHost is a reserved-by-RFC-2606 host, used only so tests have a
// syntactically valid upload URL. It is never a real endpoint, and no real
// provider endpoint is invented anywhere in this slice.
const fakeUploadHost = "https://example.invalid/upload/"

type fakeObject struct {
	contentType string
	body        []byte
}

// FakeStorage is an in-memory Storage test double. Test only: a running process
// must never be wired to it.
type FakeStorage struct {
	mu      sync.Mutex
	objects map[string]fakeObject
	clock   Clock
}

// NewFakeStorage builds an empty fake object store.
func NewFakeStorage(clock Clock) *FakeStorage {
	if clock == nil {
		clock = systemClock{}
	}
	return &FakeStorage{objects: map[string]fakeObject{}, clock: clock}
}

// CreateUploadAuthorization returns a fake short-lived PUT grant for one key.
func (f *FakeStorage) CreateUploadAuthorization(
	_ context.Context,
	objectKey string,
	contentType string,
	_ int64,
	ttl time.Duration,
) (UploadAuth, error) {
	headers := map[string]string{}
	if contentType != "" {
		headers["Content-Type"] = contentType
	}
	return UploadAuth{
		Method:    "PUT",
		URL:       fakeUploadHost + objectKey,
		ExpiresAt: f.clock.Now().Add(ttl),
		Headers:   headers,
		ObjectKey: objectKey,
	}, nil
}

// HeadObject reports what the fake store holds under objectKey.
func (f *FakeStorage) HeadObject(_ context.Context, objectKey string) (ObjectInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	obj, ok := f.objects[objectKey]
	if !ok {
		return ObjectInfo{Exists: false}, nil
	}
	return ObjectInfo{ContentType: obj.contentType, ByteSize: int64(len(obj.body)), Exists: true}, nil
}

// PutObject writes an object into the fake store.
func (f *FakeStorage) PutObject(_ context.Context, objectKey string, contentType string, body []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[objectKey] = fakeObject{contentType: contentType, body: append([]byte(nil), body...)}
	return nil
}

// GetObject reads an object back from the fake store.
func (f *FakeStorage) GetObject(_ context.Context, objectKey string) ([]byte, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	obj, ok := f.objects[objectKey]
	if !ok {
		return nil, "", apperr.NotFound("Nesne bulunamadı.")
	}
	return append([]byte(nil), obj.body...), obj.contentType, nil
}

// Has reports whether the fake store holds objectKey.
func (f *FakeStorage) Has(objectKey string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.objects[objectKey]
	return ok
}

// Test-only fake processing output. These numbers are arbitrary fixtures that
// keep the CHECK constraints satisfiable in tests; the real DETAIL/HOMEPAGE/
// SEARCH pixel sizes and compression quality are configuration values that no
// document has fixed yet.
const (
	fakeMasterEdgePx         = 100
	fakeVariantEdgePx        = 50
	fakeProcessedContentType = "image/png"
)

var (
	pngMagic  = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	jpegMagic = []byte{0xFF, 0xD8, 0xFF}
)

// FakeProcessor is an ImageProcessor test double. It accepts only bytes that
// start with a PNG or JPEG signature and emits fixed-size fake output. Test
// only: it performs no real decoding, normalization or compression.
type FakeProcessor struct{}

// ValidateAndNormalize accepts recognizable image bytes and returns fake master
// output.
func (FakeProcessor) ValidateAndNormalize(_ context.Context, raw []byte, _ string) (ProcessedImage, error) {
	if !looksLikeImage(raw) {
		return ProcessedImage{}, apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   "file",
			Message: "Dosya geçerli bir görsel değil.",
		})
	}
	return ProcessedImage{
		ContentType: fakeProcessedContentType,
		Bytes:       append(append([]byte(nil), pngMagic...), []byte("fake-master")...),
		Width:       fakeMasterEdgePx,
		Height:      fakeMasterEdgePx,
	}, nil
}

// GenerateVariant derives fake variant output from fake master bytes.
func (FakeProcessor) GenerateVariant(_ context.Context, master []byte, profile string) (ProcessedImage, error) {
	if !looksLikeImage(master) {
		return ProcessedImage{}, apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   "file",
			Message: "Canonical master geçerli bir görsel değil.",
		})
	}
	return ProcessedImage{
		ContentType: fakeProcessedContentType,
		Bytes:       append(append([]byte(nil), pngMagic...), []byte("fake-variant-"+profile)...),
		Width:       fakeVariantEdgePx,
		Height:      fakeVariantEdgePx,
	}, nil
}

// FakeImageBytes returns bytes the FakeProcessor accepts, for seeding a fake
// raw upload in tests.
func FakeImageBytes() []byte {
	return append(append([]byte(nil), pngMagic...), []byte("fake-upload")...)
}

func looksLikeImage(b []byte) bool {
	return bytes.HasPrefix(b, pngMagic) || bytes.HasPrefix(b, jpegMagic)
}

var (
	_ Repository     = MemoryRepository{}
	_ Storage        = (*FakeStorage)(nil)
	_ ImageProcessor = FakeProcessor{}
)
