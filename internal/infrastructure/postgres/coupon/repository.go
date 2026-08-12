package coupon

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domain "github.com/hkizilbulak/haradan-be/internal/domain/coupon"
)

type DB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Repository struct {
	pool *pgxpool.Pool
	db   DB
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, db: pool}
}

func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{pool: r.pool, db: tx}
}

const couponCols = `id,code,name,discount_type,discount_value,max_uses,uses_count,max_uses_per_user,min_spend_amount_minor,applicable_package_code,starts_at,ends_at,is_active,created_by_user_id,version,created_at,updated_at`

func scanCoupon(row interface{ Scan(...any) error }) (domain.Coupon, error) {
	var c domain.Coupon
	var dtype string
	err := row.Scan(
		&c.ID, &c.Code, &c.Name, &dtype, &c.DiscountValue, &c.MaxUses,
		&c.UsesCount, &c.MaxUsesPerUser, &c.MinSpendAmountMinor, &c.ApplicablePackageCode,
		&c.StartsAt, &c.EndsAt, &c.IsActive, &c.CreatedByUserID, &c.Version, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return domain.Coupon{}, err
	}
	dt, _ := domain.ParseDiscountType(dtype)
	c.DiscountType = dt
	return c, nil
}

func (r *Repository) CreateCoupon(ctx context.Context, c domain.Coupon) error {
	code := domain.NormalizeCode(c.Code)
	_, err := r.db.Exec(ctx, `
INSERT INTO hrd_coupons (`+couponCols+`)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		c.ID, code, c.Name, string(c.DiscountType), c.DiscountValue, c.MaxUses,
		c.UsesCount, c.MaxUsesPerUser, c.MinSpendAmountMinor, c.ApplicablePackageCode,
		c.StartsAt, c.EndsAt, c.IsActive, c.CreatedByUserID, c.Version, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperr.Conflict("Bu kod ile zaten bir kupon mevcut.")
		}
		return apperr.Internal(fmt.Errorf("create coupon: %w", err))
	}
	return nil
}

func (r *Repository) GetCouponByID(ctx context.Context, id uuid.UUID) (domain.Coupon, error) {
	c, err := scanCoupon(r.db.QueryRow(ctx, `SELECT `+couponCols+` FROM hrd_coupons WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Coupon{}, apperr.NotFound("Kupon bulunamadı.")
	}
	if err != nil {
		return domain.Coupon{}, apperr.Internal(fmt.Errorf("get coupon by id: %w", err))
	}
	return c, nil
}

func (r *Repository) GetCouponByCode(ctx context.Context, code string) (domain.Coupon, error) {
	norm := domain.NormalizeCode(code)
	c, err := scanCoupon(r.db.QueryRow(ctx, `SELECT `+couponCols+` FROM hrd_coupons WHERE code=$1`, norm))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Coupon{}, apperr.NotFound("Kupon bulunamadı.")
	}
	if err != nil {
		return domain.Coupon{}, apperr.Internal(fmt.Errorf("get coupon by code: %w", err))
	}
	return c, nil
}

func (r *Repository) ListCoupons(ctx context.Context, search *string, isActive *bool, limit, offset int) ([]domain.Coupon, int, error) {
	countQ := `SELECT COUNT(*) FROM hrd_coupons WHERE ($1::varchar IS NULL OR code ILIKE '%'||$1||'%' OR name ILIKE '%'||$1||'%') AND ($2::boolean IS NULL OR is_active=$2)`
	var total int
	var sArg *string
	if search != nil && *search != "" {
		sArg = search
	}
	if err := r.db.QueryRow(ctx, countQ, sArg, isActive).Scan(&total); err != nil {
		return nil, 0, apperr.Internal(fmt.Errorf("count coupons: %w", err))
	}

	q := `SELECT ` + couponCols + ` FROM hrd_coupons WHERE ($1::varchar IS NULL OR code ILIKE '%'||$1||'%' OR name ILIKE '%'||$1||'%') AND ($2::boolean IS NULL OR is_active=$2) ORDER BY created_at DESC LIMIT $3 OFFSET $4`
	rows, err := r.db.Query(ctx, q, sArg, isActive, limit, offset)
	if err != nil {
		return nil, 0, apperr.Internal(fmt.Errorf("list coupons: %w", err))
	}
	defer rows.Close()

	out := []domain.Coupon{}
	for rows.Next() {
		c, err := scanCoupon(rows)
		if err != nil {
			return nil, 0, apperr.Internal(err)
		}
		out = append(out, c)
	}
	return out, total, nil
}

