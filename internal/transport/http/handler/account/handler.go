package account

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

// ErrorResponder maps application errors to HTTP responses.
type ErrorResponder func(c *gin.Context, logger *slog.Logger, err error)

// Handler exposes ACCOUNT-01/02 and AUTH-07/08/09.
type Handler struct {
	svc     *appauth.Service
	logger  *slog.Logger
	respond ErrorResponder
}

// NewHandler constructs an account/session HTTP handler.
func NewHandler(svc *appauth.Service, logger *slog.Logger, respond ErrorResponder) *Handler {
	return &Handler{svc: svc, logger: logger, respond: respond}
}

// GetMyProfile handles GET /v1/me.
func (h *Handler) GetMyProfile(c *gin.Context) {
	principal, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return
	}
	out, err := h.svc.GetMyProfile(c.Request.Context(), principal.UserID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapProfile(out))
}

// UpdateMyProfile handles PATCH /v1/me.
func (h *Handler) UpdateMyProfile(c *gin.Context) {
	principal, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return
	}
	patch, err := decodeProfilePatch(c)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	out, err := h.svc.UpdateMyProfile(c.Request.Context(), principal.UserID, patch)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapProfile(out))
}

// LogoutAllSessions handles POST /v1/auth/logout-all.
func (h *Handler) LogoutAllSessions(c *gin.Context) {
	principal, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return
	}
	out, err := h.svc.LogoutAllSessions(c.Request.Context(), principal)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.GenericAuthMessageResponse{Message: out.Message})
}

// ListMySessions handles GET /v1/me/sessions.
func (h *Handler) ListMySessions(c *gin.Context, params generated.ListMySessionsParams) {
	principal, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return
	}
	var limit *int
	if params.Limit != nil {
		v := int(*params.Limit)
		limit = &v
	}
	out, err := h.svc.ListMySessions(c.Request.Context(), principal.UserID, principal.SessionID, params.Cursor, limit)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	items := make([]generated.SessionListItem, 0, len(out.Items))
	for _, item := range out.Items {
		ctx := generated.ClientContext(item.ClientContext)
		items = append(items, generated.SessionListItem{
			Id:            item.ID,
			ClientContext: ctx,
			CreatedAt:     item.CreatedAt,
			LastUsedAt:    item.LastUsedAt,
			IsCurrent:     item.IsCurrent,
			RevokedAt:     item.RevokedAt,
		})
	}
	c.JSON(http.StatusOK, generated.SessionListResponse{
		Items:      items,
		HasMore:    out.HasMore,
		NextCursor: out.NextCursor,
	})
}

// RevokeMySession handles DELETE /v1/me/sessions/{sessionId}.
func (h *Handler) RevokeMySession(c *gin.Context, sessionID uuid.UUID) {
	principal, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return
	}
	out, err := h.svc.RevokeMySession(c.Request.Context(), principal, sessionID)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.GenericAuthMessageResponse{Message: out.Message})
}

func mapProfile(out appauth.ProfileView) generated.MyProfileResponse {
	return generated.MyProfileResponse{
		Id:            out.ID,
		Email:         out.Email,
		EmailVerified: out.EmailVerified,
		FirstName:     out.FirstName,
		LastName:      out.LastName,
		Phone:         out.Phone,
		Role:          generated.UserRole(out.Role),
		Status:        generated.UserStatus(out.Status),
	}
}

func decodeProfilePatch(c *gin.Context) (appauth.ProfilePatch, error) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return appauth.ProfilePatch{}, apperr.BadRequest(apperr.CodeValidation, "İstek gövdesi geçersiz.")
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return appauth.ProfilePatch{}, apperr.BadRequest(apperr.CodeValidation, "İstek gövdesi geçersiz.")
	}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return appauth.ProfilePatch{}, apperr.BadRequest(apperr.CodeValidation, "İstek gövdesi geçersiz.")
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF && err != nil {
		return appauth.ProfilePatch{}, apperr.BadRequest(apperr.CodeValidation, "İstek gövdesi geçersiz.")
	}
	var patch appauth.ProfilePatch
	if v, ok := raw["firstName"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return appauth.ProfilePatch{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "firstName", Message: "Ad geçersiz."})
		}
		patch.FirstName = &s
	}
	if v, ok := raw["lastName"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return appauth.ProfilePatch{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "lastName", Message: "Soyad geçersiz."})
		}
		patch.LastName = &s
	}
	if v, ok := raw["phone"]; ok {
		patch.PhoneSet = true
		if string(v) != "null" {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return appauth.ProfilePatch{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "phone", Message: "Telefon geçersiz."})
			}
			patch.PhoneValue = &s
		}
	}
	return patch, nil
}
