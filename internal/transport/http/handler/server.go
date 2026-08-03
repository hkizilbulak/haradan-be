package handler

import (
	"log/slog"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

// Server is the HTTP transport adapter for OpenAPI operations.
type Server struct {
	NotImplementedServer
	logger *slog.Logger
}

// NewServer constructs the foundation HTTP server implementation.
func NewServer(logger *slog.Logger) *Server {
	return &Server{logger: logger}
}

var _ generated.ServerInterface = (*Server)(nil)
