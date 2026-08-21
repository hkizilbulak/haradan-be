package media_test

import (
	"context"
	"io"
	"testing"

	"github.com/google/uuid"

	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainbanner "github.com/hkizilbulak/haradan-be/internal/domain/banner"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

func seedReadyVariant(t *testing.T, f *fixture, assetID uuid.UUID, profile string, body string) string {
	t.Helper()
	key := domainmedia.VariantObjectKey(assetID, profile)
	ct := "image/png"
	size := int64(len(body))
	f.store.PutVariant(domainmedia.Variant{
		ID:               uuid.New(),
		AssetID:          assetID,
		TransformProfile: profile,
		ObjectKey:        &key,
		LifecycleStatus:  domainmedia.VariantReady,
		ContentType:      &ct,
		ByteSize:         &size,
	})
	if err := f.storage.PutObject(context.Background(), key, ct, []byte(body)); err != nil {
		t.Fatal(err)
	}
	return key
}

func attachAdvert(t *testing.T, f *fixture, advertID, assetID uuid.UUID) {
	t.Helper()
	now := f.clock.Now()
	if err := f.store.Repo().AttachAdvertMedia(context.Background(), domainmedia.AdvertMediaRelation{
		ID: uuid.New(), AdvertID: advertID, AssetID: assetID, DisplayOrder: 0, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestResolvePublicDeliveryReadyVariantOnPublishedAdvert(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	assetID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)
	key := seedReadyVariant(t, f, assetID, domainmedia.ProfileDetail, "variant-body")
	advertID := f.seedAdvert(f.owner, "PUBLISHED", 1)
	attachAdvert(t, f, advertID, assetID)

	delivery, err := f.svc.ResolvePublicDelivery(ctx, assetID, domainmedia.ProfileDetail, appmedia.PublicDeliveryViewer{})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if delivery.ObjectKey != key || delivery.CacheControl == "" {
		t.Fatalf("delivery=%+v", delivery)
	}

	reader, err := f.svc.OpenPublicObject(ctx, delivery)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer reader.Body.Close()
	body, err := io.ReadAll(reader.Body)
	if err != nil || string(body) != "variant-body" {
		t.Fatalf("body=%q err=%v", body, err)
	}
}

func TestResolvePublicDeliveryDeniesOrphanAndRawMaster(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	assetID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)
	raw := domainmedia.RawObjectKey(assetID)
	f.store.PutVariant(domainmedia.Variant{
		ID:               uuid.New(),
		AssetID:          assetID,
		TransformProfile: domainmedia.ProfileDetail,
		ObjectKey:        &raw,
		LifecycleStatus:  domainmedia.VariantReady,
	})
	// READY owned key but not attached → orphan.
	seedReadyVariant(t, f, assetID, domainmedia.ProfileHomepage, "orphan")

	_, err := f.svc.ResolvePublicDelivery(ctx, assetID, domainmedia.ProfileDetail, appmedia.PublicDeliveryViewer{})
	if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindNotFound {
		t.Fatalf("want not found for raw key, got %v", err)
	}
	_, err = f.svc.ResolvePublicDelivery(ctx, assetID, domainmedia.ProfileHomepage, appmedia.PublicDeliveryViewer{})
	if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindNotFound {
		t.Fatalf("want not found for orphan, got %v", err)
	}

	_, err = f.svc.ResolvePublicDelivery(ctx, uuid.New(), domainmedia.ProfileDetail, appmedia.PublicDeliveryViewer{})
	if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindNotFound {
		t.Fatalf("want not found for missing asset, got %v", err)
	}

	deleted := f.seedAsset(f.owner, domainmedia.AssetCleanupCandidate)
	_, err = f.svc.ResolvePublicDelivery(ctx, deleted, domainmedia.ProfileBanner, appmedia.PublicDeliveryViewer{})
	if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindNotFound {
		t.Fatalf("want not found for soft-deleted, got %v", err)
	}
}

func TestResolvePublicDeliveryAllowsDraftAdvertMedia(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	assetID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)
	seedReadyVariant(t, f, assetID, domainmedia.ProfileDetail, "draft")
	advertID := f.seedAdvert(f.owner, "DRAFT", 1)
	attachAdvert(t, f, advertID, assetID)

	// Draft advert media is displayable anonymously for browser <img> tags
	delivery, err := f.svc.ResolvePublicDelivery(ctx, assetID, domainmedia.ProfileDetail, appmedia.PublicDeliveryViewer{})
	if err != nil {
		t.Fatalf("resolve draft advert media: %v", err)
	}
	if delivery.ObjectKey == "" {
		t.Fatalf("delivery key empty")
	}
}

func TestResolvePublicDeliveryDeniesSoftDeletedAdvertMedia(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	assetID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)
	seedReadyVariant(t, f, assetID, domainmedia.ProfileSearch, "deleted-advert-media")
	advertID := f.seedAdvert(f.owner, "DRAFT", 1)
	attachAdvert(t, f, advertID, assetID)

	// Soft delete the advert
	now := f.clock.Now()
	f.store.PutAdvert(appmedia.MemoryAdvert{
		ID: advertID, OwnerUserID: f.owner, Status: "DRAFT", DeletedAt: &now, MediaVersion: 1,
	})

	// After advert soft delete, media variant is no longer accessible
	_, err := f.svc.ResolvePublicDelivery(ctx, assetID, domainmedia.ProfileSearch, appmedia.PublicDeliveryViewer{})
	if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindNotFound {
		t.Fatalf("want not found for soft deleted advert media, got %v", err)
	}
}

