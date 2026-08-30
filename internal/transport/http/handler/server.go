package handler

import (
	openapi_types "github.com/oapi-codegen/runtime/types"
	"net/http"

	"context"
	"log/slog"

	"github.com/gin-gonic/gin"

	appadminuser "github.com/hkizilbulak/haradan-be/internal/application/adminuser"
	appadvert "github.com/hkizilbulak/haradan-be/internal/application/advert"
	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	appbanner "github.com/hkizilbulak/haradan-be/internal/application/banner"
	appcampaign "github.com/hkizilbulak/haradan-be/internal/application/campaign"
	appcatalog "github.com/hkizilbulak/haradan-be/internal/application/catalog"
	appcomment "github.com/hkizilbulak/haradan-be/internal/application/comment"
	appcoupon "github.com/hkizilbulak/haradan-be/internal/application/coupon"
	appemail "github.com/hkizilbulak/haradan-be/internal/application/email"
	appfavorite "github.com/hkizilbulak/haradan-be/internal/application/favorite"
	appgeo "github.com/hkizilbulak/haradan-be/internal/application/geo"
	apphorse "github.com/hkizilbulak/haradan-be/internal/application/horse"
	appjobadmin "github.com/hkizilbulak/haradan-be/internal/application/jobadmin"
	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	appnotification "github.com/hkizilbulak/haradan-be/internal/application/notification"
	apppackaging "github.com/hkizilbulak/haradan-be/internal/application/packaging"
	apppaytr "github.com/hkizilbulak/haradan-be/internal/application/paytr"
	apptjk "github.com/hkizilbulak/haradan-be/internal/application/tjk"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainstudfarm "github.com/hkizilbulak/haradan-be/internal/domain/studfarm"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	accounthandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/account"
	admincommenthandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/admin"
	adminuserhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/adminuser"
	adverthandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/advert"
	authhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/auth"
	bannerhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/banner"
	campaignhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/campaign"
	cataloghandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/catalog"
	commenthandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/comment"
	couponhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/coupon"
	emailtplhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/emailtpl"
	favoritehandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/favorite"
	geohandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/geo"
	horsehandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/horse"
	jobadminhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/jobadmin"
	mediahandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/media"
	notificationinboxhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/notificationinbox"
	notificationtplhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/notificationtpl"
	packaginghandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/packaging"
	paytrhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/paytr"
	studfarmhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/studfarm"
	tjkhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/tjk"
)

// DependencyChecker is a minimal health dependency contract.
type DependencyChecker interface {
	Ping(ctx context.Context) error
}

// Server is the HTTP transport adapter for OpenAPI operations.
type Server struct {
	NotImplementedServer
	logger       *slog.Logger
	deps         DependencyChecker
	geo          *geohandler.Handler
	catalog      *cataloghandler.Handler
	horse        *horsehandler.Handler
	advert       *adverthandler.Handler
	media        *mediahandler.Handler
	favorite     *favoritehandler.Handler
	packaging    *packaginghandler.Handler
	campaign     *campaignhandler.Handler
	banner       *bannerhandler.Handler
	notification *notificationtplhandler.Handler
	emailtpl     *emailtplhandler.Handler
	jobadmin     *jobadminhandler.Handler
	inbox        *notificationinboxhandler.Handler
	auth         *authhandler.Handler
	account      *accounthandler.Handler
	adminuser    *adminuserhandler.Handler
	tjk          *tjkhandler.Handler
	coupon       *couponhandler.Handler
	comment      *commenthandler.Handler
	paytr        *paytrhandler.Handler
	studfarm     *studfarmhandler.Handler
	admincomment *admincommenthandler.CommentHandler
}

func (s *Server) WithCommentService(svc *appcomment.Service) *Server {
	if svc != nil {
		s.comment = commenthandler.NewHandler(svc, s.logger, respondError)
	}
	return s
}

func (s *Server) WithAdminCommentService(svc *appcomment.Service) *Server {
	if svc != nil {
		s.admincomment = admincommenthandler.NewCommentHandler(svc, s.logger, respondError)
	}
	return s
}

func (s *Server) WithCouponService(svc *appcoupon.Service) *Server {
	if svc != nil {
		s.coupon = couponhandler.NewHandler(svc, s.logger, respondError)
	}
	return s
}

