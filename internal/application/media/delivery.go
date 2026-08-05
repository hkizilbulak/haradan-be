package media

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

const (
	publicMediaNotFoundMessage = "Görsel bulunamadı."
	// variantCacheControl is a long-lived immutable policy for content-addressed
	// variant keys (assets/{id}/variants/{profile}).
	variantCacheControl = "public, max-age=31536000, immutable"
)

// PublicDelivery describes a READY public variant that may be streamed or headed
// anonymously. Object keys never leave the application layer.
type PublicDelivery struct {
	AssetID      uuid.UUID
	Profile      string
	ObjectKey    string
	ContentType  string
	ByteSize     *int64
	ETag         string
	LastModified time.Time
	CacheControl string
}

// ResolvePublicDelivery locates a READY, Haradan-owned variant for anonymous
// delivery. Missing, soft-deleted, non-READY, raw/master and foreign keys all
// collapse to the same NOT_FOUND response (enumeration-safe).
func (s *Service) ResolvePublicDelivery(ctx context.Context, assetID uuid.UUID, profile string) (PublicDelivery, error) {
	profile = strings.TrimSpace(profile)
	if assetID == uuid.Nil || !domainmedia.IsKnownDeliveryProfile(profile) {
		return PublicDelivery{}, apperr.NotFound(publicMediaNotFoundMessage)
	}

	asset, err := s.repo.FindAssetByID(ctx, assetID)
	if err != nil {
		if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
			return PublicDelivery{}, apperr.NotFound(publicMediaNotFoundMessage)
		}
		return PublicDelivery{}, err
	}
	if domainmedia.IsSoftDeletedAssetLifecycle(asset.LifecycleStatus) {
		return PublicDelivery{}, apperr.NotFound(publicMediaNotFoundMessage)
	}

	variants, err := s.repo.ListVariantsByAsset(ctx, assetID)
	if err != nil {
		return PublicDelivery{}, err
	}
	var match *domainmedia.Variant
	for i := range variants {
		v := &variants[i]
		if v.TransformProfile != profile {
			continue
		}
		if v.LifecycleStatus != domainmedia.VariantReady {
			continue
		}
		if v.ObjectKey == nil || !domainmedia.IsOwnedVariantObjectKey(assetID, profile, *v.ObjectKey) {
			continue
		}
		match = v
		break
	}
	if match == nil {
		return PublicDelivery{}, apperr.NotFound(publicMediaNotFoundMessage)
	}

	out := PublicDelivery{
		AssetID:      assetID,
		Profile:      profile,
		ObjectKey:    *match.ObjectKey,
		CacheControl: variantCacheControl,
	}
	if match.ContentType != nil {
		out.ContentType = strings.TrimSpace(*match.ContentType)
	}
	if match.ByteSize != nil {
		size := *match.ByteSize
		out.ByteSize = &size
	}
	return out, nil
}

// HeadPublicObject returns provider metadata for a resolved delivery key.
func (s *Service) HeadPublicObject(ctx context.Context, delivery PublicDelivery) (ObjectInfo, error) {
	info, err := s.storage.HeadObject(ctx, delivery.ObjectKey)
	if err != nil {
		return ObjectInfo{}, err
	}
	if !info.Exists {
		return ObjectInfo{}, apperr.NotFound(publicMediaNotFoundMessage)
	}
	return info, nil
}

// OpenPublicObject opens a streaming body for a resolved delivery key.
func (s *Service) OpenPublicObject(ctx context.Context, delivery PublicDelivery) (ObjectReader, error) {
	reader, err := s.storage.OpenObject(ctx, delivery.ObjectKey)
	if err != nil {
		if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
			return ObjectReader{}, apperr.NotFound(publicMediaNotFoundMessage)
		}
		return ObjectReader{}, err
	}
	return reader, nil
}
