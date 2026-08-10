package campaign

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/application/authz"
	appcampaign "github.com/hkizilbulak/haradan-be/internal/application/campaign"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincampaign "github.com/hkizilbulak/haradan-be/internal/domain/campaign"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bind"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

// ErrorResponder maps application errors to HTTP responses.
type ErrorResponder func(c *gin.Context, logger *slog.Logger, err error)

// Handler exposes campaign admin OpenAPI operations.
type Handler struct {
	svc      *appcampaign.Service
	packages appcampaign.PackageLookup
	logger   *slog.Logger
	respond  ErrorResponder
}

// NewHandler constructs a campaign HTTP handler.
func NewHandler(
	svc *appcampaign.Service,
	packages appcampaign.PackageLookup,
	logger *slog.Logger,
	respond ErrorResponder,
) *Handler {
	return &Handler{svc: svc, packages: packages, logger: logger, respond: respond}
}

// ListAdminCampaigns handles GET /v1/admin/campaigns.
func (h *Handler) ListAdminCampaigns(c *gin.Context, params generated.ListAdminCampaignsParams) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	in := appcampaign.ListCampaignsInput{
		Cursor:   params.Cursor,
		Limit:    params.Limit,
		IsActive: params.IsActive,
	}
	if params.EventType != nil {
		et := domaincampaign.CampaignEventType(*params.EventType)
		in.EventType = &et
	}
	if params.SourcePackageCode != nil {
		s := string(*params.SourcePackageCode)
		in.SourcePackageCode = &s
	}
	if params.TargetPackageCode != nil {
		s := string(*params.TargetPackageCode)
		in.TargetPackageCode = &s
	}
	out, err := h.svc.ListCampaigns(c.Request.Context(), actorID, in)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	items := make([]generated.CampaignAdminView, 0, len(out.Items))
	for _, item := range out.Items {
		view, err := h.mapCampaign(c.Request.Context(), item)
		if err != nil {
			h.respond(c, h.logger, err)
			return
		}
		items = append(items, view)
	}
	c.JSON(http.StatusOK, generated.CampaignPage{
		Items:      items,
		NextCursor: out.NextCursor,
		HasMore:    out.HasMore,
	})
}

// CreateAdminCampaign handles POST /v1/admin/campaigns.
func (h *Handler) CreateAdminCampaign(c *gin.Context) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	var req generated.CreateCampaignRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	currency := "TRY"
	if req.CurrencyCode != nil {
		currency = *req.CurrencyCode
	}
	in := appcampaign.CreateCampaignInput{
		ActorUserID:  actorID,
		Name:         req.Name,
		EventType:    domaincampaign.CampaignEventType(req.EventType),
		Title:        req.Title,
		Description:  req.Description,
		EmailSubject: req.EmailSubject,
		EmailHeading: req.EmailHeading,
		EmailBody:    req.EmailBody,
		EmailProviderTemplateID: req.EmailProviderTemplateId,
		CTALabel:     req.CtaLabel,
		CTAURL:       req.CtaUrl,
		BadgeText:    req.BadgeText,
		ImageAssetID: req.ImageAssetId,
		CurrencyCode: currency,
		StartsAt:     req.StartsAt,
		EndsAt:       req.EndsAt,
		IsActive:     isActive,
	}
	if req.Code != nil {
		in.Code = strings.TrimSpace(*req.Code)
	}
	if req.SourcePackageCode != nil {
		s := string(*req.SourcePackageCode)
		in.SourcePackageCode = &s
	}
	if req.TargetPackageCode != nil {
		s := string(*req.TargetPackageCode)
		in.TargetPackageCode = &s
	}
	if req.OriginalPrice != nil {
		amount := int64(req.OriginalPrice.AmountMinor)
		in.DisplayOriginalPriceAmountMinor = &amount
		if req.CurrencyCode == nil {
			in.CurrencyCode = req.OriginalPrice.Currency
		}
	}
	if req.CampaignPrice != nil {
		amount := int64(req.CampaignPrice.AmountMinor)
		in.DisplayCampaignPriceAmountMinor = &amount
		if req.CurrencyCode == nil && req.OriginalPrice == nil {
			in.CurrencyCode = req.CampaignPrice.Currency
		}
	}
	out, err := h.svc.CreateCampaign(c.Request.Context(), in)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	view, err := h.mapCampaign(c.Request.Context(), out)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusCreated, view)
}

// GetAdminCampaign handles GET /v1/admin/campaigns/{campaignId}.
func (h *Handler) GetAdminCampaign(c *gin.Context, campaignID generated.CampaignIdPath) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	out, err := h.svc.GetCampaign(c.Request.Context(), actorID, campaignID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	view, err := h.mapCampaign(c.Request.Context(), out)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

// UpdateAdminCampaign handles PATCH /v1/admin/campaigns/{campaignId}.
func (h *Handler) UpdateAdminCampaign(c *gin.Context, campaignID generated.CampaignIdPath) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	in, err := decodeUpdateCampaignInput(c, actorID, campaignID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	out, err := h.svc.UpdateCampaign(c.Request.Context(), in)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	view, err := h.mapCampaign(c.Request.Context(), out)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

func (h *Handler) mapCampaign(ctx context.Context, camp domaincampaign.Campaign) (generated.CampaignAdminView, error) {
	view := generated.CampaignAdminView{
		Id:            camp.ID,
		Code:          camp.Code,
		Name:          camp.Name,
		EventType:     generated.CampaignEventType(camp.EventType),
		Title:         camp.Title,
		Description:   camp.Description,
		EmailSubject:  camp.EmailSubject,
		EmailHeading:  camp.EmailHeading,
		EmailBody:     camp.EmailBody,
		EmailProviderTemplateId: camp.EmailProviderTemplateID,
		CtaLabel:      camp.CTALabel,
		CtaUrl:        camp.CTAURL,
		BadgeText:     camp.BadgeText,
		ImageAssetId:  camp.ImageAssetID,
		OriginalPrice: moneyFromAmount(camp.DisplayOriginalPriceAmountMinor, camp.CurrencyCode),
		CampaignPrice: moneyFromAmount(camp.DisplayCampaignPriceAmountMinor, camp.CurrencyCode),
		CurrencyCode:  camp.CurrencyCode,
		StartsAt:      camp.StartsAt,
		EndsAt:        camp.EndsAt,
		IsActive:      camp.IsActive,
		Version:       camp.Version,
		CreatedAt:     camp.CreatedAt,
		UpdatedAt:     camp.UpdatedAt,
	}
	if camp.SourcePackageID != nil {
		pkg, err := h.packages.FindByID(ctx, *camp.SourcePackageID)
		if err != nil {
			return generated.CampaignAdminView{}, err
		}
		code := generated.PackageCode(pkg.Code)
		view.SourcePackageCode = &code
	}
	if camp.TargetPackageID != nil {
		pkg, err := h.packages.FindByID(ctx, *camp.TargetPackageID)
		if err != nil {
			return generated.CampaignAdminView{}, err
		}
		code := generated.PackageCode(pkg.Code)
		view.TargetPackageCode = &code
	}
	return view, nil
}

func moneyFromAmount(amount *int64, currency string) *generated.Money {
	if amount == nil {
		return nil
	}
	return &generated.Money{AmountMinor: int(*amount), Currency: currency}
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