func (s *Server) WithPayTRService(svc *apppaytr.Service) *Server {
	if svc != nil {
		s.paytr = paytrhandler.NewHandler(svc, s.logger, respondError)
	}
	return s
}

func (s *Server) WithStudFarmService(svc domainstudfarm.Service) *Server {
	if svc != nil {
		s.studfarm = studfarmhandler.NewHandler(svc, s.logger, respondError)
	}
	return s
}

func (s *Server) RegisterPayTRRoutes(r gin.IRouter) {
	if s.paytr == nil {
		return
	}
	r.POST("/v1/paytr/notify", s.paytr.Notify)
	r.POST("/v1/me/adverts/:advertId/paytr/checkout", s.paytr.StartCheckout)
	r.GET("/v1/me/adverts/:advertId/paytr/charges/:merchantOid", s.paytr.GetChargeStatus)
}

func (s *Server) RegisterCouponRoutes(r gin.IRouter) {
	if s.coupon == nil {
		return
	}
	v1Admin := r.Group("/v1/admin/coupons")
	{
		v1Admin.GET("", s.coupon.AdminList)
		v1Admin.POST("", s.coupon.AdminCreate)
		v1Admin.GET("/:id", s.coupon.AdminGetByID)
		v1Admin.PUT("/:id", s.coupon.AdminUpdate)
		v1Admin.PATCH("/:id/active", s.coupon.AdminSetActive)
	}

	v1Public := r.Group("/v1/coupons")
	{
		v1Public.POST("/validate", s.coupon.UserValidate)
	}
}

func (s *Server) RegisterAdminCommentRoutes(r gin.IRouter) {
	
	if s.admincomment == nil {
		return
	}
	s.admincomment.RegisterRoutes(r)
}

func (s *Server) WithTJKService(svc *apptjk.Service) *Server {
	if svc != nil {
		s.tjk = tjkhandler.NewHandler(svc, s.logger, respondError)
	}
	return s
}

// WithPublicMediaDelivery wires GET|HEAD /v1/media/{assetId}/{profile}.
// The media handler is already constructed in NewServer when mediaSvc is non-nil.
func (s *Server) WithPublicMediaDelivery(svc *appmedia.Service) *Server {
	if svc != nil && s.media == nil {
		s.media = mediahandler.NewHandler(svc, s.logger, respondError)
	}
	return s
}

// GetPublicMedia implements OpenAPI GET /v1/media/{assetId}/{profile}.
func (s *Server) GetPublicMedia(c *gin.Context, assetId generated.AssetIdPath, profile generated.MediaDeliveryProfile) {
	if s.media == nil {
		respondNotImplemented(c)
		return
	}
	s.media.GetPublicMedia(c, assetId, profile)
}

// HeadPublicMedia implements OpenAPI HEAD /v1/media/{assetId}/{profile}.
func (s *Server) HeadPublicMedia(c *gin.Context, assetId generated.AssetIdPath, profile generated.MediaDeliveryProfile) {
	if s.media == nil {
		respondNotImplemented(c)
		return
	}
	s.media.HeadPublicMedia(c, assetId, profile)
}

// WithEmailTemplateDiscovery wires Resend provider template discovery.
func (s *Server) WithEmailTemplateDiscovery(discovery appemail.TemplateDiscovery) *Server {
	if discovery != nil {
		s.emailtpl = emailtplhandler.NewHandler(discovery, s.logger, respondError)
	}
	return s
}

// WithJobAdminService wires BO job definition management.
func (s *Server) WithJobAdminService(svc *appjobadmin.Service) *Server {
	if svc != nil {
		s.jobadmin = jobadminhandler.NewHandler(svc, s.logger, respondError)
	}
	return s
}

func (s *Server) respondDependencyUnavailable(c *gin.Context, message string) {
	respondError(c, s.logger, apperr.DependencyUnavailable(message))
}

// WithAdminUserService activates the ADMIN-USER operations.
func (s *Server) WithAdminUserService(svc *appadminuser.Service) *Server {
	if svc != nil {
		s.adminuser = adminuserhandler.NewHandler(svc, s.logger, respondError)
	}
	return s
}

// WithBannerService wires the banner operations.
func (s *Server) WithBannerService(svc *appbanner.Service, media appbanner.MediaReader) *Server {
	if svc != nil && media != nil {
		s.banner = bannerhandler.NewHandler(svc, media, s.logger, respondError)
	}
	return s
}

