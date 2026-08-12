package coupon

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/application/authz"
	appcoupon "github.com/hkizilbulak/haradan-be/internal/application/coupon"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domain "github.com/hkizilbulak/haradan-be/internal/domain/coupon"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bind"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authctx"
)

type Service interface {
	Create(context.Context, appcoupon.CreateCouponCommand) (domain.Coupon, error)
	Update(context.Context, appcoupon.UpdateCouponCommand) (domain.Coupon, error)
	SetActive(context.Context, uuid.UUID, bool, int) (domain.Coupon, error)
	GetByID(context.Context, uuid.UUID) (domain.Coupon, error)
	List(context.Context, *string, *bool, int, int) ([]domain.Coupon, int, error)
	ValidateCoupon(context.Context, uuid.UUID, string, int64, *string) (appcoupon.ValidationResult, error)
}

type ErrorResponder func(*gin.Context, *slog.Logger, error)

type Handler struct {
	service Service
	logger  *slog.Logger
	respond ErrorResponder
}

func NewHandler(s Service, l *slog.Logger, r ErrorResponder) *Handler {
	return &Handler{service: s, logger: l, respond: r}
}

func (h *Handler) admin(c *gin.Context) (uuid.UUID, bool) {
	p, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return uuid.Nil, false
	}
	if e := authz.RequireAdminBO(p); e != nil {
		h.respond(c, h.logger, e)
		return uuid.Nil, false
	}
	return p.UserID, true
}

type CouponResponse struct {
	ID                    uuid.UUID  `json:"id"`
	Code                  string     `json:"code"`
	Name                  string     `json:"name"`
	DiscountType          string     `json:"discountType"`
	DiscountValue         int64      `json:"discountValue"`
	MaxUses               *int       `json:"maxUses,omitempty"`
	UsesCount             int        `json:"usesCount"`
	MaxUsesPerUser        int        `json:"maxUsesPerUser"`
	MinSpendAmountMinor   *int64     `json:"minSpendAmountMinor,omitempty"`
	ApplicablePackageCode *string    `json:"applicablePackageCode,omitempty"`
	StartsAt              time.Time  `json:"startsAt"`
	EndsAt                *time.Time `json:"endsAt,omitempty"`
	IsActive              bool       `json:"isActive"`
	CreatedByUserID       uuid.UUID  `json:"createdByUserId"`
	Version               int        `json:"version"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

func mapCouponResponse(c domain.Coupon) CouponResponse {
	return CouponResponse{
		ID:                    c.ID,
		Code:                  c.Code,
		Name:                  c.Name,
		DiscountType:          string(c.DiscountType),
		DiscountValue:         c.DiscountValue,
		MaxUses:               c.MaxUses,
		UsesCount:             c.UsesCount,
		MaxUsesPerUser:        c.MaxUsesPerUser,
		MinSpendAmountMinor:   c.MinSpendAmountMinor,
		ApplicablePackageCode: c.ApplicablePackageCode,
		StartsAt:              c.StartsAt,
		EndsAt:                c.EndsAt,
		IsActive:              c.IsActive,
		CreatedByUserID:       c.CreatedByUserID,
		Version:               c.Version,
		CreatedAt:             c.CreatedAt,
		UpdatedAt:             c.UpdatedAt,
	}
}

type CreateCouponRequest struct {
	Code                  string     `json:"code"`
	Name                  string     `json:"name"`
	DiscountType          string     `json:"discountType"`
	DiscountValue         int64      `json:"discountValue"`
	MaxUses               *int       `json:"maxUses,omitempty"`
	MaxUsesPerUser        int        `json:"maxUsesPerUser"`
	MinSpendAmountMinor   *int64     `json:"minSpendAmountMinor,omitempty"`
	ApplicablePackageCode *string    `json:"applicablePackageCode,omitempty"`
	StartsAt              time.Time  `json:"startsAt"`
	EndsAt                *time.Time `json:"endsAt,omitempty"`
}

func (h *Handler) AdminCreate(c *gin.Context) {
	actorID, ok := h.admin(c)
	if !ok {
		return
	}

	var req CreateCouponRequest
	if !bind.JSONBody(c, &req) {
		return
	}

	cmd := appcoupon.CreateCouponCommand{
		Code:                  req.Code,
		Name:                  req.Name,
		DiscountType:          req.DiscountType,
		DiscountValue:         req.DiscountValue,
		MaxUses:               req.MaxUses,
		MaxUsesPerUser:        req.MaxUsesPerUser,
		MinSpendAmountMinor:   req.MinSpendAmountMinor,
		ApplicablePackageCode: req.ApplicablePackageCode,
		StartsAt:              req.StartsAt,
		EndsAt:                req.EndsAt,
		CreatedByUserID:       actorID,
	}

	res, err := h.service.Create(c.Request.Context(), cmd)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}

	c.JSON(http.StatusCreated, mapCouponResponse(res))
}

type UpdateCouponRequest struct {
	ExpectedVersion       int        `json:"expectedVersion"`
	Name                  string     `json:"name"`
	DiscountType          string     `json:"discountType"`
	DiscountValue         int64      `json:"discountValue"`
	MaxUses               *int       `json:"maxUses,omitempty"`
	MaxUsesPerUser        int        `json:"maxUsesPerUser"`
	MinSpendAmountMinor   *int64     `json:"minSpendAmountMinor,omitempty"`
	ApplicablePackageCode *string    `json:"applicablePackageCode,omitempty"`
	StartsAt              time.Time  `json:"startsAt"`
	EndsAt                *time.Time `json:"endsAt,omitempty"`
	IsActive              bool       `json:"isActive"`
}

func (h *Handler) AdminUpdate(c *gin.Context) {
	if _, ok := h.admin(c); !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.respond(c, h.logger, apperr.Validation("Geçersiz kupon kimliği."))
		return
	}

	var req UpdateCouponRequest
	if !bind.JSONBody(c, &req) {
		return
	}

	cmd := appcoupon.UpdateCouponCommand{
		ID:                    id,
		ExpectedVersion:       req.ExpectedVersion,
		Name:                  req.Name,
		DiscountType:          req.DiscountType,
		DiscountValue:         req.DiscountValue,
		MaxUses:               req.MaxUses,
		MaxUsesPerUser:        req.MaxUsesPerUser,
		MinSpendAmountMinor:   req.MinSpendAmountMinor,
		ApplicablePackageCode: req.ApplicablePackageCode,
		StartsAt:              req.StartsAt,
		EndsAt:                req.EndsAt,
		IsActive:              req.IsActive,
	}

	res, err := h.service.Update(c.Request.Context(), cmd)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, mapCouponResponse(res))
}

type SetActiveRequest struct {
	ExpectedVersion int  `json:"expectedVersion"`
	IsActive        bool `json:"isActive"`
}

func (h *Handler) AdminSetActive(c *gin.Context) {
	if _, ok := h.admin(c); !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.respond(c, h.logger, apperr.Validation("Geçersiz kupon kimliği."))
		return
	}

	var req SetActiveRequest
	if !bind.JSONBody(c, &req) {
		return
	}

	res, err := h.service.SetActive(c.Request.Context(), id, req.IsActive, req.ExpectedVersion)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, mapCouponResponse(res))
}

func (h *Handler) AdminGetByID(c *gin.Context) {
	if _, ok := h.admin(c); !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.respond(c, h.logger, apperr.Validation("Geçersiz kupon kimliği."))
		return
	}

	res, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, mapCouponResponse(res))
}

type ListResponse struct {
	Content []CouponResponse `json:"content"`
	Total   int              `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
}

