package coupon

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domain "github.com/hkizilbulak/haradan-be/internal/domain/coupon"
)

type Repository interface {
	CreateCoupon(context.Context, domain.Coupon) error
	GetCouponByID(context.Context, uuid.UUID) (domain.Coupon, error)
	GetCouponByCode(context.Context, string) (domain.Coupon, error)
	ListCoupons(context.Context, *string, *bool, int, int) ([]domain.Coupon, int, error)
	UpdateCoupon(context.Context, domain.Coupon, int, time.Time) (domain.Coupon, error)
	SetActiveStatus(context.Context, uuid.UUID, bool, int, time.Time) (domain.Coupon, error)
	GetUserUsageCount(context.Context, uuid.UUID, uuid.UUID) (int, error)
	RecordUsage(context.Context, domain.CouponUsage, time.Time) error
}

type Config struct {
	Repo Repository
	Now  func() time.Time
}

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(cfg Config) (*Service, error) {
	if cfg.Repo == nil {
		return nil, fmt.Errorf("coupon repository is required")
	}
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{repo: cfg.Repo, now: now}, nil
}

type CreateCouponCommand struct {
	Code                  string
	Name                  string
	DiscountType          string
	DiscountValue         int64
	MaxUses               *int
	MaxUsesPerUser        int
	MinSpendAmountMinor   *int64
	ApplicablePackageCode *string
	StartsAt              time.Time
	EndsAt                *time.Time
	CreatedByUserID       uuid.UUID
}

func (s *Service) Create(ctx context.Context, cmd CreateCouponCommand) (domain.Coupon, error) {
	code := domain.NormalizeCode(cmd.Code)
	if code == "" {
		return domain.Coupon{}, apperr.Validation("Kupon kodu boş olamaz.")
	}
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return domain.Coupon{}, apperr.Validation("Kupon adı boş olamaz.")
	}
	dt, valid := domain.ParseDiscountType(cmd.DiscountType)
	if !valid {
		return domain.Coupon{}, apperr.Validation("Geçersiz indirim türü.")
	}
	if cmd.DiscountValue <= 0 {
		return domain.Coupon{}, apperr.Validation("İndirim değeri sıfırdan büyük olmalıdır.")
	}
	if dt == domain.DiscountTypePercentage && cmd.DiscountValue > 100 {
		return domain.Coupon{}, apperr.Validation("Yüzdesel indirim %100'den fazla olamaz.")
	}
	if cmd.MaxUsesPerUser <= 0 {
		cmd.MaxUsesPerUser = 1
	}
	if !domain.ValidTimeRange(cmd.StartsAt, cmd.EndsAt) {
		return domain.Coupon{}, apperr.Validation("Bitiş tarihi başlangıç tarihinden önce olamaz.")
	}

	now := s.now()
	c := domain.Coupon{
		ID:                    uuid.New(),
		Code:                  code,
		Name:                  name,
		DiscountType:          dt,
		DiscountValue:         cmd.DiscountValue,
		MaxUses:               cmd.MaxUses,
		UsesCount:             0,
		MaxUsesPerUser:        cmd.MaxUsesPerUser,
		MinSpendAmountMinor:   cmd.MinSpendAmountMinor,
		ApplicablePackageCode: cmd.ApplicablePackageCode,
		StartsAt:              cmd.StartsAt,
		EndsAt:                cmd.EndsAt,
		IsActive:              true,
		CreatedByUserID:       cmd.CreatedByUserID,
		Version:               1,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	if err := s.repo.CreateCoupon(ctx, c); err != nil {
		return domain.Coupon{}, err
	}
	return c, nil
}

type UpdateCouponCommand struct {
	ID                    uuid.UUID
	ExpectedVersion       int
	Name                  string
	DiscountType          string
	DiscountValue         int64
	MaxUses               *int
	MaxUsesPerUser        int
	MinSpendAmountMinor   *int64
	ApplicablePackageCode *string
	StartsAt              time.Time
	EndsAt                *time.Time
	IsActive              bool
}

