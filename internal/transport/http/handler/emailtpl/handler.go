// Package emailtpl exposes BO provider email template discovery OpenAPI ops.
package emailtpl

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/application/authz"
	appemail "github.com/hkizilbulak/haradan-be/internal/application/email"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

// ErrorResponder maps application errors to HTTP responses.
type ErrorResponder func(c *gin.Context, logger *slog.Logger, err error)

// Handler exposes provider email template discovery operations.
type Handler struct {
	discovery appemail.TemplateDiscovery
	logger    *slog.Logger
	respond   ErrorResponder
}

// NewHandler constructs an email template discovery HTTP handler.
func NewHandler(discovery appemail.TemplateDiscovery, logger *slog.Logger, respond ErrorResponder) *Handler {
	return &Handler{discovery: discovery, logger: logger, respond: respond}
}

// ListAdminProviderEmailTemplates handles GET /v1/admin/email-templates/provider.
func (h *Handler) ListAdminProviderEmailTemplates(c *gin.Context) {
	if _, ok := h.requireAdminBO(c); !ok {
		return
	}
	if h.discovery == nil {
		h.respond(c, h.logger, apperr.DependencyUnavailable("E-posta sağlayıcı yapılandırılmamış."))
		return
	}
	items, err := h.discovery.ListTemplates(c.Request.Context())
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	out := make([]generated.ProviderEmailTemplateSummary, 0, len(items))
	for _, item := range items {
		summary := generated.ProviderEmailTemplateSummary{
			Id:   item.ID,
			Name: item.Name,
		}
		if item.Status != "" {
			status := item.Status
			summary.Status = &status
		}
		if item.Alias != "" {
			alias := item.Alias
			summary.Alias = &alias
		}
		out = append(out, summary)
	}
	c.JSON(http.StatusOK, generated.ProviderEmailTemplateListResponse{Items: out})
}

// GetAdminProviderEmailTemplateVariables handles GET /v1/admin/email-templates/provider/{templateId}/variables.
func (h *Handler) GetAdminProviderEmailTemplateVariables(
	c *gin.Context,
	templateID generated.ProviderEmailTemplateIdPath,
) {
	if _, ok := h.requireAdminBO(c); !ok {
		return
	}
	if h.discovery == nil {
		h.respond(c, h.logger, apperr.DependencyUnavailable("E-posta sağlayıcı yapılandırılmamış."))
		return
	}
	vars, err := h.discovery.GetTemplateVariables(c.Request.Context(), templateID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	if vars == nil {
		vars = []string{}
	}
	c.JSON(http.StatusOK, generated.ProviderEmailTemplateVariablesResponse{Variables: vars})
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
