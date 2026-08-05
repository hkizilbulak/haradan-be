package handler

import (
	"context"
	"log/slog"

	appadminuser "github.com/hkizilbulak/haradan-be/internal/application/adminuser"
	appadvert "github.com/hkizilbulak/haradan-be/internal/application/advert"
	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	appbanner "github.com/hkizilbulak/haradan-be/internal/application/banner"
	appcampaign "github.com/hkizilbulak/haradan-be/internal/application/campaign"
	appcatalog "github.com/hkizilbulak/haradan-be/internal/application/catalog"
	appfavorite "github.com/hkizilbulak/haradan-be/internal/application/favorite"
	appgeo "github.com/hkizilbulak/haradan-be/internal/application/geo"
	apphorse "github.com/hkizilbulak/haradan-be/internal/application/horse"
	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	appnotification "github.com/hkizilbulak/haradan-be/internal/application/notification"
	apppackaging "github.com/hkizilbulak/haradan-be/internal/application/packaging"
	apptjk "github.com/hkizilbulak/haradan-be/internal/application/tjk"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	accounthandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/account"
	adminuserhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/adminuser"
	adverthandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/advert"
	authhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/auth"
	bannerhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/banner"
	campaignhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/campaign"
	cataloghandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/catalog"
	favoritehandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/favorite"
	geohandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/geo"
	horsehandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/horse"
	mediahandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/media"
	notificationinboxhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/notificationinbox"
	notificationtplhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/notificationtpl"
	packaginghandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/packaging"
	tjkhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/tjk"
)

// DependencyChecker is a minimal health dependency contract.
type DependencyChecker interface {
	Ping(ctx context.Context) error
}

// Server is the HTTP transport adapter for OpenAPI operations.
type Server struct {
	NotImplementedServer
	logger             *slog.Logger
	deps               DependencyChecker
	geo                *geohandler.Handler
	catalog            *cataloghandler.Handler
	horse              *horsehandler.Handler
	advert             *adverthandler.Handler
	media              *mediahandler.Handler
	favorite           *favoritehandler.Handler
	packaging          *packaginghandler.Handler
	campaign           *campaignhandler.Handler
	banner             *bannerhandler.Handler
	notification       *notificationtplhandler.Handler
	inbox              *notificationinboxhandler.Handler
	auth               *authhandler.Handler
	account            *accounthandler.Handler
	adminuser          *adminuserhandler.Handler
	tjk                *tjkhandler.Handler
	publicMediaBaseURL string
}

func (s *Server) WithTJKService(svc *apptjk.Service) *Server {
	if svc != nil {
		s.tjk = tjkhandler.NewHandler(svc, s.logger, respondError)
	}
	return s
}

// WithPublicMediaBaseURL sets the configured public media origin used only for
// buyer-facing projections. Object keys remain internal regardless of config.
func (s *Server) WithPublicMediaBaseURL(baseURL string) *Server {
	s.publicMediaBaseURL = baseURL
	return s
}

// WithAdminUserService activates the ADMIN-USER operations.
func (s *Server) WithAdminUserService(svc *appadminuser.Service) *Server {
	if svc != nil {
		s.adminuser = adminuserhandler.NewHandler(svc, s.logger, respondError)
	}
	return s
}

// WithBannerService wires the banner operations after shared public URL config.
func (s *Server) WithBannerService(svc *appbanner.Service, media appbanner.MediaReader, baseURL string) *Server {
	if svc != nil && media != nil {
		s.banner = bannerhandler.NewHandler(svc, media, s.logger, respondError, baseURL)
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
	}
	return s
}

var _ generated.ServerInterface = (*Server)(nil)
