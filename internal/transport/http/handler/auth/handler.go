package auth

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bind"
)

// ErrorResponder maps application errors to HTTP responses.
type ErrorResponder func(c *gin.Context, logger *slog.Logger, err error)

// Handler exposes Auth OpenAPI operations AUTH-01/02/03/04/05/06.
type Handler struct {
	svc     *appauth.Service
	logger  *slog.Logger
	respond ErrorResponder
}

// NewHandler constructs an Auth HTTP handler.
func NewHandler(svc *appauth.Service, logger *slog.Logger, respond ErrorResponder) *Handler {
	return &Handler{svc: svc, logger: logger, respond: respond}
}

// RegisterUser handles POST /v1/auth/register.
func (h *Handler) RegisterUser(c *gin.Context) {
	var body generated.RegisterUserRequest
	if !bind.JSONBody(c, &body) {
		return
	}
	var phone *string
	if body.Phone != nil {
		phone = body.Phone
	}
	out, err := h.svc.Register(c.Request.Context(), appauth.RegisterInput{
		Email:     string(body.Email),
		Password:  body.Password,
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Phone:     phone,
		ClientIP:  c.ClientIP(),
	})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusCreated, generated.GenericAuthMessageResponse{Message: out.Message})
}

// Login handles POST /v1/auth/login.
func (h *Handler) Login(c *gin.Context) {
	var body generated.LoginRequest
	if !bind.JSONBody(c, &body) {
		return
	}
	out, err := h.svc.Login(c.Request.Context(), appauth.LoginInput{
		Email:         string(body.Email),
		Password:      body.Password,
		ClientContext: domainauth.ClientContext(body.ClientContext),
		UserAgent:     c.Request.UserAgent(),
		ClientIP:      c.ClientIP(),
	})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapToken(out))
}

// RefreshSession handles POST /v1/auth/refresh.
func (h *Handler) RefreshSession(c *gin.Context) {
	var body generated.RefreshSessionRequest
	if !bind.JSONBody(c, &body) {
		return
	}
	out, err := h.svc.Refresh(c.Request.Context(), appauth.RefreshInput{
		RefreshToken:  body.RefreshToken,
		ClientContext: domainauth.ClientContext(body.ClientContext),
		UserAgent:     c.Request.UserAgent(),
		ClientIP:      c.ClientIP(),
	})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, mapToken(out))
}

// LogoutCurrentSession handles POST /v1/auth/logout.
func (h *Handler) LogoutCurrentSession(c *gin.Context) {
	tok, err := bind.BearerToken(c)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	out, err := h.svc.Logout(c.Request.Context(), appauth.LogoutInput{AccessToken: tok})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.GenericAuthMessageResponse{Message: out.Message})
}

// VerifyRegistrationEmail handles POST /v1/auth/verify-email (AUTH-02).
func (h *Handler) VerifyRegistrationEmail(c *gin.Context) {
	var body generated.TokenRequest
	if !bind.JSONBody(c, &body) {
		return
	}
	out, err := h.svc.VerifyEmail(c.Request.Context(), appauth.VerifyEmailInput{Token: body.Token})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.GenericAuthMessageResponse{Message: out.Message})
}

// ResendRegistrationEmailVerification handles POST /v1/auth/resend-verification (AUTH-03).
func (h *Handler) ResendRegistrationEmailVerification(c *gin.Context) {
	var body generated.EmailRequest
	if !bind.JSONBody(c, &body) {
		return
	}
	out, err := h.svc.ResendVerification(c.Request.Context(), appauth.ResendVerificationInput{
		Email:    string(body.Email),
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.GenericAuthMessageResponse{Message: out.Message})
}

func (h *Handler) RequestPasswordReset(c *gin.Context) {
	var body generated.EmailRequest
	if !bind.JSONBody(c, &body) {
		return
	}
	out, err := h.svc.RequestPasswordReset(c.Request.Context(), appauth.RequestPasswordResetInput{Email: string(body.Email), ClientIP: c.ClientIP()})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.GenericAuthMessageResponse{Message: out.Message})
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var body generated.ResetPasswordRequest
	if !bind.JSONBody(c, &body) {
		return
	}
	out, err := h.svc.ResetPassword(c.Request.Context(), appauth.ResetPasswordInput{Token: body.Token, NewPassword: body.NewPassword})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.GenericAuthMessageResponse{Message: out.Message})
}

func (h *Handler) ConfirmEmailChange(c *gin.Context) {
	var body generated.TokenRequest
	if !bind.JSONBody(c, &body) {
		return
	}
	out, err := h.svc.ConfirmEmailChange(c.Request.Context(), appauth.ConfirmEmailChangeInput{Token: body.Token})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, generated.GenericAuthMessageResponse{Message: out.Message})
}

func mapToken(out appauth.TokenResult) generated.AuthTokenResponse {
	ctx := generated.ClientContext(out.ClientContext)
	return generated.AuthTokenResponse{
		AccessToken:   out.AccessToken,
		RefreshToken:  out.RefreshToken,
		TokenType:     generated.Bearer,
		ExpiresIn:     out.ExpiresIn,
		ClientContext: &ctx,
	}
}

// ClientIPHint returns a stable non-secret string for tests.
func ClientIPHint(c *gin.Context) string {
	return strings.TrimSpace(c.ClientIP())
}
