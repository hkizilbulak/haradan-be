package paytrhandler

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	apppaytr "github.com/hkizilbulak/haradan-be/internal/application/paytr"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bind"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

// ErrorResponder maps domain errors to HTTP.
type ErrorResponder func(*gin.Context, *slog.Logger, error)

// Handler exposes PayTR checkout / notify / status routes.
type Handler struct {
	service *apppaytr.Service
	logger  *slog.Logger
	respond ErrorResponder
}

// NewHandler constructs the PayTR HTTP adapter.
func NewHandler(svc *apppaytr.Service, log *slog.Logger, respond ErrorResponder) *Handler {
	return &Handler{service: svc, logger: log, respond: respond}
}

type checkoutRequest struct {
	PackageCode string `json:"packageCode"`
}

type checkoutResponse struct {
	ChargeID     string `json:"chargeId"`
	MerchantOID  string `json:"merchantOid"`
	IframeToken  string `json:"iframeToken"`
	IframeURL    string `json:"iframeUrl"`
	AmountMinor  int64  `json:"amountMinor"`
	CurrencyCode string `json:"currencyCode"`
	PackageCode  string `json:"packageCode"`
	AdvertID     string `json:"advertId"`
	Status       string `json:"status"`
}

type chargeStatusResponse struct {
	MerchantOID       string  `json:"merchantOid"`
	AdvertID          string  `json:"advertId"`
	PackageCode       string  `json:"packageCode"`
	AmountMinor       int64   `json:"amountMinor"`
	CurrencyCode      string  `json:"currencyCode"`
	Status            string  `json:"status"`
	PaidAt            *string `json:"paidAt,omitempty"`
	AdvertSubmittedAt *string `json:"advertSubmittedAt,omitempty"`
}

// StartCheckout POST /v1/me/adverts/:advertId/paytr/checkout
func (h *Handler) StartCheckout(c *gin.Context) {
	principal, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return
	}
	advertID, err := strconv.ParseInt(c.Param("advertId"), 10, 64)
	if err != nil || advertID <= 0 {
		h.respond(c, h.logger, apperr.Validation("Geçersiz ilan kimliği."))
		return
	}
	var body checkoutRequest
	if !bind.JSONBody(c, &body) {
		return
	}
	code, okCode := domainpackaging.ParsePackageCode(body.PackageCode)
	if !okCode {
		h.respond(c, h.logger, apperr.Validation("Geçersiz paket kodu."))
		return
	}
	res, err := h.service.StartCheckout(c.Request.Context(), apppaytr.CheckoutInput{
		OwnerUserID: principal.UserID,
		AdvertID:    advertID,
		PackageCode: code,
		UserIP:      clientIP(c),
	})
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, checkoutResponse{
		ChargeID:     res.ChargeID.String(),
		MerchantOID:  res.MerchantOID,
		IframeToken:  res.IframeToken,
		IframeURL:    res.IframeURL,
		AmountMinor:  res.AmountMinor,
		CurrencyCode: res.CurrencyCode,
		PackageCode:  res.PackageCode,
		AdvertID:     fmt.Sprintf("%d", res.AdvertID),
		Status:       string(res.Status),
	})
}

// GetChargeStatus GET /v1/me/adverts/:advertId/paytr/charges/:merchantOid
func (h *Handler) GetChargeStatus(c *gin.Context) {
	principal, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return
	}
	advertID, err := strconv.ParseInt(c.Param("advertId"), 10, 64)
	if err != nil || advertID <= 0 {
		h.respond(c, h.logger, apperr.Validation("Geçersiz ilan kimliği."))
		return
	}
	charge, err := h.service.GetChargeForOwner(c.Request.Context(), principal.UserID, c.Param("merchantOid"))
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}
	if charge.AdvertID != advertID {
		h.respond(c, h.logger, apperr.NotFound("Ödeme kaydı bulunamadı."))
		return
	}
	resp := chargeStatusResponse{
		MerchantOID:  charge.MerchantOID,
		AdvertID:     fmt.Sprintf("%d", charge.AdvertID),
		PackageCode:  string(charge.PackageCode),
		AmountMinor:  charge.AmountMinor,
		CurrencyCode: charge.CurrencyCode,
		Status:       string(charge.Status),
	}
	if charge.PaidAt != nil {
		v := charge.PaidAt.UTC().Format("2006-01-02T15:04:05Z")
		resp.PaidAt = &v
	}
	if charge.AdvertSubmittedAt != nil {
		v := charge.AdvertSubmittedAt.UTC().Format("2006-01-02T15:04:05Z")
		resp.AdvertSubmittedAt = &v
	}
	c.JSON(http.StatusOK, resp)
}

// Notify POST /v1/paytr/notify — public PayTR callback (form-urlencoded).
func (h *Handler) Notify(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, "OK")
		return
	}
	values := map[string]string{}
	for k, vs := range c.Request.PostForm {
		if len(vs) > 0 {
			values[k] = vs[0]
		}
	}
	body, err := h.service.HandleNotify(c.Request.Context(), apppaytr.NotifyInput{
		MerchantOID:      c.PostForm("merchant_oid"),
		Status:           c.PostForm("status"),
		TotalAmount:      c.PostForm("total_amount"),
		Hash:             c.PostForm("hash"),
		FailedReasonCode: c.PostForm("failed_reason_code"),
		FailedReasonMsg:  c.PostForm("failed_reason_msg"),
		RawPayloadJSON:   apppaytr.NotifyPayloadMap(values),
	})
	if err != nil && h.logger != nil {
		h.logger.Error("paytr notify side-effect failed", "err", err.Error())
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, body)
}

func clientIP(c *gin.Context) string {
	if xff := strings.TrimSpace(c.GetHeader("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	if xri := strings.TrimSpace(c.GetHeader("X-Real-IP")); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(c.Request.RemoteAddr)
}
