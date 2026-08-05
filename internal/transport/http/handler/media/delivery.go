package media

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authn"
)

// GetPublicMedia handles OpenAPI GET /v1/media/{assetId}/{profile}.
func (h *Handler) GetPublicMedia(c *gin.Context, assetID generated.AssetIdPath, profile generated.MediaDeliveryProfile) {
	h.deliverPublicMedia(c, assetID, string(profile))
}

// HeadPublicMedia handles OpenAPI HEAD /v1/media/{assetId}/{profile}.
func (h *Handler) HeadPublicMedia(c *gin.Context, assetID generated.AssetIdPath, profile generated.MediaDeliveryProfile) {
	h.deliverPublicMedia(c, assetID, string(profile))
}

func (h *Handler) deliverPublicMedia(c *gin.Context, assetID uuid.UUID, profile string) {
	if !domainmedia.IsKnownDeliveryProfile(profile) {
		h.respond(c, h.logger, apperr.NotFound(assetNotFoundPublic))
		return
	}

	viewer := h.resolvePublicDeliveryViewer(c)
	delivery, err := h.svc.ResolvePublicDelivery(c.Request.Context(), assetID, profile, viewer)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}

	head, err := h.svc.HeadPublicObject(c.Request.Context(), delivery)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}

	contentType := strings.TrimSpace(delivery.ContentType)
	if contentType == "" {
		contentType = strings.TrimSpace(head.ContentType)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	etag := strings.TrimSpace(head.ETag)
	lastMod := head.LastModified
	byteSize := head.ByteSize
	if byteSize <= 0 && delivery.ByteSize != nil {
		byteSize = *delivery.ByteSize
	}

	if etag != "" {
		c.Header("ETag", etag)
	}
	if !lastMod.IsZero() {
		c.Header("Last-Modified", lastMod.UTC().Format(http.TimeFormat))
	}
	c.Header("Cache-Control", delivery.CacheControl)
	c.Header("Content-Type", contentType)
	if byteSize > 0 {
		c.Header("Content-Length", strconv.FormatInt(byteSize, 10))
	}
	// Inline display only — never force download.
	c.Header("Content-Disposition", "inline")

	if checkNotModified(c.Request, etag, lastMod) {
		c.Status(http.StatusNotModified)
		return
	}

	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusOK)
		return
	}

	reader, err := h.svc.OpenPublicObject(c.Request.Context(), delivery)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	defer reader.Body.Close()

	// Prefer exact stored/detected type from the live object when present.
	if ct := strings.TrimSpace(reader.ContentType); ct != "" {
		c.Header("Content-Type", ct)
	}
	if reader.ByteSize > 0 {
		c.Header("Content-Length", strconv.FormatInt(reader.ByteSize, 10))
	}
	if et := strings.TrimSpace(reader.ETag); et != "" {
		c.Header("ETag", et)
	}
	if !reader.LastModified.IsZero() {
		c.Header("Last-Modified", reader.LastModified.UTC().Format(http.TimeFormat))
	}

	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, reader.Body)
}

const assetNotFoundPublic = "Görsel bulunamadı."

// resolvePublicDeliveryViewer soft-authenticates an optional Bearer token.
// Absent or invalid tokens stay anonymous (enumeration-safe; never 401 here).
func (h *Handler) resolvePublicDeliveryViewer(c *gin.Context) appmedia.PublicDeliveryViewer {
	if p, ok := authctx.PrincipalFromContext(c.Request.Context()); ok {
		return appmedia.PublicDeliveryViewer{UserID: p.UserID, Role: p.Role}
	}
	if h.auth == nil {
		return appmedia.PublicDeliveryViewer{}
	}
	tok, ok := authn.ExtractBearer(c.GetHeader("Authorization"))
	if !ok {
		return appmedia.PublicDeliveryViewer{}
	}
	principal, err := h.auth.AuthenticateAccessToken(c.Request.Context(), tok)
	if err != nil {
		return appmedia.PublicDeliveryViewer{}
	}
	return viewerFromPrincipal(principal)
}

func viewerFromPrincipal(p domainauth.Principal) appmedia.PublicDeliveryViewer {
	return appmedia.PublicDeliveryViewer{UserID: p.UserID, Role: p.Role}
}

func checkNotModified(r *http.Request, etag string, lastMod time.Time) bool {
	if etag != "" {
		if inm := strings.TrimSpace(r.Header.Get("If-None-Match")); inm != "" {
			for _, candidate := range strings.Split(inm, ",") {
				candidate = strings.TrimSpace(candidate)
				if candidate == "*" || candidate == etag {
					return true
				}
			}
		}
	}
	if !lastMod.IsZero() {
		if ims := strings.TrimSpace(r.Header.Get("If-Modified-Since")); ims != "" {
			since, err := http.ParseTime(ims)
			if err == nil && !lastMod.After(since) {
				return true
			}
		}
	}
	return false
}
