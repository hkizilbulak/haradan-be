package media_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	pgadvert "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/advert"
	pgmedia "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/media"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/testutil"
	pguser "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/user"
)

type refs struct {
	owner    uuid.UUID
	stranger uuid.UUID
	advert   int64
}

func seedRefs(t *testing.T, ctx context.Context, tx pgx.Tx, now time.Time) refs {
	t.Helper()
	users := pguser.NewRepository(tx)

	newUser := func(prefix string) uuid.UUID {
		email := prefix + "-" + uuid.NewString() + "@example.com"
		u := domainuser.User{
			ID: uuid.New(), Email: email, EmailNormalized: email,
			PasswordHash: "hash", Role: domainuser.RoleUser, Status: domainuser.StatusActive,
			FirstName: "A", LastName: "B", SecurityStamp: uuid.New(), CreatedAt: now, UpdatedAt: now,
		}
		if err := users.Create(ctx, u); err != nil {
			t.Fatalf("create user: %v", err)
		}
		return u.ID
	}

	out := refs{owner: newUser("media-owner"), stranger: newUser("media-stranger")}

	adverts := pgadvert.NewRepository(nil).WithTx(tx)
	advert := domainadvert.Advert{
		OwnerUserID:  out.owner,
		Status:       domainadvert.StatusDraft,
		Properties:   domainadvert.EmptyProperties(),
		Version:      1,
		MediaVersion: 1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := adverts.Create(ctx, &advert); err != nil {
		t.Fatalf("create advert: %v", err)
	}
	out.advert = advert.ID
	return out
}

func newPendingAsset(ownerID uuid.UUID, now time.Time) domainmedia.Asset {
	id := uuid.New()
	rawKey := domainmedia.RawObjectKey(id)
	return domainmedia.Asset{
		ID:                id,
		OwnerUserID:       ownerID,
		Provider:          domainmedia.ProviderB2,
		RawObjectKey:      &rawKey,
		LifecycleStatus:   domainmedia.AssetUploadPending,
		TechnicalMetadata: domainmedia.EmptyMetadata(),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func TestRepositoryAssetLifecycleIntegration(t *testing.T) {
	ctx, tx, cleanup := testutil.OpenTestTx(t)
	defer cleanup()

	now := time.Now().UTC().Truncate(time.Microsecond)
	ref := seedRefs(t, ctx, tx, now)
	repo := pgmedia.NewRepository(nil).WithTx(tx)

	asset := newPendingAsset(ref.owner, now)
	if err := repo.CreateAsset(ctx, asset); err != nil {
		t.Fatalf("create asset: %v", err)
	}

	found, err := repo.FindAssetByIDForOwner(ctx, ref.owner, asset.ID)
	if err != nil {
		t.Fatalf("find asset for owner: %v", err)
	}
	if found.LifecycleStatus != domainmedia.AssetUploadPending || found.Provider != domainmedia.ProviderB2 {
		t.Fatalf("found=%+v", found)
	}

	// A foreign owner must not be able to tell the asset exists.
	if _, err := repo.FindAssetByIDForOwner(ctx, ref.stranger, asset.ID); err == nil {
		t.Fatal("cross-user asset read must fail")
	} else if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("want NOT_FOUND, got %v", err)
	}

	rawKey := domainmedia.RawObjectKey(asset.ID)
	uploaded, err := repo.SetAssetUploaded(ctx, asset.ID, rawKey, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("set uploaded: %v", err)
	}
	if uploaded.LifecycleStatus != domainmedia.AssetUploaded ||
		uploaded.RawObjectKey == nil || *uploaded.RawObjectKey != rawKey {
		t.Fatalf("uploaded=%+v", uploaded)
	}

	// Replaying the same transition is rejected instead of writing twice.
	if _, err := repo.SetAssetUploaded(ctx, asset.ID, rawKey, now.Add(2*time.Minute)); err == nil {
		t.Fatal("repeated upload confirmation must fail")
	} else if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeInvalidState {
		t.Fatalf("want INVALID_STATE, got %v", err)
	}

	validating, err := repo.SetAssetValidating(ctx, asset.ID, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("set validating: %v", err)
	}
	if validating.LifecycleStatus != domainmedia.AssetValidating {
		t.Fatalf("validating=%+v", validating)
	}

	masterKey := domainmedia.MasterObjectKey(asset.ID)
	master, err := repo.SetAssetMasterReady(ctx, asset.ID, masterKey, "image/png", 4096, 100, 100, now.Add(4*time.Minute))
	if err != nil {
		t.Fatalf("set master ready: %v", err)
	}
	if master.LifecycleStatus != domainmedia.AssetMasterReady {
		t.Fatalf("master=%+v", master)
	}
	if master.MasterObjectKey == nil || master.ContentType == nil ||
		master.ByteSize == nil || master.WidthPx == nil || master.HeightPx == nil {
		t.Fatalf("master ready fields incomplete: %+v", master)
	}

	// A worker read is not owner scoped; it is driven by the job payload.
	byID, err := repo.FindAssetByID(ctx, asset.ID)
	if err != nil {
		t.Fatalf("find asset by id: %v", err)
	}
	if byID.ID != asset.ID {
		t.Fatalf("byID=%+v", byID)
	}
}

func TestRepositoryAssetValidationFailureIntegration(t *testing.T) {
	ctx, tx, cleanup := testutil.OpenTestTx(t)
	defer cleanup()

	now := time.Now().UTC().Truncate(time.Microsecond)
	ref := seedRefs(t, ctx, tx, now)
	repo := pgmedia.NewRepository(nil).WithTx(tx)

	asset := newPendingAsset(ref.owner, now)
	if err := repo.CreateAsset(ctx, asset); err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if _, err := repo.SetAssetUploaded(ctx, asset.ID, domainmedia.RawObjectKey(asset.ID), now); err != nil {
		t.Fatalf("set uploaded: %v", err)
	}

	failed, err := repo.SetAssetValidationFailed(ctx, asset.ID, "Dosya geçerli bir görsel değil.", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("set validation failed: %v", err)
	}
	if failed.LifecycleStatus != domainmedia.AssetValidationFailed {
		t.Fatalf("failed=%+v", failed)
	}
	if failed.FailureReason == nil || *failed.FailureReason == "" {
		t.Fatal("VALIDATION_FAILED needs a non-empty reason")
	}
}

func TestRepositoryVariantsIntegration(t *testing.T) {
	ctx, tx, cleanup := testutil.OpenTestTx(t)
	defer cleanup()

	now := time.Now().UTC().Truncate(time.Microsecond)
	ref := seedRefs(t, ctx, tx, now)
	repo := pgmedia.NewRepository(nil).WithTx(tx)

	asset := newPendingAsset(ref.owner, now)
	if err := repo.CreateAsset(ctx, asset); err != nil {
		t.Fatalf("create asset: %v", err)
	}

	for _, profile := range domainmedia.RequiredTransformProfiles() {
		v, err := repo.UpsertPendingVariant(ctx, domainmedia.Variant{
			ID: uuid.New(),
			AssetID:           asset.ID,
			TransformProfile:  profile,
			LifecycleStatus:   domainmedia.VariantPending,
			TechnicalMetadata: domainmedia.EmptyMetadata(),
			CreatedAt:         now,
			UpdatedAt:         now,
		})
		if err != nil {
			t.Fatalf("upsert variant %s: %v", profile, err)
		}
		if v.LifecycleStatus != domainmedia.VariantPending {
			t.Fatalf("variant=%+v", v)
		}
	}

	// The same master and profile must never produce a duplicate row.
	first, err := repo.UpsertPendingVariant(ctx, domainmedia.Variant{
		ID: uuid.New(),
		AssetID:           asset.ID,
		TransformProfile:  domainmedia.ProfileDetail,
		LifecycleStatus:   domainmedia.VariantPending,
		TechnicalMetadata: domainmedia.EmptyMetadata(),
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		t.Fatalf("re-upsert variant: %v", err)
	}
	variants, err := repo.ListVariantsByAsset(ctx, asset.ID)
	if err != nil {
		t.Fatalf("list variants: %v", err)
	}
	if len(variants) != 3 {
		t.Fatalf("want 3 variants, got %d", len(variants))
	}

	ready, err := repo.MarkVariantReady(
		ctx, asset.ID, domainmedia.ProfileDetail,
		domainmedia.VariantObjectKey(asset.ID, domainmedia.ProfileDetail),
		"image/png", 1024, 50, 50, now.Add(time.Minute),
	)
	if err != nil {
		t.Fatalf("mark variant ready: %v", err)
	}
	if ready.ID != first.ID {
		t.Fatalf("the existing variant row must be updated: %s vs %s", ready.ID, first.ID)
	}
	if ready.LifecycleStatus != domainmedia.VariantReady || ready.ObjectKey == nil ||
		ready.ContentType == nil || ready.ByteSize == nil || ready.WidthPx == nil || ready.HeightPx == nil {
		t.Fatalf("ready variant fields incomplete: %+v", ready)
	}

	failedVariant, err := repo.MarkVariantFailed(ctx, asset.ID, domainmedia.ProfileSearch, "Sıkıştırma başarısız.", now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("mark variant failed: %v", err)
	}
	if failedVariant.LifecycleStatus != domainmedia.VariantFailed ||
		failedVariant.FailureReason == nil || *failedVariant.FailureReason == "" {
		t.Fatalf("failed variant=%+v", failedVariant)
	}

	// One READY profile says nothing about the others.
	after, err := repo.ListVariantsByAsset(ctx, asset.ID)
	if err != nil {
		t.Fatalf("list variants after: %v", err)
	}
	statuses := map[string]domainmedia.VariantLifecycle{}
	for _, v := range after {
		statuses[v.TransformProfile] = v.LifecycleStatus
	}
	if statuses[domainmedia.ProfileDetail] != domainmedia.VariantReady ||
		statuses[domainmedia.ProfileSearch] != domainmedia.VariantFailed ||
		statuses[domainmedia.ProfileHomepage] != domainmedia.VariantPending {
		t.Fatalf("statuses=%+v", statuses)
	}
}

func TestRepositoryAdvertMediaRelationsIntegration(t *testing.T) {
	ctx, tx, cleanup := testutil.OpenTestTx(t)
	defer cleanup()

	now := time.Now().UTC().Truncate(time.Microsecond)
	ref := seedRefs(t, ctx, tx, now)
	repo := pgmedia.NewRepository(nil).WithTx(tx)

	first := newPendingAsset(ref.owner, now)
	second := newPendingAsset(ref.owner, now)
	for _, a := range []domainmedia.Asset{first, second} {
		if err := repo.CreateAsset(ctx, a); err != nil {
			t.Fatalf("create asset: %v", err)
		}
	}
	if _, err := repo.SetAssetUploaded(ctx, first.ID, domainmedia.RawObjectKey(first.ID), now); err != nil {
		t.Fatalf("set uploaded: %v", err)
	}

	advert, err := repo.FindOwnerAdvertForUpdate(ctx, ref.owner, ref.advert)
	if err != nil {
		t.Fatalf("find owner advert: %v", err)
	}
	if advert.Status != string(domainadvert.StatusDraft) || advert.MediaVersion != 1 || advert.IsDeleted() {
		t.Fatalf("advert=%+v", advert)
	}
	// A foreign advert must be indistinguishable from a missing one.
	if _, err := repo.FindOwnerAdvertForUpdate(ctx, ref.stranger, ref.advert); err == nil {
		t.Fatal("cross-user advert read must fail")
	} else if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("want NOT_FOUND, got %v", err)
	}

	relation := func(assetID uuid.UUID, order int, isCover bool) domainmedia.AdvertMediaRelation {
		return domainmedia.AdvertMediaRelation{
			ID: uuid.New(),
			AdvertID:     ref.advert,
			AssetID:      assetID,
			DisplayOrder: order,
			IsCover:      isCover,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
	}
	if err := repo.AttachAdvertMedia(ctx, relation(first.ID, 0, true)); err != nil {
		t.Fatalf("attach first: %v", err)
	}
	if err := repo.AttachAdvertMedia(ctx, relation(second.ID, 1, false)); err != nil {
		t.Fatalf("attach second: %v", err)
	}

	// The same asset twice, a taken display order and a second cover are all
	// conflicts enforced by the table, not internal errors.
	testutil.WithSavepoint(t, ctx, tx, func() {
		if err := repo.AttachAdvertMedia(ctx, relation(first.ID, 2, false)); err == nil {
			t.Fatal("duplicate asset attach must fail")
		} else if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeConflict {
			t.Fatalf("want CONFLICT, got %v", err)
		}
	})
	testutil.WithSavepoint(t, ctx, tx, func() {
		if err := repo.AttachAdvertMedia(ctx, relation(uuid.New(), 0, false)); err == nil {
			t.Fatal("attaching an unknown asset must fail")
		}
	})

	rows, err := repo.ListAdvertMediaByAdvert(ctx, ref.advert)
	if err != nil {
		t.Fatalf("list advert media: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 relations, got %d", len(rows))
	}
	if rows[0].Relation.AssetID != first.ID || rows[1].Relation.AssetID != second.ID {
		t.Fatalf("rows must come back in display order: %+v", rows)
	}
	if rows[0].AssetLifecycle != domainmedia.AssetUploaded ||
		rows[1].AssetLifecycle != domainmedia.AssetUploadPending {
		t.Fatalf("asset lifecycles must be joined: %+v", rows)
	}

	count, err := repo.CountAdvertMediaByAdvert(ctx, ref.advert)
	if err != nil {
		t.Fatalf("count advert media: %v", err)
	}
	if count != 2 {
		t.Fatalf("count=%d", count)
	}

	// Two-phase reorder: park both rows above the final range, then write the
	// final orders, so the unique (advert_id, display_order) index always holds.
	for i, id := range []uuid.UUID{second.ID, first.ID} {
		if err := repo.UpdateAdvertMediaDisplayOrder(ctx, ref.advert, id, 2+i, now); err != nil {
			t.Fatalf("temp order: %v", err)
		}
	}
	for i, id := range []uuid.UUID{second.ID, first.ID} {
		if err := repo.UpdateAdvertMediaDisplayOrder(ctx, ref.advert, id, i, now); err != nil {
			t.Fatalf("final order: %v", err)
		}
	}
	reordered, err := repo.ListAdvertMediaByAdvert(ctx, ref.advert)
	if err != nil {
		t.Fatalf("list after reorder: %v", err)
	}
	if reordered[0].Relation.AssetID != second.ID || reordered[1].Relation.AssetID != first.ID {
		t.Fatalf("reordered=%+v", reordered)
	}

	// Moving the cover requires clearing the old one in the same transaction.
	if err := repo.ClearAdvertCover(ctx, ref.advert, now); err != nil {
		t.Fatalf("clear cover: %v", err)
	}
	if err := repo.SetAdvertCover(ctx, ref.advert, second.ID, now); err != nil {
		t.Fatalf("set cover: %v", err)
	}
	withCover, err := repo.ListAdvertMediaByAdvert(ctx, ref.advert)
	if err != nil {
		t.Fatalf("list after cover: %v", err)
	}
	for _, row := range withCover {
		if row.Relation.IsCover != (row.Relation.AssetID == second.ID) {
			t.Fatalf("exactly the new cover must be flagged: %+v", withCover)
		}
	}

	newVersion, err := repo.BumpAdvertMediaVersion(ctx, ref.owner, ref.advert, 1, now)
	if err != nil {
		t.Fatalf("bump media version: %v", err)
	}
	if newVersion != 2 {
		t.Fatalf("newVersion=%d", newVersion)
	}
	if _, err := repo.BumpAdvertMediaVersion(ctx, ref.owner, ref.advert, 1, now); err == nil {
		t.Fatal("stale media version must be rejected")
	} else if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeStaleVersion {
		t.Fatalf("want STALE_VERSION, got %v", err)
	}
	if _, err := repo.BumpAdvertMediaVersion(ctx, ref.stranger, ref.advert, 2, now); err == nil {
		t.Fatal("cross-user bump must be rejected")
	}

	removed, wasCover, err := repo.DetachAdvertMedia(ctx, ref.advert, second.ID)
	if err != nil {
		t.Fatalf("detach: %v", err)
	}
	if !removed || !wasCover {
		t.Fatalf("removed=%v wasCover=%v", removed, wasCover)
	}
	// Detaching only breaks the relation; the asset row stays untouched.
	if _, err := repo.FindAssetByID(ctx, second.ID); err != nil {
		t.Fatalf("detach must not delete the asset: %v", err)
	}
	removed, _, err = repo.DetachAdvertMedia(ctx, ref.advert, second.ID)
	if err != nil {
		t.Fatalf("repeat detach: %v", err)
	}
	if removed {
		t.Fatal("repeat detach must report nothing removed")
	}
}

func TestRepositoryJobDeduplicationIntegration(t *testing.T) {
	ctx, tx, cleanup := testutil.OpenTestTx(t)
	defer cleanup()

	now := time.Now().UTC().Truncate(time.Microsecond)
	ref := seedRefs(t, ctx, tx, now)
	repo := pgmedia.NewRepository(nil).WithTx(tx)

	asset := newPendingAsset(ref.owner, now)
	if err := repo.CreateAsset(ctx, asset); err != nil {
		t.Fatalf("create asset: %v", err)
	}

	key := domainmedia.ValidateJobDedupKey(asset.ID)
	job := domainmedia.BackgroundJob{
		ID: uuid.New(),
		JobType:          domainmedia.JobValidateAndNormalize,
		Status:           domainmedia.JobQueued,
		Payload:          domainmedia.EmptyMetadata(),
		DeduplicationKey: &key,
		MaxAttempts:      5,
		AvailableAt:      now,
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := repo.EnqueueJob(ctx, job); err != nil {
		t.Fatalf("enqueue job: %v", err)
	}

	// A repeated completion must not create a second job for the same asset.
	duplicate := job
	duplicate.ID = uuid.New()
	testutil.WithSavepoint(t, ctx, tx, func() {
		if err := repo.EnqueueJob(ctx, duplicate); err == nil {
			t.Fatal("duplicate dedup key must fail")
		} else if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeConflict {
			t.Fatalf("want CONFLICT, got %v", err)
		}
	})

	found, err := repo.FindJobByDedupKey(ctx, key)
	if err != nil {
		t.Fatalf("find job by dedup key: %v", err)
	}
	if found.ID != job.ID || found.JobType != domainmedia.JobValidateAndNormalize ||
		found.Status != domainmedia.JobQueued {
		t.Fatalf("found=%+v", found)
	}

	if _, err := repo.FindJobByDedupKey(ctx, "MEDIA_VALIDATE_AND_NORMALIZE:"+uuid.NewString()); err == nil {
		t.Fatal("unknown dedup key must not be found")
	} else if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("want NOT_FOUND, got %v", err)
	}

	variantKey := domainmedia.VariantJobDedupKey(asset.ID, domainmedia.ProfileDetail)
	variantJob := job
	variantJob.ID = uuid.New()
	variantJob.JobType = domainmedia.JobGenerateVariant
	variantJob.DeduplicationKey = &variantKey
	if err := repo.EnqueueJob(ctx, variantJob); err != nil {
		t.Fatalf("enqueue variant job: %v", err)
	}
}
