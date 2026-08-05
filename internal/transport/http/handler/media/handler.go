// Package media exposes the MEDIA-01..07 owner-scoped OpenAPI operations.
package media

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/application/authz"
	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bind"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

// ErrorResponder maps application errors to HTTP responses.
type ErrorResponder func(c *gin.Context, logger *slog.Logger, err error)

// Handler exposes the owner-scoped media OpenAPI operations. Identity comes
// exclusively from the authenticated principal; object keys and the storage
// provider are never part of any response this handler produces.
type Handler struct {
	svc     *appmedia.Service
	logger  *slog.Logger
	respond ErrorResponder
}

// NewHandler constructs a media owner HTTP handler.
func NewHandler(svc *appmedia.Service, logger *slog.Logger, respond ErrorResponder) *Handler {
	return &Handler{svc: svc, logger: logger, respond: respond}
}

// InitiateMediaUpload handles POST /v1/media/uploads (MEDIA-01).
func (h *Handler) InitiateMediaUpload(c *gin.Context) {
	ownerID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	var req generated.InitiateMediaUploadRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	in := appmedia.InitiateInput{DeclaredContentType: req.DeclaredContentType}
	if req.DeclaredByteSize != nil {
		size := int64(*req.DeclaredByteSize)
		in.DeclaredByteSize = &size
	}
	out, err := h.svc.InitiateMediaUpload(c.Request.Context(), ownerID, in)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusCreated, mapInitiateView(out))
}

// ConfirmMediaUpload handles POST /v1/media/assets/{assetId}/confirm (MEDIA-02).
func (h *Handler) ConfirmMediaUpload(c *gin.Context, assetID generated.AssetIdPath) {
	ownerID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	out, err := h.svc.ConfirmMediaUpload(c.Request.Context(), ownerID, assetID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusAccepted, mapProcessingView(out))
}

// GetMediaProcessingStatus handles GET /v1/media/assets/{assetId} (MEDIA-03).
func (h *Handler) GetMediaProcessingStatus(c *gin.Context, assetID generated.AssetIdPath) {
	ownerID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	out, err := h.svc.GetMediaProcessingStatus(c.Request.Context(), ownerID, assetID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapProcessingView(out))
}

// InitiateAdminMediaUpload handles the admin equivalent of MEDIA-01.
func (h *Handler) InitiateAdminMediaUpload(c *gin.Context) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	var req generated.InitiateMediaUploadRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	in := appmedia.InitiateInput{DeclaredContentType: req.DeclaredContentType}
	if req.DeclaredByteSize != nil {
		size := int64(*req.DeclaredByteSize)
		in.DeclaredByteSize = &size
	}
	out, err := h.svc.InitiateMediaUpload(c.Request.Context(), actorID, in)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusCreated, mapInitiateView(out))
}

// ConfirmAdminMediaUpload handles the admin equivalent of MEDIA-02.
func (h *Handler) ConfirmAdminMediaUpload(c *gin.Context, assetID generated.AssetIdPath) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	out, err := h.svc.ConfirmMediaUpload(c.Request.Context(), actorID, assetID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusAccepted, mapProcessingView(out))
}

// GetAdminMediaProcessingStatus handles the admin equivalent of MEDIA-03.
func (h *Handler) GetAdminMediaProcessingStatus(c *gin.Context, assetID generated.AssetIdPath) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	out, err := h.svc.GetMediaProcessingStatus(c.Request.Context(), actorID, assetID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapProcessingView(out))
}

// AttachMediaToAdvert handles POST /v1/me/adverts/{advertId}/media (MEDIA-04).
func (h *Handler) AttachMediaToAdvert(c *gin.Context, advertID generated.AdvertIdPath) {
	ownerID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	var req generated.AttachMediaToAdvertRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	out, err := h.svc.AttachMediaToAdvert(c.Request.Context(), ownerID, advertID, req.AssetId, req.DisplayOrder, req.ExpectedMediaVersion)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapOwnerMediaView(out))
}

// DetachMediaFromAdvert handles DELETE /v1/me/adverts/{advertId}/media/{assetId} (MEDIA-05).
func (h *Handler) DetachMediaFromAdvert(
	c *gin.Context,
	advertID generated.AdvertIdPath,
	assetID generated.AssetIdPath,
	params generated.DetachMediaFromAdvertParams,
) {
	ownerID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	out, err := h.svc.DetachMediaFromAdvert(c.Request.Context(), ownerID, advertID, assetID, params.ExpectedMediaVersion)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapOwnerMediaView(out))
}

