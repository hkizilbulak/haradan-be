package communicationtemplate

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	domain "github.com/hkizilbulak/haradan-be/internal/domain/communication_template"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

type RespondErrorFunc func(c *gin.Context, logger *slog.Logger, err error)

type Handler struct {
	repo         domain.Repository
	logger       *slog.Logger
	respondError RespondErrorFunc
}

func NewHandler(repo domain.Repository, logger *slog.Logger, respondError RespondErrorFunc) *Handler {
	return &Handler{
		repo:         repo,
		logger:       logger,
		respondError: respondError,
	}
}

func (h *Handler) ListCommunicationTemplates(c *gin.Context) {
	templates, err := h.repo.GetAll(c.Request.Context())
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	out := make([]generated.CommunicationTemplate, len(templates))
	for i, t := range templates {
		uid, _ := uuid.Parse(t.ID)
		out[i] = generated.CommunicationTemplate{
			Id:        uid,
			Title:     t.Title,
			Channel:   t.Channel,
			Content:   t.Content,
			Subject:   t.Subject,
			IsDefault: t.IsDefault,
			CreatedAt: t.CreatedAt,
			UpdatedAt: t.UpdatedAt,
		}
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) CreateCommunicationTemplate(c *gin.Context) {
	var req generated.CommunicationTemplateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	p := domain.CreateParams{
		Title:   req.Title,
		Channel: req.Channel,
		Content: req.Content,
		Subject: req.Subject,
	}

	t, err := h.repo.Create(c.Request.Context(), p)
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	uid, _ := uuid.Parse(t.ID)
	out := generated.CommunicationTemplate{
		Id:        uid,
		Title:     t.Title,
		Channel:   t.Channel,
		Content:   t.Content,
		Subject:   t.Subject,
		IsDefault: t.IsDefault,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) UpdateCommunicationTemplate(c *gin.Context, id openapi_types.UUID) {
	var req generated.CommunicationTemplateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	p := domain.UpdateParams{
		Title:   req.Title,
		Channel: req.Channel,
		Content: req.Content,
		Subject: req.Subject,
	}

	t, err := h.repo.Update(c.Request.Context(), id.String(), p)
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	uid, _ := uuid.Parse(t.ID)
	out := generated.CommunicationTemplate{
		Id:        uid,
		Title:     t.Title,
		Channel:   t.Channel,
		Content:   t.Content,
		Subject:   t.Subject,
		IsDefault: t.IsDefault,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) DeleteCommunicationTemplate(c *gin.Context, id openapi_types.UUID) {
	err := h.repo.Delete(c.Request.Context(), id.String())
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}
	c.Status(http.StatusNoContent)
}
