package ai

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	appai "github.com/hkizilbulak/haradan-be/internal/application/ai"
)

type Handler struct {
	svc    *appai.Service
	logger *slog.Logger
	errFn  func(*gin.Context, *slog.Logger, error)
}

func NewHandler(svc *appai.Service, logger *slog.Logger, errFn func(*gin.Context, *slog.Logger, error)) *Handler {
	return &Handler{
		svc:    svc,
		logger: logger,
		errFn:  errFn,
	}
}

func (h *Handler) GenerateAdvert(c *gin.Context) {
	var req appai.GenerateAdvertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	resp, err := h.svc.GenerateAdvert(c.Request.Context(), req)
	if err != nil {
		h.errFn(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}
