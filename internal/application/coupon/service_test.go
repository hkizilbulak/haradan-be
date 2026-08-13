package coupon

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	domain "github.com/hkizilbulak/haradan-be/internal/domain/coupon"
)

type mockRepo struct {
	coupons map[uuid.UUID]domain.Coupon
	byCode  map[string]domain.Coupon
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		coupons: make(map[uuid.UUID]domain.Coupon),
		byCode:  make(map[string]domain.Coupon),
	}
}

func (m *mockRepo) CreateCoupon(ctx context.Context, c domain.Coupon) error {
	m.coupons[c.ID] = c
	m.byCode[c.Code] = c
	return nil
}

func (m *mockRepo) GetCouponByID(ctx context.Context, id uuid.UUID) (domain.Coupon, error) {
	if c, ok := m.coupons[id]; ok {
		return c, nil
	}
	return domain.Coupon{}, domainErrNotFound()
}

func (m *mockRepo) GetCouponByCode(ctx context.Context, code string) (domain.Coupon, error) {
	if c, ok := m.byCode[code]; ok {
		return c, nil
	}
	return domain.Coupon{}, domainErrNotFound()
}

func (m *mockRepo) ListCoupons(ctx context.Context, search *string, isActive *bool, limit, offset int) ([]domain.Coupon, int, error) {
	out := []domain.Coupon{}
	for _, c := range m.coupons {
		out = append(out, c)
	}
	return out, len(out), nil
}

func (m *mockRepo) UpdateCoupon(ctx context.Context, c domain.Coupon, expectedVersion int, now time.Time) (domain.Coupon, error) {
	c.Version++
	m.coupons[c.ID] = c
	m.byCode[c.Code] = c
	return c, nil
}

func (m *mockRepo) SetActiveStatus(ctx context.Context, id uuid.UUID, isActive bool, expectedVersion int, now time.Time) (domain.Coupon, error) {
	c := m.coupons[id]
	c.IsActive = isActive
	c.Version++
	m.coupons[id] = c
	m.byCode[c.Code] = c
	return c, nil
}

func (m *mockRepo) GetUserUsageCount(ctx context.Context, couponID, userID uuid.UUID) (int, error) {
	return 0, nil
}

func (m *mockRepo) RecordUsage(ctx context.Context, usage domain.CouponUsage, now time.Time) error {
	return nil
}

func domainErrNotFound() error {
	return &mockErr{msg: "not found"}
}

type mockErr struct{ msg string }

func (e *mockErr) Error() string { return e.msg }

func TestServiceCreateAndValidate(t *testing.T) {
	repo := newMockRepo()
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	svc, err := NewService(Config{Repo: repo, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}

	pkg := "ADVANCED"
	endsAt := now.Add(48 * time.Hour)
	cmd := CreateCouponCommand{
		Code:                  "HAR2026",
		Name:                  "Haradan Special",
		DiscountType:          "PERCENTAGE",
		DiscountValue:         20,
		MaxUsesPerUser:        1,
		ApplicablePackageCode: &pkg,
		StartsAt:              now.Add(-1 * time.Hour),
		EndsAt:                &endsAt,
		CreatedByUserID:       uuid.New(),
	}

	c, err := svc.Create(context.Background(), cmd)
	if err != nil {
		t.Fatalf("create error = %v", err)
	}
	if c.Code != "HAR2026" || c.DiscountValue != 20 {
		t.Fatalf("coupon = %#v", c)
	}

	// Validate coupon
	val, err := svc.ValidateCoupon(context.Background(), uuid.New(), "HAR2026", 10000, &pkg)
	if err != nil {
		t.Fatalf("validate error = %v", err)
	}
	if !val.Valid || val.DiscountAmountMinor != 2000 || val.FinalAmountMinor != 8000 {
		t.Fatalf("validation result = %#v", val)
	}
}
