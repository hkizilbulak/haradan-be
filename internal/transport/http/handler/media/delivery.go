package media

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

// DeliverPublicMedia handles anonymous GET|HEAD /v1/media/{assetId}/{profile}.
//
// Range requests are intentionally not implemented yet; clients receive the
// full object. Object keys and storage credentials never appear in responses.
func (h *Handler) DeliverPublicMedia(c *gin.Context) {
	assetID, err := uuid.Parse(strings.TrimSpace(c.Param("assetId")))
	if err != nil || assetID == uuid.Nil {
		h.respond(c, h.logger, apperr.NotFound(assetNotFoundPublic))
		return
	}
	profile := strings.TrimSpace(c.Param("profile"))
	h.deliverPublicMedia(c, assetID, profile)
}

// GetPublicMedia handles OpenAPI GET /v1/media/{assetId}/{profile}.
func (h *Handler) GetPublicMedia(c *gin.Context, assetID generated.AssetIdPath, profile generated.MediaDeliveryProfile) {
	h.deliverPublicMedia(c, assetID, string(profile))
}

func (h *Handler) deliverPublicMedia(c *gin.Context, assetID uuid.UUID, profile string) {
	if !domainmedia.IsKnownDeliveryProfile(profile) {
		h.respond(c, h.logger, apperr.NotFound(assetNotFoundPublic))
		return
	}

	delivery, err := h.svc.ResolvePublicDelivery(c.Request.Context(), assetID, profile)
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
