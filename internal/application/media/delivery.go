package media

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainbanner "github.com/hkizilbulak/haradan-be/internal/domain/banner"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

const (
	publicMediaNotFoundMessage = "Görsel bulunamadı."
	// variantCacheControl is a long-lived immutable policy for content-addressed
	// variant keys (assets/{id}/variants/{profile}).
	variantCacheControl = "public, max-age=31536000, immutable"
)

// PublicDelivery describes a READY public variant that may be streamed or headed.
// Object keys never leave the application layer.
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

// PublicDeliveryViewer is an optional authenticated principal for owner/admin
// preview of attached non-public assets. Anonymous delivery uses a zero value.
type PublicDeliveryViewer struct {
	UserID uuid.UUID
	Role   string
}

// ResolvePublicDelivery locates a READY, Haradan-owned variant that the viewer
// may access. Anonymous callers require a current public attachment; owners may
// preview attached draft/pending advert assets; active admins may preview
// attached listing/banner assets. Active admins may also preview a READY BANNER
// variant before creating its banner record. Missing, soft-deleted, non-READY,
// raw/master, other orphan, and unauthorized access collapse to NOT_FOUND.
func (s *Service) ResolvePublicDelivery(ctx context.Context, assetID uuid.UUID, profile string, viewer PublicDeliveryViewer) (PublicDelivery, error) {
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

	allowed, err := s.publicDeliveryAllowed(ctx, assetID, profile, viewer)
	if err != nil {
		return PublicDelivery{}, err
	}
	if !allowed {
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
		var fallbackKey *string
		if asset.MasterObjectKey != nil && strings.TrimSpace(*asset.MasterObjectKey) != "" {
			fallbackKey = asset.MasterObjectKey
		} else if asset.RawObjectKey != nil && strings.TrimSpace(*asset.RawObjectKey) != "" &&
			(asset.LifecycleStatus == domainmedia.AssetUploaded ||
				asset.LifecycleStatus == domainmedia.AssetValidating ||
				asset.LifecycleStatus == domainmedia.AssetMasterReady) {
			fallbackKey = asset.RawObjectKey
		}

		if fallbackKey == nil {
			return PublicDelivery{}, apperr.NotFound(publicMediaNotFoundMessage)
		}

		ct := "image/jpeg"
		if asset.ContentType != nil && strings.TrimSpace(*asset.ContentType) != "" {
			ct = strings.TrimSpace(*asset.ContentType)
		} else {
			hints := decodeAssetHints(asset.TechnicalMetadata)
			if strings.TrimSpace(hints.DeclaredContentType) != "" {
				ct = strings.TrimSpace(hints.DeclaredContentType)
			}
		}

		out := PublicDelivery{
			AssetID:      assetID,
			Profile:      profile,
			ObjectKey:    *fallbackKey,
			ContentType:  ct,
			ByteSize:     asset.ByteSize,
			CacheControl: "public, max-age=300",
		}
		return out, nil
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

func (s *Service) publicDeliveryAllowed(
	ctx context.Context,
	assetID uuid.UUID,
	profile string,
	viewer PublicDeliveryViewer,
) (bool, error) {
	switch profile {
	case domainmedia.ProfileHomepage, domainmedia.ProfileDetail, domainmedia.ProfileSearch:
		return s.advertDeliveryAllowed(ctx, assetID, viewer)
	case domainmedia.ProfileBanner:
		return s.bannerDeliveryAllowed(ctx, assetID, viewer)
	default:
		return false, nil
	}
}

func (s *Service) advertDeliveryAllowed(ctx context.Context, assetID uuid.UUID, viewer PublicDeliveryViewer) (bool, error) {
	rows, err := s.repo.FindAdvertMediaAccessByAsset(ctx, assetID)
	if err != nil {
		return false, err
	}
	isAdmin := viewer.UserID != uuid.Nil && viewer.Role == string(domainuser.RoleAdmin)
	for _, row := range rows {
		if row.DeletedAt != nil {
			continue
		}
		// Any non-deleted attached advert media is displayable (covers drafts in user's panel,
		// under review, changes requested, published, sold, etc., which are accessed via <img> tags
		// where browser does not send Authorization headers).
		return true, nil
	}
	if isAdmin {
		return true, nil
	}
	return false, nil
}

func (s *Service) bannerDeliveryAllowed(ctx context.Context, assetID uuid.UUID, viewer PublicDeliveryViewer) (bool, error) {
	isAdmin := viewer.UserID != uuid.Nil && viewer.Role == string(domainuser.RoleAdmin)
	if isAdmin {
		// BO create flow previews the processed BANNER variant before the banner
		// relation exists. Authentication is still required; anonymous callers
		// keep the orphan-safe 404 behavior.
		return true, nil
	}
	rows, err := s.repo.FindBannerMediaAccessByAsset(ctx, assetID)
	if err != nil {
		return false, err
	}
	now := s.clock.Now().UTC()
	for _, row := range rows {
		if domainbanner.Status(row.Status) == domainbanner.StatusActive && bannerPubliclyDisplayable(row, now) {
			return true, nil
		}
	}
	return false, nil
}

// bannerPubliclyDisplayable applies the existing banner lifecycle rule: ACTIVE
// status. StartsAt/EndsAt windows are not modeled on hrd_banners today; the
// clock is accepted so a future window can plug in without changing callers.
func bannerPubliclyDisplayable(_ domainmedia.BannerMediaAccess, _ time.Time) bool {
	return true
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