func TestResolvePublicDeliveryAdminPreviewPending(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	assetID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)
	seedReadyVariant(t, f, assetID, domainmedia.ProfileHomepage, "admin")
	advertID := f.seedAdvert(f.owner, "PENDING_REVIEW", 1)
	attachAdvert(t, f, advertID, assetID)

	adminID := uuid.New()
	viewer := appmedia.PublicDeliveryViewer{UserID: adminID, Role: string(domainuser.RoleAdmin)}
	if _, err := f.svc.ResolvePublicDelivery(ctx, assetID, domainmedia.ProfileHomepage, viewer); err != nil {
		t.Fatalf("admin preview: %v", err)
	}
}

func TestResolvePublicDeliveryBannerLifecycle(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	assetID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)
	seedReadyVariant(t, f, assetID, domainmedia.ProfileBanner, "banner")

	_, err := f.svc.ResolvePublicDelivery(ctx, assetID, domainmedia.ProfileBanner, appmedia.PublicDeliveryViewer{})
	if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindNotFound {
		t.Fatalf("want not found without banner attachment, got %v", err)
	}
	admin := appmedia.PublicDeliveryViewer{UserID: uuid.New(), Role: string(domainuser.RoleAdmin)}
	if _, err := f.svc.ResolvePublicDelivery(ctx, assetID, domainmedia.ProfileBanner, admin); err != nil {
		t.Fatalf("admin pre-create banner preview: %v", err)
	}

	f.store.PutBanner(appmedia.MemoryBanner{
		ID: uuid.New(), AssetID: assetID, Status: string(domainbanner.StatusInactive),
	})
	_, err = f.svc.ResolvePublicDelivery(ctx, assetID, domainmedia.ProfileBanner, appmedia.PublicDeliveryViewer{})
	if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindNotFound {
		t.Fatalf("want not found for inactive banner, got %v", err)
	}

	f.store.PutBanner(appmedia.MemoryBanner{
		ID: uuid.New(), AssetID: assetID, Status: string(domainbanner.StatusActive),
	})
	delivery, err := f.svc.ResolvePublicDelivery(ctx, assetID, domainmedia.ProfileBanner, appmedia.PublicDeliveryViewer{})
	if err != nil {
		t.Fatalf("active banner: %v", err)
	}
	if delivery.Profile != domainmedia.ProfileBanner {
		t.Fatalf("delivery=%+v", delivery)
	}

	inactiveAsset := f.seedAsset(f.owner, domainmedia.AssetMasterReady)
	seedReadyVariant(t, f, inactiveAsset, domainmedia.ProfileBanner, "inactive")
	f.store.PutBanner(appmedia.MemoryBanner{
		ID: uuid.New(), AssetID: inactiveAsset, Status: string(domainbanner.StatusInactive),
	})
	if _, err := f.svc.ResolvePublicDelivery(ctx, inactiveAsset, domainmedia.ProfileBanner, admin); err != nil {
		t.Fatalf("admin inactive banner preview: %v", err)
	}
	_, err = f.svc.ResolvePublicDelivery(ctx, inactiveAsset, domainmedia.ProfileBanner, appmedia.PublicDeliveryViewer{})
	if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindNotFound {
		t.Fatalf("anonymous inactive banner must 404, got %v", err)
	}
}

// Ensure FakeStorage satisfies streaming Storage.
var _ appmedia.Storage = (*appmedia.FakeStorage)(nil)