// ReorderAdvertMedia handles PUT /v1/me/adverts/{advertId}/media/order (MEDIA-06).
func (h *Handler) ReorderAdvertMedia(c *gin.Context, advertID generated.AdvertIdPath) {
	ownerID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	var req generated.ReorderAdvertMediaRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	out, err := h.svc.ReorderAdvertMedia(c.Request.Context(), ownerID, advertID, req.OrderedAssetIds, req.ExpectedMediaVersion)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapOwnerMediaView(out))
}

// SetAdvertCover handles PUT /v1/me/adverts/{advertId}/media/cover (MEDIA-07).
func (h *Handler) SetAdvertCover(c *gin.Context, advertID generated.AdvertIdPath) {
	ownerID, ok := h.requirePrincipal(c)
	if !ok {
		return
	}
	var req generated.SetAdvertCoverRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	out, err := h.svc.SetAdvertCover(c.Request.Context(), ownerID, advertID, req.AssetId, req.ExpectedMediaVersion)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapOwnerMediaView(out))
}

func (h *Handler) requirePrincipal(c *gin.Context) (uuid.UUID, bool) {
	p, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return uuid.Nil, false
	}
	return p.UserID, true
}

func (h *Handler) requireAdminBO(c *gin.Context) (uuid.UUID, bool) {
	p, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return uuid.Nil, false
	}
	if err := authz.RequireAdminBO(p); err != nil {
		h.respond(c, h.logger, err)
		return uuid.Nil, false
	}
	return p.UserID, true
}

// mapInitiateView projects MEDIA-01 output. No object key ever reaches this
// mapping: appmedia.UploadAuthView already stripped it.
func mapInitiateView(v appmedia.InitiateView) generated.InitiateMediaUploadResponse {
	return generated.InitiateMediaUploadResponse{
		AssetId: v.AssetID,
		Upload: generated.UploadAuthorization{
			Method:    generated.UploadAuthorizationMethod(v.Upload.Method),
			Url:       v.Upload.URL,
			ExpiresAt: v.Upload.ExpiresAt,
			Headers:   headersPointer(v.Upload.Headers),
		},
		Constraints: generated.UploadConstraints{
			AllowedContentTypes: v.Constraints.AllowedContentTypes,
			MaxByteSize:         int(v.Constraints.MaxByteSize),
			RequiredHeaders:     v.Constraints.RequiredHeaders,
		},
	}
}

// mapProcessingView projects MEDIA-02/03 output. PublicUrl always stays nil:
// the public URL strategy is undecided and object keys are never exposed.
func mapProcessingView(v appmedia.ProcessingView) generated.MediaProcessingState {
	variants := make([]generated.MediaVariantStatusItem, 0, len(v.Variants))
	for _, item := range v.Variants {
		variants = append(variants, generated.MediaVariantStatusItem{
			TransformProfile: item.TransformProfile,
			LifecycleStatus:  generated.MediaVariantLifecycle(item.LifecycleStatus),
			PublicUrl:        nil,
			Usage:            item.Usage,
		})
	}
	return generated.MediaProcessingState{
		AssetId:         v.AssetID,
		LifecycleStatus: generated.MediaAssetLifecycle(v.LifecycleStatus),
		FailureCode:     v.FailureCode,
		FailureMessage:  v.FailureMessage,
		Variants:        variants,
	}
}

// mapOwnerMediaView projects MEDIA-04..07 output.
func mapOwnerMediaView(v appmedia.OwnerMediaView) generated.AdvertMediaCollectionResponse {
	items := make([]generated.OwnerMediaRelationItem, 0, len(v.Items))
	for _, item := range v.Items {
		items = append(items, generated.OwnerMediaRelationItem{
			AssetId:         item.AssetID,
			DisplayOrder:    item.DisplayOrder,
			IsCover:         item.IsCover,
			LifecycleStatus: generated.MediaAssetLifecycle(item.LifecycleStatus),
		})
	}
	return generated.AdvertMediaCollectionResponse{
		AdvertId:     v.AdvertID,
		MediaVersion: v.MediaVersion,
		Items:        items,
	}
}

// headersPointer returns nil for an empty map so the response omits
// "headers" entirely instead of serializing an empty object.
func headersPointer(h map[string]string) *map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = v
	}
	return &out
}
