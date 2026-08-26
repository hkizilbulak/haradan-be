package studfarm

import (
	"github.com/google/uuid"

	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	openapi_types "github.com/oapi-codegen/runtime/types"

	domainstudfarm "github.com/hkizilbulak/haradan-be/internal/domain/studfarm"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

type respondErrorFunc func(c *gin.Context, logger *slog.Logger, err error)

// Handler handles stud farm HTTP operations.
type Handler struct {
	svc          domainstudfarm.Service
	logger       *slog.Logger
	respondError respondErrorFunc
}

// NewHandler creates a new stud farm handler.
func NewHandler(svc domainstudfarm.Service, logger *slog.Logger, respondError respondErrorFunc) *Handler {
	return &Handler{
		svc:          svc,
		logger:       logger,
		respondError: respondError,
	}
}

// ListStudFarms implements generated.ServerInterface.
func (h *Handler) ListStudFarms(c *gin.Context, params generated.ListStudFarmsParams) {
	var cursor *string
	if params.Cursor != nil {
		cursor = params.Cursor
	}

	limit := 20
	if params.Limit != nil {
		limit = *params.Limit
	}

	result, err := h.svc.List(c.Request.Context(), cursor, limit)
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	items := make([]generated.StudFarmItem, len(result.Items))
	for i, item := range result.Items {
		items[i] = generated.StudFarmItem{
			Id:                  openapi_types.UUID(item.ID),
			FirstName:           item.FirstName,
			LastName:            item.LastName,
			Email:               item.Email,
			Phone:               item.Phone,
			Location:            item.Location,
			CreatedAt:           item.CreatedAt,
			UpdatedAt:           item.UpdatedAt,
			LatestInterviewDate: item.LatestInterviewDate,
			InterviewerName:     item.InterviewerName,
			InterviewNotesUrl:   item.InterviewNotesURL,
			InterviewCount:      &item.InterviewCount,
		}
	}

	c.JSON(http.StatusOK, generated.StudFarmListResponse{
		Items:      items,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	})
}

// CreateStudFarm implements generated.ServerInterface.
func (h *Handler) CreateStudFarm(c *gin.Context) {
	var req generated.StudFarmCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	param := domainstudfarm.CreateParam{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     string(req.Email),
		Phone:     req.Phone,
		Location:  req.Location,
	}

	sf, err := h.svc.Create(c.Request.Context(), param)
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	item := generated.StudFarmItem{
		Id:        openapi_types.UUID(sf.ID),
		FirstName: sf.FirstName,
		LastName:  sf.LastName,
		Email:     sf.Email,
		Phone:     sf.Phone,
		Location:  sf.Location,
		CreatedAt: sf.CreatedAt,
		UpdatedAt: sf.UpdatedAt,
	}

	c.JSON(http.StatusCreated, item)
}

// DeleteStudFarm implements generated.ServerInterface.
func (h *Handler) DeleteStudFarm(c *gin.Context, studFarmId openapi_types.UUID) {
	err := h.svc.Delete(c.Request.Context(), uuid.UUID(studFarmId))
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// AddStudFarmNote implements generated.ServerInterface.
func (h *Handler) AddStudFarmNote(c *gin.Context, studFarmId openapi_types.UUID) {
	var req generated.StudFarmNoteCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	param := domainstudfarm.NoteCreateParam{
		StudFarmID:      uuid.UUID(studFarmId),
		InterviewerName: req.InterviewerName,
		InterviewDate:   req.InterviewDate,
		Notes:           req.Notes,
	}

	if err := h.svc.AddNote(c.Request.Context(), param); err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	c.Status(http.StatusCreated)
}

// ListStudFarmNotes implements generated.ServerInterface.
func (h *Handler) ListStudFarmNotes(c *gin.Context, studFarmId openapi_types.UUID) {
	notes, err := h.svc.ListNotes(c.Request.Context(), uuid.UUID(studFarmId))
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	res := generated.StudFarmNoteListResponse{
		Items: make([]generated.StudFarmNoteResponse, len(notes)),
	}
	for i, n := range notes {
		res.Items[i] = generated.StudFarmNoteResponse{
			Id:              openapi_types.UUID(n.ID),
			InterviewDate:   n.InterviewDate,
			InterviewerName: n.InterviewerName,
			Notes:           n.Notes,
			CreatedAt:       n.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, res)
}

// DeleteStudFarmNote implements generated.ServerInterface.
func (h *Handler) DeleteStudFarmNote(c *gin.Context, studFarmId openapi_types.UUID, noteId openapi_types.UUID) {
	err := h.svc.DeleteNote(c.Request.Context(), uuid.UUID(studFarmId), uuid.UUID(noteId))
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) UpdateStudFarmNote(c *gin.Context, studFarmId openapi_types.UUID, noteId openapi_types.UUID) {
	ctx := c.Request.Context()
	var req generated.StudFarmNoteCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	param := domainstudfarm.NoteCreateParam{
		InterviewDate:   req.InterviewDate,
		InterviewerName: req.InterviewerName,
		Notes:           req.Notes,
	}

	err := h.svc.UpdateNote(ctx, uuid.UUID(studFarmId), uuid.UUID(noteId), param)
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) UpdateStudFarm(c *gin.Context, id openapi_types.UUID) {
	var req generated.StudFarmCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	param := domainstudfarm.CreateParam{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     string(req.Email),
		Phone:     req.Phone,
		Location:  req.Location,
	}

	err := h.svc.Update(c.Request.Context(), uuid.UUID(id), param)
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	c.Status(http.StatusNoContent)
}
