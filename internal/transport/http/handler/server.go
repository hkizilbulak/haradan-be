package handler

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"

	appadminuser "github.com/hkizilbulak/haradan-be/internal/application/adminuser"
	appadvert "github.com/hkizilbulak/haradan-be/internal/application/advert"
	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	appbanner "github.com/hkizilbulak/haradan-be/internal/application/banner"
	appcampaign "github.com/hkizilbulak/haradan-be/internal/application/campaign"
	appcatalog "github.com/hkizilbulak/haradan-be/internal/application/catalog"
	appemail "github.com/hkizilbulak/haradan-be/internal/application/email"
	appfavorite "github.com/hkizilbulak/haradan-be/internal/application/favorite"
	appgeo "github.com/hkizilbulak/haradan-be/internal/application/geo"
	apphorse "github.com/hkizilbulak/haradan-be/internal/application/horse"
	appjobadmin "github.com/hkizilbulak/haradan-be/internal/application/jobadmin"
	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	appnotification "github.com/hkizilbulak/haradan-be/internal/application/notification"
	apppackaging "github.com/hkizilbulak/haradan-be/internal/application/packaging"
	apptjk "github.com/hkizilbulak/haradan-be/internal/application/tjk"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	accounthandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/account"
	adminuserhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/adminuser"
	adverthandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/advert"
	authhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/auth"
	bannerhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/banner"
	campaignhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/campaign"
	cataloghandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/catalog"
	emailtplhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/emailtpl"
	favoritehandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/favorite"
	geohandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/geo"
	horsehandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/horse"
	jobadminhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/jobadmin"
	mediahandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/media"
	notificationinboxhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/notificationinbox"
	notificationtplhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/notificationtpl"
	packaginghandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/packaging"
	tjkhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/tjk"
	appcoupon "github.com/hkizilbulak/haradan-be/internal/application/coupon"
	couponhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/coupon"
	appcomment "github.com/hkizilbulak/haradan-be/internal/application/comment"
	commenthandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/comment"
	apppaytr "github.com/hkizilbulak/haradan-be/internal/application/paytr"
	paytrhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/paytr"
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
}

func (s *Server) WithCommentService(svc *appcomment.Service) *Server {
	if svc != nil {
		s.comment = commenthandler.NewHandler(svc, s.logger, respondError)
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

var _ generated.ServerInterface = (*Server)(nil)