func (r *Repository) UpdateCoupon(ctx context.Context, c domain.Coupon, expectedVersion int, now time.Time) (domain.Coupon, error) {
	tag, err := r.db.Exec(ctx, `
UPDATE hrd_coupons SET
  name=$3, discount_type=$4, discount_value=$5, max_uses=$6, max_uses_per_user=$7,
  min_spend_amount_minor=$8, applicable_package_code=$9, starts_at=$10, ends_at=$11,
  is_active=$12, version=version+1, updated_at=$13
WHERE id=$1 AND version=$2`,
		c.ID, expectedVersion, c.Name, string(c.DiscountType), c.DiscountValue, c.MaxUses,
		c.MaxUsesPerUser, c.MinSpendAmountMinor, c.ApplicablePackageCode, c.StartsAt, c.EndsAt,
		c.IsActive, now,
	)
	if err != nil {
		return domain.Coupon{}, apperr.Internal(fmt.Errorf("update coupon: %w", err))
	}
	if tag.RowsAffected() != 1 {
		return domain.Coupon{}, apperr.Conflict("Kupon güncellenemedi veya versiyon çakışması var.")
	}
	return r.GetCouponByID(ctx, c.ID)
}

func (r *Repository) SetActiveStatus(ctx context.Context, id uuid.UUID, isActive bool, expectedVersion int, now time.Time) (domain.Coupon, error) {
	tag, err := r.db.Exec(ctx, `UPDATE hrd_coupons SET is_active=$3, version=version+1, updated_at=$4 WHERE id=$1 AND version=$2`, id, expectedVersion, isActive, now)
	if err != nil {
		return domain.Coupon{}, apperr.Internal(fmt.Errorf("set coupon active status: %w", err))
	}
	if tag.RowsAffected() != 1 {
		return domain.Coupon{}, apperr.Conflict("Kupon durumu güncellenemedi.")
	}
	return r.GetCouponByID(ctx, id)
}

func (r *Repository) GetUserUsageCount(ctx context.Context, couponID, userID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrd_coupon_usages WHERE coupon_id=$1 AND user_id=$2`, couponID, userID).Scan(&count)
	if err != nil {
		return 0, apperr.Internal(fmt.Errorf("get user usage count: %w", err))
	}
	return count, nil
}

func (r *Repository) RecordUsage(ctx context.Context, usage domain.CouponUsage, now time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(fmt.Errorf("begin coupon usage tx: %w", err))
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `UPDATE hrd_coupons SET uses_count=uses_count+1, version=version+1, updated_at=$2 WHERE id=$1 AND is_active=true AND (max_uses IS NULL OR uses_count < max_uses)`, usage.CouponID, now)
	if err != nil {
		return apperr.Internal(fmt.Errorf("increment coupon uses: %w", err))
	}
	if tag.RowsAffected() != 1 {
		return apperr.Conflict("Kupon kullanım limiti dolmuş veya pasife alınmış.")
	}

	_, err = tx.Exec(ctx, `INSERT INTO hrd_coupon_usages (id, coupon_id, user_id, advert_id, discount_amount_minor, used_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		usage.ID, usage.CouponID, usage.UserID, usage.AdvertID, usage.DiscountAmountMinor, usage.UsedAt, usage.CreatedAt,
	)
	if err != nil {
		return apperr.Internal(fmt.Errorf("insert coupon usage: %w", err))
	}

	return tx.Commit(ctx)
}
