// Package adminuser exposes ADMIN-USER HTTP operations.
package adminuser

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	appadminuser "github.com/hkizilbulak/haradan-be/internal/application/adminuser"
	"github.com/hkizilbulak/haradan-be/internal/application/authz"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bind"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

type ErrorResponder func(*gin.Context, *slog.Logger, error)

type Handler struct {
	svc     *appadminuser.Service
	logger  *slog.Logger
	respond ErrorResponder
}

func NewHandler(svc *appadminuser.Service, logger *slog.Logger, respond ErrorResponder) *Handler {
	return &Handler{svc: svc, logger: logger, respond: respond}
}

func (h *Handler) requireAdminBO(c *gin.Context) (uuid.UUID, bool) {
	principal, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return uuid.Nil, false
	}
	if err := authz.RequireAdminBO(principal); err != nil {
		h.respond(c, h.logger, err)
		return uuid.Nil, false
	}
	return principal.UserID, true
}

func (h *Handler) ListUsers(c *gin.Context, params generated.ListUsersParams) {
	if _, ok := h.requireAdminBO(c); !ok {
		return
	}
	var status, role *string
	if params.Status != nil {
		v := string(*params.Status)
		status = &v
	}
	if params.Role != nil {
		v := string(*params.Role)
		role = &v
	}
	out, err := h.svc.ListUsers(c.Request.Context(), appadminuser.ListInput{
		Cursor: params.Cursor, Limit: params.Limit, Status: status, Role: role, Query: params.Q,
	})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	items := make([]generated.AdminUserListItem, 0, len(out.Items))
	for _, user := range out.Items {
		items = append(items, mapListItem(user))
	}
	c.JSON(http.StatusOK, generated.AdminUserListResponse{Items: items, NextCursor: out.NextCursor, HasMore: out.HasMore})
}

func (h *Handler) GetUserAdminDetail(c *gin.Context, userID generated.UserIdPath) {
	if _, ok := h.requireAdminBO(c); !ok {
		return
	}
	out, err := h.svc.GetDetail(c.Request.Context(), uuid.UUID(userID))
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapDetail(out))
}

func (h *Handler) ChangeUserRole(c *gin.Context, userID generated.UserIdPath) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	var req generated.ChangeUserRoleRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	out, err := h.svc.ChangeRole(c.Request.Context(), actorID, uuid.UUID(userID),
		toRole(req.ExpectedCurrentRole), toRole(req.NewRole))
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapDetail(out))
}

func (h *Handler) ChangeUserStatus(c *gin.Context, userID generated.UserIdPath) {
	actorID, ok := h.requireAdminBO(c)
	if !ok {
		return
	}
	var req generated.ChangeUserStatusRequest
	if !bind.JSONBody(c, &req) {
		return
	}
	out, err := h.svc.ChangeStatus(c.Request.Context(), actorID, uuid.UUID(userID),
		toStatus(req.ExpectedCurrentStatus), toStatus(req.NewStatus))
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapDetail(out))
}

func (h *Handler) ListUserSecurityEvents(c *gin.Context, userID generated.UserIdPath, params generated.ListUserSecurityEventsParams) {
	if _, ok := h.requireAdminBO(c); !ok {
		return
	}
	var eventType *string
	if params.EventType != nil {
		v := string(*params.EventType)
		eventType = &v
	}
	out, err := h.svc.ListSecurityEvents(c.Request.Context(), uuid.UUID(userID),
		appadminuser.EventListInput{Cursor: params.Cursor, Limit: params.Limit, EventType: eventType})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	items := make([]generated.SecurityEventItem, 0, len(out.Items))
	for _, event := range out.Items {
		items = append(items, mapEvent(event))
	}
	c.JSON(http.StatusOK, generated.SecurityEventListResponse{Items: items, NextCursor: out.NextCursor, HasMore: out.HasMore})
}

func mapListItem(user domainuser.User) generated.AdminUserListItem {
	return generated.AdminUserListItem{
		Id: openapi_types.UUID(user.ID), Email: user.Email, FirstName: user.FirstName, LastName: user.LastName,
		Role: generated.UserRole(user.Role), Status: generated.UserStatus(user.Status),
		EmailVerified: user.EmailVerifiedAt != nil, CreatedAt: user.CreatedAt,
	}
}

func mapDetail(detail appadminuser.Detail) generated.AdminUserDetailResponse {
	user := detail.User
	return generated.AdminUserDetailResponse{
		Id: openapi_types.UUID(user.ID), Email: user.Email, FirstName: user.FirstName, LastName: user.LastName,
		Phone: user.Phone, Role: generated.UserRole(user.Role), Status: generated.UserStatus(user.Status),
		EmailVerified: user.EmailVerifiedAt != nil, CreatedAt: user.CreatedAt, ActiveSessionCount: detail.ActiveSessionCount,
	}
}

func mapEvent(event domainauth.SecurityEvent) generated.SecurityEventItem {
	var contextValue *generated.ClientContext
	if event.ClientContext != nil {
		v := generated.ClientContext(*event.ClientContext)
		contextValue = &v
	}
	var metadata *map[string]interface{}
	if event.Metadata != nil {
		v := event.Metadata
		metadata = &v
	}
	return generated.SecurityEventItem{
		Id: openapi_types.UUID(event.ID), EventType: generated.SecurityEventType(event.EventType),
		CreatedAt: event.CreatedAt, ClientContext: contextValue, Metadata: metadata,
	}
}

func toRole(value generated.UserRole) domainuser.Role       { return domainuser.Role(value) }
func toStatus(value generated.UserStatus) domainuser.Status { return domainuser.Status(value) }