func (h *Handler) AdminList(c *gin.Context) {
	if _, ok := h.admin(c); !ok {
		return
	}

	var search *string
	if s := strings.TrimSpace(c.Query("search")); s != "" {
		search = &s
	}
	var isActive *bool
	if a := strings.TrimSpace(c.Query("isActive")); a != "" {
		b, err := strconv.ParseBool(a)
		if err == nil {
			isActive = &b
		}
	}

	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))

	list, total, err := h.service.List(c.Request.Context(), search, isActive, limit, offset)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}

	res := make([]CouponResponse, len(list))
	for i, item := range list {
		res[i] = mapCouponResponse(item)
	}

	c.JSON(http.StatusOK, ListResponse{
		Content: res,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	})
}

type ValidateCouponRequest struct {
	Code             string  `json:"code"`
	SpendAmountMinor int64   `json:"spendAmountMinor"`
	PackageCode      *string `json:"packageCode,omitempty"`
}

func (h *Handler) UserValidate(c *gin.Context) {
	p, ok := authctx.PrincipalFromContext(c.Request.Context())
	if !ok {
		h.respond(c, h.logger, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli."))
		return
	}

	var req ValidateCouponRequest
	if !bind.JSONBody(c, &req) {
		return
	}

	res, err := h.service.ValidateCoupon(c.Request.Context(), p.UserID, req.Code, req.SpendAmountMinor, req.PackageCode)
	if err != nil {
		h.respond(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, res)
}
