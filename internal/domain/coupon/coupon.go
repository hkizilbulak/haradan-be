// Package coupon defines domain logic and validation for promotion coupons aligned with migration 00017.
package coupon

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type DiscountType string

const (
	DiscountTypePercentage  DiscountType = "PERCENTAGE"
	DiscountTypeFixedAmount DiscountType = "FIXED_AMOUNT"
)

func (t DiscountType) Valid() bool {
	return t == DiscountTypePercentage || t == DiscountTypeFixedAmount
}

func ParseDiscountType(v string) (DiscountType, bool) {
	t := DiscountType(strings.ToUpper(strings.TrimSpace(v)))
	return t, t.Valid()
}

type Coupon struct {
	ID                    uuid.UUID
	Code                  string
	Name                  string
	DiscountType          DiscountType
	DiscountValue         int64
	MaxUses               *int
	UsesCount             int
	MaxUsesPerUser        int
	MinSpendAmountMinor   *int64
	ApplicablePackageCode *string
	StartsAt              time.Time
	EndsAt                *time.Time
	IsActive              bool
	CreatedByUserID       uuid.UUID
	Version               int
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type CouponUsage struct {
	ID                  uuid.UUID
	CouponID            uuid.UUID
	UserID              uuid.UUID
	AdvertID            *uuid.UUID
	DiscountAmountMinor int64
	UsedAt              time.Time
	CreatedAt           time.Time
}

func NormalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func ValidTimeRange(startsAt time.Time, endsAt *time.Time) bool {
	if endsAt == nil {
		return true
	}
	return !endsAt.Before(startsAt)
}

func (c *Coupon) CalculateDiscount(spendAmountMinor int64) int64 {
	if spendAmountMinor <= 0 || c.DiscountValue <= 0 {
		return 0
	}
	if c.DiscountType == DiscountTypeFixedAmount {
		if c.DiscountValue >= spendAmountMinor {
			return spendAmountMinor
		}
		return c.DiscountValue
	}
	if c.DiscountType == DiscountTypePercentage {
		// Percentage value: e.g. 20 means 20%
		discount := (spendAmountMinor * c.DiscountValue) / 100
		if discount >= spendAmountMinor {
			return spendAmountMinor
		}
		return discount
	}
	return 0
}

func (c *Coupon) ValidateForUser(now time.Time, userUsageCount int, spendAmountMinor int64, packageCode *string) (bool, string) {
	if !c.IsActive {
		return false, "Kupon şu anda aktif değil."
	}
	if now.Before(c.StartsAt) {
		return false, "Kupon henüz kullanıma açılmadı."
	}
	if c.EndsAt != nil && now.After(*c.EndsAt) {
		return false, "Kuponun kullanım süresi dolmuş."
	}
	if c.MaxUses != nil && c.UsesCount >= *c.MaxUses {
		return false, "Kupon toplam kullanım limitine ulaştı."
	}
	if userUsageCount >= c.MaxUsesPerUser {
		return false, "Bu kuponu tekrar kullanamazsınız."
	}
	if c.MinSpendAmountMinor != nil && spendAmountMinor < *c.MinSpendAmountMinor {
		return false, "Kupon için gereken minimum sepet tutarı sağlanmadı."
	}
	if c.ApplicablePackageCode != nil && strings.TrimSpace(*c.ApplicablePackageCode) != "" {
		if packageCode == nil || !strings.EqualFold(strings.TrimSpace(*packageCode), strings.TrimSpace(*c.ApplicablePackageCode)) {
			return false, "Kupon bu paket türünde geçerli değil."
		}
	}
	return true, ""
}
