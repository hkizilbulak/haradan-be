package paytr

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	domainpaytr "github.com/hkizilbulak/haradan-be/internal/domain/paytr"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

// PostgresChargeRepository persists hrd_paytr_charges.
type PostgresChargeRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresChargeRepository constructs the repository.
func NewPostgresChargeRepository(pool *pgxpool.Pool) *PostgresChargeRepository {
	return &PostgresChargeRepository{pool: pool}
}

const chargeColumns = `
	id, merchant_oid, advert_id, owner_user_id, pkg_code, amount_minor, currency_code,
	status, iframe_token, user_ip, token_request_json, token_response_json, notify_payload_json,
	fail_reason_code, fail_reason_msg, paid_at, advert_submitted_at, version, created_at, updated_at
`

func (r *PostgresChargeRepository) Create(ctx context.Context, c domainpaytr.Charge) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO hrd_paytr_charges (
			id, merchant_oid, advert_id, owner_user_id, pkg_code, amount_minor, currency_code,
			status, iframe_token, user_ip, token_request_json, token_response_json, notify_payload_json,
			fail_reason_code, fail_reason_msg, paid_at, advert_submitted_at, version, created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,
			$8,$9,$10,$11,$12,$13,
			$14,$15,$16,$17,$18,$19,$20
		)`,
		c.ID, c.MerchantOID, c.AdvertID, c.OwnerUserID, string(c.PackageCode), c.AmountMinor, c.CurrencyCode,
		string(c.Status), c.IframeToken, c.UserIP, c.TokenRequestJSON, c.TokenResponseJSON, c.NotifyPayloadJSON,
		c.FailReasonCode, c.FailReasonMsg, c.PaidAt, c.AdvertSubmittedAt, c.Version, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return sanitize(err)
	}
	return nil
}

func (r *PostgresChargeRepository) FindByMerchantOID(ctx context.Context, merchantOID string) (domainpaytr.Charge, error) {
	return r.scanOne(ctx, `
		SELECT `+chargeColumns+`
		FROM hrd_paytr_charges
		WHERE merchant_oid = $1`, merchantOID)
}

func (r *PostgresChargeRepository) FindByMerchantOIDForUpdate(ctx context.Context, merchantOID string) (domainpaytr.Charge, error) {
	return r.scanOne(ctx, `
		SELECT `+chargeColumns+`
		FROM hrd_paytr_charges
		WHERE merchant_oid = $1
		FOR UPDATE`, merchantOID)
}

func (r *PostgresChargeRepository) FindByIDForOwner(ctx context.Context, ownerID, chargeID uuid.UUID) (domainpaytr.Charge, error) {
	return r.scanOne(ctx, `
		SELECT `+chargeColumns+`
		FROM hrd_paytr_charges
		WHERE id = $1 AND owner_user_id = $2`, chargeID, ownerID)
}

func (r *PostgresChargeRepository) Update(ctx context.Context, c domainpaytr.Charge) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE hrd_paytr_charges SET
			status = $2,
			iframe_token = $3,
			user_ip = $4,
			token_request_json = $5,
			token_response_json = $6,
			notify_payload_json = $7,
			fail_reason_code = $8,
			fail_reason_msg = $9,
			paid_at = $10,
			advert_submitted_at = $11,
			version = version + 1,
			updated_at = $12
		WHERE id = $1 AND version = $13`,
		c.ID, string(c.Status), c.IframeToken, c.UserIP, c.TokenRequestJSON, c.TokenResponseJSON,
		c.NotifyPayloadJSON, c.FailReasonCode, c.FailReasonMsg, c.PaidAt, c.AdvertSubmittedAt,
		c.UpdatedAt, c.Version,
	)
	if err != nil {
		return sanitize(err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.StaleVersion("Ödeme kaydı güncellenemedi.")
	}
	return nil
}

func (r *PostgresChargeRepository) scanOne(ctx context.Context, q string, args ...any) (domainpaytr.Charge, error) {
	return scanCharge(r.pool.QueryRow(ctx, q, args...))
}

type scannable interface {
	Scan(dest ...any) error
}

func scanCharge(row scannable) (domainpaytr.Charge, error) {
	var c domainpaytr.Charge
	var pkgCode, status string
	err := row.Scan(
		&c.ID, &c.MerchantOID, &c.AdvertID, &c.OwnerUserID, &pkgCode, &c.AmountMinor, &c.CurrencyCode,
		&status, &c.IframeToken, &c.UserIP, &c.TokenRequestJSON, &c.TokenResponseJSON, &c.NotifyPayloadJSON,
		&c.FailReasonCode, &c.FailReasonMsg, &c.PaidAt, &c.AdvertSubmittedAt, &c.Version, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainpaytr.Charge{}, apperr.NotFound("Ödeme kaydı bulunamadı.")
		}
		return domainpaytr.Charge{}, sanitize(err)
	}
	c.PackageCode = domainpackaging.PackageCode(pkgCode)
	c.Status = domainpaytr.ChargeStatus(status)
	return c, nil
}

func sanitize(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "password") || strings.Contains(msg, "postgres://") {
		return fmt.Errorf("database error")
	}
	return postgres.SanitizeErr(err)
}
