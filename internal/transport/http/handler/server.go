package handler

import (
	"context"
	"log/slog"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

// DependencyChecker is a minimal health dependency contract.
type DependencyChecker interface {
	Ping(ctx context.Context) error
}

// Server is the HTTP transport adapter for OpenAPI operations.
type Server struct {
	NotImplementedServer
	logger *slog.Logger
	deps   DependencyChecker
}

// NewServer constructs the foundation HTTP server implementation.
func NewServer(logger *slog.Logger, deps DependencyChecker) *Server {
	return &Server{logger: logger, deps: deps}
}

var _ generated.ServerInterface = (*Server)(nil)
