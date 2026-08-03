package handler

import (
	"context"
	"log/slog"

	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	appcatalog "github.com/hkizilbulak/haradan-be/internal/application/catalog"
	appgeo "github.com/hkizilbulak/haradan-be/internal/application/geo"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	authhandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/auth"
	cataloghandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/catalog"
	geohandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/geo"
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
	auth    *authhandler.Handler
}

// NewServer constructs the HTTP server implementation.
func NewServer(
	logger *slog.Logger,
	deps DependencyChecker,
	geoSvc *appgeo.Service,
	catalogSvc *appcatalog.Service,
	authSvc *appauth.Service,
) *Server {
	s := &Server{logger: logger, deps: deps}
	if geoSvc != nil {
		s.geo = geohandler.NewHandler(geoSvc, logger, respondError)
	}
	if catalogSvc != nil {
		s.catalog = cataloghandler.NewHandler(catalogSvc, logger, respondError)
	}
	if authSvc != nil {
		s.auth = authhandler.NewHandler(authSvc, logger, respondError)
	}
	return s
}

var _ generated.ServerInterface = (*Server)(nil)
