package banner

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/application/authz"
	appbanner "github.com/hkizilbulak/haradan-be/internal/application/banner"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainbanner "github.com/hkizilbulak/haradan-be/internal/domain/banner"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bind"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

type ErrorResponder func(*gin.Context, *slog.Logger, error)
type Handler struct {
	svc        *appbanner.Service
	media      appbanner.MediaReader
	logger     *slog.Logger
	respond    ErrorResponder
	publicBase string
}

func NewHandler(svc *appbanner.Service, media appbanner.MediaReader, logger *slog.Logger, respond ErrorResponder, publicBase string) *Handler {
	return &Handler{svc, media, logger, respond, publicBase}
}
func (h *Handler) ListBannersAdmin(c *gin.Context, p generated.ListBannersAdminParams) {
	id, ok := h.admin(c)
	if !ok {
		return
	}
	f := appbanner.ListFilter{Limit: 20}
	if p.Limit != nil {
		f.Limit = *p.Limit
	}
	if p.Placement != nil {
		x := domainbanner.Placement(*p.Placement)
		f.Placement = &x
	}
	if p.Status != nil {
		x := domainbanner.Status(*p.Status)
		f.Status = &x
	}
	items, e := h.svc.ListBannersAdmin(c, id, f)
	if e != nil {
		h.respond(c, h.logger, e)
		return
	}
	h.adminList(c, items)
}
func (h *Handler) CreateBanner(c *gin.Context) {
	id, ok := h.admin(c)
	if !ok {
		return
	}
	var r generated.CreateBannerRequest
	if !bind.JSONBody(c, &r) {
		return
	}
	out, e := h.svc.CreateBanner(c, appbanner.CreateInput{ActorUserID: id, Placement: domainbanner.Placement(r.Placement), AssetID: r.AssetId, Title: r.Title, AltText: r.AltText, TargetURL: r.TargetUrl, SortOrder: r.SortOrder})
	h.adminResult(c, http.StatusCreated, out, e)
}
func (h *Handler) GetBannerAdminDetail(c *gin.Context, id generated.BannerIdPath) {
	actor, ok := h.admin(c)
	if !ok {
		return
	}
	out, e := h.svc.GetBannerAdminDetail(c, actor, id)
	h.adminResult(c, http.StatusOK, out, e)
}
func (h *Handler) UpdateBanner(c *gin.Context, id generated.BannerIdPath) {
	actor, ok := h.admin(c)
	if !ok {
		return
	}
	var r generated.UpdateBannerRequest
	if !bind.JSONBody(c, &r) {
		return
	}
	out, e := h.svc.UpdateBanner(c, appbanner.UpdateInput{ActorUserID: actor, BannerID: id, ExpectedVersion: r.ExpectedVersion, AssetID: r.AssetId, Title: r.Title, AltText: r.AltText, TargetURL: r.TargetUrl, SortOrder: r.SortOrder})
	h.adminResult(c, http.StatusOK, out, e)
}
func (h *Handler) SetBannerStatus(c *gin.Context, id generated.BannerIdPath) {
	actor, ok := h.admin(c)
	if !ok {
		return
	}
	var r generated.SetBannerStatusRequest
	if !bind.JSONBody(c, &r) {
		return
	}
	out, e := h.svc.SetBannerStatus(c, appbanner.SetStatusInput{ActorUserID: actor, BannerID: id, ExpectedVersion: r.ExpectedVersion, Status: domainbanner.Status(r.Status)})
	h.adminResult(c, http.StatusOK, out, e)
}
func (h *Handler) ReorderBanners(c *gin.Context) {
	actor, ok := h.admin(c)
	if !ok {
		return
	}
	var r generated.ReorderBannersRequest
	if !bind.JSONBody(c, &r) {
		return
	}
	items := make([]appbanner.ReorderItem, 0, len(r.Items))
	for _, i := range r.Items {
		items = append(items, appbanner.ReorderItem{ID: i.Id, ExpectedVersion: i.ExpectedVersion, SortOrder: i.SortOrder})
	}
	if e := h.svc.ReorderBanners(c, actor, domainbanner.Placement(r.Placement), items); e != nil {
		h.respond(c, h.logger, e)
		return
	}
	c.JSON(http.StatusOK, generated.SuccessMessageResponse{Message: "Banner sıralaması güncellendi."})
}
func (h *Handler) ListActiveBannersByPlacement(c *gin.Context, p generated.ListActiveBannersByPlacementParams) {
	items, e := h.svc.ListActiveBannersByPlacement(c, domainbanner.Placement(p.Placement))
	if e != nil {
		h.respond(c, h.logger, e)
		return
	}
	out := make([]generated.ActiveBannerItem, 0, len(items))
	for _, b := range items {
		url, e := h.imageURL(c, b)
		if e != nil {
			h.respond(c, h.logger, e)
			return
		}
		out = append(out, generated.ActiveBannerItem{Id: b.ID, Placement: generated.BannerPlacement(b.Placement), SortOrder: b.SortOrder, ImageUrl: url, Title: b.Title, AltText: b.AltText, TargetUrl: b.TargetURL})
	}
	c.JSON(http.StatusOK, generated.ActiveBannerListResponse{Items: out})
}
func (h *Handler) adminList(c *gin.Context, items []domainbanner.Banner) {
	out := make([]generated.AdminBannerDetailResponse, 0, len(items))
	for _, b := range items {
		v, e := h.adminView(c, b)
		if e != nil {
			h.respond(c, h.logger, e)
			return
		}
		out = append(out, v)
	}
	c.JSON(http.StatusOK, generated.AdminBannerListResponse{Items: out, HasMore: false})
}
func (h *Handler) adminResult(c *gin.Context, status int, b domainbanner.Banner, e error) {
	if e != nil {
		h.respond(c, h.logger, e)
		return
	}
	v, e := h.adminView(c, b)
	if e != nil {
		h.respond(c, h.logger, e)
		return
	}
	c.JSON(status, v)
}
func (h *Handler) adminView(ctx context.Context, b domainbanner.Banner) (generated.AdminBannerDetailResponse, error) {
	a, e := h.media.FindAssetByID(ctx, b.AssetID)
	if e != nil {
		return generated.AdminBannerDetailResponse{}, e
	}
	return generated.AdminBannerDetailResponse{Id: b.ID, Placement: generated.BannerPlacement(b.Placement), Status: generated.BannerStatus(b.Status), AssetId: b.AssetID, SortOrder: b.SortOrder, Version: b.Version, Title: b.Title, AltText: b.AltText, TargetUrl: b.TargetURL, AssetLifecycleStatus: generated.MediaAssetLifecycle(a.LifecycleStatus)}, nil
}
func (h *Handler) imageURL(ctx context.Context, b domainbanner.Banner) (string, error) {
	if strings.TrimSpace(h.publicBase) == "" {
		return "", apperr.DependencyUnavailable("MEDIA_PUBLIC_BASE_URL is required for public banner URLs")
	}
	vs, e := h.media.ListVariantsByAsset(ctx, b.AssetID)
	if e != nil {
		return "", e
	}
	profile := map[domainbanner.Placement]string{domainbanner.PlacementHomepage: domainmedia.ProfileHomepage, domainbanner.PlacementListingDetail: domainmedia.ProfileDetail, domainbanner.PlacementSearch: domainmedia.ProfileSearch}[b.Placement]
	for _, v := range vs {
		if v.TransformProfile == profile && v.LifecycleStatus == domainmedia.VariantReady && v.ObjectKey != nil {
			if u := domainmedia.PublicURL(h.publicBase, *v.ObjectKey); u != "" {
				return u, nil
			}
		}
	}
	return "", apperr.DependencyUnavailable("active banner has no public READY variant")
}
func (h *Handler) admin(c *gin.Context) (uuid.UUID, bool) {
	p, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return uuid.Nil, false
	}
	if e := authz.RequireAdminBO(p); e != nil {
		h.respond(c, h.logger, e)
		return uuid.Nil, false
	}
	return p.UserID, true
}
