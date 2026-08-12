package coupon

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCalculateDiscount(t *testing.T) {
	cFixed := Coupon{
		DiscountType:  DiscountTypeFixedAmount,
		DiscountValue: 2000, // 20.00 TRY
	}
	if d := cFixed.CalculateDiscount(10000); d != 2000 {
		t.Fatalf("fixed discount = %d, want 2000", d)
	}
	if d := cFixed.CalculateDiscount(1500); d != 1500 {
		t.Fatalf("fixed discount cap = %d, want 1500", d)
	}

	cPerc := Coupon{
		DiscountType:  DiscountTypePercentage,
		DiscountValue: 25, // 25%
	}
	if d := cPerc.CalculateDiscount(10000); d != 2500 {
		t.Fatalf("percentage discount = %d, want 2500", d)
	}
}

func TestValidateForUser(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	startsAt := now.Add(-1 * time.Hour)
	endsAt := now.Add(24 * time.Hour)
	maxUses := 10
	minSpend := int64(5000)
	pkg := "ADVANCED"

	c := Coupon{
		ID:                    uuid.New(),
		Code:                  "TEST20",
		Name:                  "Test Coupon",
		DiscountType:          DiscountTypePercentage,
		DiscountValue:         20,
		MaxUses:               &maxUses,
		UsesCount:             2,
		MaxUsesPerUser:        1,
		MinSpendAmountMinor:   &minSpend,
		ApplicablePackageCode: &pkg,
		StartsAt:              startsAt,
		EndsAt:                &endsAt,
		IsActive:              true,
	}

	// Valid
	ok, msg := c.ValidateForUser(now, 0, 6000, &pkg)
	if !ok || msg != "" {
		t.Fatalf("expected valid, got ok=%v, msg=%s", ok, msg)
	}

	// Inactive
	cInactive := c
	cInactive.IsActive = false
	if ok, _ := cInactive.ValidateForUser(now, 0, 6000, &pkg); ok {
		t.Fatal("expected inactive error")
	}

	// User usage count exceeded
	if ok, _ := c.ValidateForUser(now, 1, 6000, &pkg); ok {
		t.Fatal("expected user usage limit error")
	}

	// Min spend not met
	if ok, _ := c.ValidateForUser(now, 0, 4000, &pkg); ok {
		t.Fatal("expected min spend error")
	}

	// Wrong package
	wrongPkg := "STARTER"
	if ok, _ := c.ValidateForUser(now, 0, 6000, &wrongPkg); ok {
		t.Fatal("expected package mismatch error")
	}
}