func (s *Service) Update(ctx context.Context, cmd UpdateCouponCommand) (domain.Coupon, error) {
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return domain.Coupon{}, apperr.Validation("Kupon adı boş olamaz.")
	}
	dt, valid := domain.ParseDiscountType(cmd.DiscountType)
	if !valid {
		return domain.Coupon{}, apperr.Validation("Geçersiz indirim türü.")
	}
	if cmd.DiscountValue <= 0 {
		return domain.Coupon{}, apperr.Validation("İndirim değeri sıfırdan büyük olmalıdır.")
	}
	if dt == domain.DiscountTypePercentage && cmd.DiscountValue > 100 {
		return domain.Coupon{}, apperr.Validation("Yüzdesel indirim %100'den fazla olamaz.")
	}
	if cmd.MaxUsesPerUser <= 0 {
		cmd.MaxUsesPerUser = 1
	}
	if !domain.ValidTimeRange(cmd.StartsAt, cmd.EndsAt) {
		return domain.Coupon{}, apperr.Validation("Bitiş tarihi başlangıç tarihinden önce olamaz.")
	}

	existing, err := s.repo.GetCouponByID(ctx, cmd.ID)
	if err != nil {
		return domain.Coupon{}, err
	}

	existing.Name = name
	existing.DiscountType = dt
	existing.DiscountValue = cmd.DiscountValue
	existing.MaxUses = cmd.MaxUses
	existing.MaxUsesPerUser = cmd.MaxUsesPerUser
	existing.MinSpendAmountMinor = cmd.MinSpendAmountMinor
	existing.ApplicablePackageCode = cmd.ApplicablePackageCode
	existing.StartsAt = cmd.StartsAt
	existing.EndsAt = cmd.EndsAt
	existing.IsActive = cmd.IsActive

	return s.repo.UpdateCoupon(ctx, existing, cmd.ExpectedVersion, s.now())
}

func (s *Service) SetActive(ctx context.Context, id uuid.UUID, isActive bool, expectedVersion int) (domain.Coupon, error) {
	return s.repo.SetActiveStatus(ctx, id, isActive, expectedVersion, s.now())
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (domain.Coupon, error) {
	return s.repo.GetCouponByID(ctx, id)
}

func (s *Service) List(ctx context.Context, search *string, isActive *bool, limit, offset int) ([]domain.Coupon, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListCoupons(ctx, search, isActive, limit, offset)
}

type ValidationResult struct {
	Valid               bool          `json:"valid"`
	Message             string        `json:"message,omitempty"`
	Coupon              *domain.Coupon `json:"coupon,omitempty"`
	DiscountAmountMinor int64         `json:"discountAmountMinor"`
	FinalAmountMinor    int64         `json:"finalAmountMinor"`
}

func (s *Service) ValidateCoupon(ctx context.Context, userID uuid.UUID, code string, spendAmountMinor int64, packageCode *string) (ValidationResult, error) {
	normCode := domain.NormalizeCode(code)
	if normCode == "" {
		return ValidationResult{Valid: false, Message: "Kupon kodu giriniz."}, nil
	}

	c, err := s.repo.GetCouponByCode(ctx, normCode)
	if err != nil {
		if e, ok := apperr.As(err); ok && e.Kind == apperr.KindNotFound {
			return ValidationResult{Valid: false, Message: "Geçersiz veya bulunamayan kupon kodu."}, nil
		}
		return ValidationResult{}, err
	}

	userUsageCount, err := s.repo.GetUserUsageCount(ctx, c.ID, userID)
	if err != nil {
		return ValidationResult{}, err
	}

	now := s.now()
	valid, msg := c.ValidateForUser(now, userUsageCount, spendAmountMinor, packageCode)
	if !valid {
		return ValidationResult{Valid: false, Message: msg}, nil
	}

	discount := c.CalculateDiscount(spendAmountMinor)
	finalAmount := spendAmountMinor - discount
	if finalAmount < 0 {
		finalAmount = 0
	}

	return ValidationResult{
		Valid:               true,
		Coupon:              &c,
		DiscountAmountMinor: discount,
		FinalAmountMinor:    finalAmount,
	}, nil
}
