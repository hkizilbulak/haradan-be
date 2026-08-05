package media_test

import (
	"context"
	"io"
	"testing"

	"github.com/google/uuid"

	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

func TestResolvePublicDeliveryReadyVariant(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	assetID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)
	key := domainmedia.VariantObjectKey(assetID, domainmedia.ProfileDetail)
	ct := "image/png"
	size := int64(12)
	f.store.PutVariant(domainmedia.Variant{
		ID:               uuid.New(),
		AssetID:          assetID,
		TransformProfile: domainmedia.ProfileDetail,
		ObjectKey:        &key,
		LifecycleStatus:  domainmedia.VariantReady,
		ContentType:      &ct,
		ByteSize:         &size,
	})
	if err := f.storage.PutObject(ctx, key, ct, []byte("variant-body")); err != nil {
		t.Fatal(err)
	}

	delivery, err := f.svc.ResolvePublicDelivery(ctx, assetID, domainmedia.ProfileDetail)
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

func TestResolvePublicDeliveryDeniesRawMasterAndMissing(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	assetID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)
	raw := domainmedia.RawObjectKey(assetID)
	f.store.PutVariant(domainmedia.Variant{
		ID:               uuid.New(),
		AssetID:          assetID,
		TransformProfile: domainmedia.ProfileDetail,
		ObjectKey:        &raw, // wrong key — must not be delivered
		LifecycleStatus:  domainmedia.VariantReady,
	})

	_, err := f.svc.ResolvePublicDelivery(ctx, assetID, domainmedia.ProfileDetail)
	if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindNotFound {
		t.Fatalf("want not found for raw key, got %v", err)
	}

	_, err = f.svc.ResolvePublicDelivery(ctx, uuid.New(), domainmedia.ProfileDetail)
	if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindNotFound {
		t.Fatalf("want not found for missing asset, got %v", err)
	}

	deleted := f.seedAsset(f.owner, domainmedia.AssetCleanupCandidate)
	_, err = f.svc.ResolvePublicDelivery(ctx, deleted, domainmedia.ProfileBanner)
	if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindNotFound {
		t.Fatalf("want not found for soft-deleted, got %v", err)
	}
}

func TestResolvePublicDeliveryBannerProfile(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	assetID := f.seedAsset(f.owner, domainmedia.AssetMasterReady)
	key := domainmedia.VariantObjectKey(assetID, domainmedia.ProfileBanner)
	ct := "image/jpeg"
	f.store.PutVariant(domainmedia.Variant{
		ID:               uuid.New(),
		AssetID:          assetID,
		TransformProfile: domainmedia.ProfileBanner,
		ObjectKey:        &key,
		LifecycleStatus:  domainmedia.VariantReady,
		ContentType:      &ct,
	})

	delivery, err := f.svc.ResolvePublicDelivery(ctx, assetID, domainmedia.ProfileBanner)
	if err != nil {
		t.Fatalf("resolve banner: %v", err)
	}
	wantURL := domainmedia.PublicDeliveryURL(assetID, domainmedia.ProfileBanner)
	if wantURL == "" || delivery.Profile != domainmedia.ProfileBanner {
		t.Fatalf("delivery=%+v wantURL=%q", delivery, wantURL)
	}
}

// Ensure FakeStorage satisfies streaming Storage.
var _ appmedia.Storage = (*appmedia.FakeStorage)(nil)
