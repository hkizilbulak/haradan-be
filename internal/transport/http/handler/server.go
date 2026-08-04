package handler

import (
	"context"
	"log/slog"

	appadvert "github.com/hkizilbulak/haradan-be/internal/application/advert"
	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	appcampaign "github.com/hkizilbulak/haradan-be/internal/application/campaign"
	appcatalog "github.com/hkizilbulak/haradan-be/internal/application/catalog"
	appfavorite "github.com/hkizilbulak/haradan-be/internal/application/favorite"
	appgeo "github.com/hkizilbulak/haradan-be/internal/application/geo"
	apphorse "github.com/hkizilbulak/haradan-be/internal/application/horse"
	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	appnotification "github.com/hkizilbulak/haradan-be/internal/application/notification"
	apppackaging "github.com/hkizilbulak/haradan-be/internal/application/packaging"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	accounthandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/account"
	adverthandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/advert"
	authhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/auth"
	campaignhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/campaign"
	cataloghandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/catalog"
	favoritehandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/favorite"
	geohandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/geo"
	horsehandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/horse"
	mediahandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/media"
	notificationtplhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/notificationtpl"
	packaginghandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/packaging"
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
	notification *notificationtplhandler.Handler
	auth         *authhandler.Handler
	account      *accounthandler.Handler
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
	if authSvc != nil {
		s.auth = authhandler.NewHandler(authSvc, logger, respondError)
		s.account = accounthandler.NewHandler(authSvc, logger, respondError)
	}
	return s
}

var _ generated.ServerInterface = (*Server)(nil)
