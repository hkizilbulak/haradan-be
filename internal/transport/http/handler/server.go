package handler

import (
	"context"
	"log/slog"

	appadvert "github.com/hkizilbulak/haradan-be/internal/application/advert"
	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	appcatalog "github.com/hkizilbulak/haradan-be/internal/application/catalog"
	appgeo "github.com/hkizilbulak/haradan-be/internal/application/geo"
	apphorse "github.com/hkizilbulak/haradan-be/internal/application/horse"
	appmedia "github.com/hkizilbulak/haradan-be/internal/application/media"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	accounthandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/account"
	adverthandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/advert"
	authhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/auth"
	cataloghandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/catalog"
	geohandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/geo"
	horsehandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/horse"
	mediahandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/media"
)

// DependencyChecker is a minimal health dependency contract.
type DependencyChecker interface {
	Ping(ctx context.Context) error
}

// Server is the HTTP transport adapter for OpenAPI operations.
type Server struct {
	NotImplementedServer
	logger  *slog.Logger
	deps    DependencyChecker
	geo     *geohandler.Handler
	catalog *cataloghandler.Handler
	horse   *horsehandler.Handler
	advert  *adverthandler.Handler
	media   *mediahandler.Handler
	auth    *authhandler.Handler
	account *accounthandler.Handler
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
	if authSvc != nil {
		s.auth = authhandler.NewHandler(authSvc, logger, respondError)
		s.account = accounthandler.NewHandler(authSvc, logger, respondError)
	}
	return s
}

var _ generated.ServerInterface = (*Server)(nil)