// NewServer constructs the HTTP server implementation.
func NewServer(
	logger *slog.Logger,
	deps DependencyChecker,
	geoSvc *appgeo.Service,
	catalogSvc *appcatalog.Service,
	horseSvc *apphorse.Service,
	advertSvc *appadvert.Service,
	mediaSvc *appmedia.Service,
	favoriteSvc *appfavorite.Service,
	packagingSvc *apppackaging.Service,
	campaignSvc *appcampaign.Service,
	campaignPackages appcampaign.PackageLookup,
	notificationSvc *appnotification.Service,
	authSvc *appauth.Service,
	inboxSvcs ...*appnotification.UserNotificationService,
) *Server {
	s := &Server{logger: logger, deps: deps}
	if geoSvc != nil {
		s.geo = geohandler.NewHandler(geoSvc, logger, respondError)
	}
	if catalogSvc != nil {
		s.catalog = cataloghandler.NewHandler(catalogSvc, logger, respondError)
	}
	if horseSvc != nil {
		s.horse = horsehandler.NewHandler(horseSvc, logger, respondError)
	}
	if advertSvc != nil {
		s.advert = adverthandler.NewHandler(advertSvc, logger, respondError)
	}
	if mediaSvc != nil {
		s.media = mediahandler.NewHandler(mediaSvc, logger, respondError)
	}
	if favoriteSvc != nil {
		s.favorite = favoritehandler.NewHandler(favoriteSvc, logger, respondError)
	}
	if packagingSvc != nil {
		s.packaging = packaginghandler.NewHandler(packagingSvc, logger, respondError)
	}
	if campaignSvc != nil && campaignPackages != nil {
		s.campaign = campaignhandler.NewHandler(campaignSvc, campaignPackages, logger, respondError)
	}
	if notificationSvc != nil {
		s.notification = notificationtplhandler.NewHandler(notificationSvc, logger, respondError)
	}
	if len(inboxSvcs) > 0 && inboxSvcs[0] != nil {
		s.inbox = notificationinboxhandler.NewHandler(inboxSvcs[0], logger, respondError)
	}
	if authSvc != nil {
		s.auth = authhandler.NewHandler(authSvc, logger, respondError)
		s.account = accounthandler.NewHandler(authSvc, logger, respondError)
		if s.media != nil {
			s.media.WithAccessAuthenticator(authSvc)
		}
	}
	return s
}

func (s *Server) CreateStudFarm(c *gin.Context) {
	if s.studfarm != nil {
		s.studfarm.CreateStudFarm(c)
	} else {
		c.JSON(501, gin.H{"error": "not implemented"})
	}
}

var _ generated.ServerInterface = (*Server)(nil)

func (s *Server) DeleteStudFarm(c *gin.Context, studFarmId openapi_types.UUID) {
	if s.studfarm != nil {
		s.studfarm.DeleteStudFarm(c, studFarmId)
	} else {
		c.Status(http.StatusNotImplemented)
	}
}

func (s *Server) AddStudFarmNote(c *gin.Context, studFarmId openapi_types.UUID) {
	if s.studfarm != nil {
		s.studfarm.AddStudFarmNote(c, studFarmId)
	} else {
		c.Status(http.StatusNotImplemented)
	}
}

func (s *Server) ListStudFarmNotes(c *gin.Context, studFarmId openapi_types.UUID) {
	if s.studfarm != nil {
		s.studfarm.ListStudFarmNotes(c, studFarmId)
	} else {
		c.Status(http.StatusNotImplemented)
	}
}

func (s *Server) RegisterCatalogDynamicRoutes(rg gin.IRouter) {
	if s.catalog != nil {
		s.catalog.RegisterDynamicRoutes(rg)
	}
}

func (s *Server) DeleteStudFarmNote(c *gin.Context, studFarmId openapi_types.UUID, noteId openapi_types.UUID) {
	if s.studfarm != nil {
		s.studfarm.DeleteStudFarmNote(c, studFarmId, noteId)
	} else {
		c.Status(http.StatusNotImplemented)
	}
}

func (s *Server) UpdateStudFarmNote(c *gin.Context, studFarmId openapi_types.UUID, noteId openapi_types.UUID) {
	s.studfarm.UpdateStudFarmNote(c, studFarmId, noteId)
}

func (s *Server) UpdateStudFarm(c *gin.Context, id openapi_types.UUID) {
	s.studfarm.UpdateStudFarm(c, id)
}
