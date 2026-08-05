package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

// Querier is implemented by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository implements user persistence.
type Repository struct {
	db Querier
}

// NewRepository constructs a user repository.
func NewRepository(db Querier) *Repository {
	return &Repository{db: db}
}

const userColumns = `id, email, email_normalized, password_hash, role, status, email_verified_at,
first_name, last_name, phone, security_stamp, failed_login_count, locked_until, created_at, updated_at`

// FindByNormalizedEmail returns a user by normalized email.
func (r *Repository) FindByNormalizedEmail(ctx context.Context, emailNormalized string) (domainuser.User, error) {
	const q = `SELECT ` + userColumns + ` FROM hrd_users WHERE email_normalized = $1`
	u, err := scanUser(r.db.QueryRow(ctx, q, emailNormalized))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainuser.User{}, apperr.NotFound("user not found")
	}
	if err != nil {
		return domainuser.User{}, apperr.Internal(fmt.Errorf("find user by email: %w", pg.SanitizeErr(err)))
	}
	return u, nil
}

// FindByID returns a user by id.
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (domainuser.User, error) {
	const q = `SELECT ` + userColumns + ` FROM hrd_users WHERE id = $1`
	u, err := scanUser(r.db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainuser.User{}, apperr.NotFound("user not found")
	}
	if err != nil {
		return domainuser.User{}, apperr.Internal(fmt.Errorf("find user by id: %w", pg.SanitizeErr(err)))
	}
	return u, nil
}

// FindByIDForUpdate locks and returns a user by id.
func (r *Repository) FindByIDForUpdate(ctx context.Context, id uuid.UUID) (domainuser.User, error) {
	const q = `SELECT ` + userColumns + ` FROM hrd_users WHERE id = $1 FOR UPDATE`
	u, err := scanUser(r.db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainuser.User{}, apperr.NotFound("user not found")
	}
	if err != nil {
		return domainuser.User{}, apperr.Internal(fmt.Errorf("find user by id for update: %w", pg.SanitizeErr(err)))
	}
	return u, nil
}

// Create inserts a new user.
func (r *Repository) Create(ctx context.Context, u domainuser.User) error {
	const q = `
INSERT INTO hrd_users (
  id, email, email_normalized, password_hash, role, status, email_verified_at,
  first_name, last_name, phone, security_stamp, failed_login_count, locked_until, created_at, updated_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15
)`
	_, err := r.db.Exec(ctx, q,
		u.ID, u.Email, u.EmailNormalized, u.PasswordHash, string(u.Role), string(u.Status), u.EmailVerifiedAt,
		u.FirstName, u.LastName, u.Phone, u.SecurityStamp, u.FailedLoginCount, u.LockedUntil, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict("email already registered")
		}
		return apperr.Internal(fmt.Errorf("create user: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// RecordFailedLogin increments failed_login_count. Exact lock thresholds are not locked in docs.
func (r *Repository) RecordFailedLogin(ctx context.Context, userID uuid.UUID, now time.Time) error {
	const q = `
UPDATE hrd_users
SET failed_login_count = failed_login_count + 1,
    updated_at = $2
WHERE id = $1`
	_, err := r.db.Exec(ctx, q, userID, now)
	if err != nil {
		return apperr.Internal(fmt.Errorf("record failed login: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// ResetFailedLogin clears brute-force counters after a successful login.
func (r *Repository) ResetFailedLogin(ctx context.Context, userID uuid.UUID, now time.Time) error {
	const q = `
UPDATE hrd_users
SET failed_login_count = 0,
    locked_until = NULL,
    updated_at = $2
WHERE id = $1`
	_, err := r.db.Exec(ctx, q, userID, now)
	if err != nil {
		return apperr.Internal(fmt.Errorf("reset failed login: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// MarkEmailVerified sets email_verified_at when not already set.
func (r *Repository) MarkEmailVerified(ctx context.Context, userID uuid.UUID, verifiedAt time.Time) error {
	const q = `
UPDATE hrd_users
SET email_verified_at = COALESCE(email_verified_at, $2),
    updated_at = $2
WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, userID, verifiedAt)
	if err != nil {
		return apperr.Internal(fmt.Errorf("mark email verified: %w", pg.SanitizeErr(err)))
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("user not found")
	}
	return nil
}

// UpdatePasswordHash atomically replaces a credential and invalidates access tokens.
func (r *Repository) UpdatePasswordHash(ctx context.Context, userID uuid.UUID, passwordHash string, securityStamp uuid.UUID, now time.Time) error {
	const q = `UPDATE hrd_users SET password_hash = $2, security_stamp = $3, failed_login_count = 0, locked_until = NULL, updated_at = $4 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, userID, passwordHash, securityStamp, now)
	if err != nil {
		return apperr.Internal(fmt.Errorf("update password: %w", pg.SanitizeErr(err)))
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("user not found")
	}
	return nil
}

// UpdateEmail replaces the canonical address and requires re-verification.
func (r *Repository) UpdateEmail(ctx context.Context, userID uuid.UUID, email, emailNormalized string, securityStamp uuid.UUID, now time.Time) error {
	const q = `UPDATE hrd_users SET email = $2, email_normalized = $3, email_verified_at = NULL, security_stamp = $4, updated_at = $5 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, userID, email, emailNormalized, securityStamp, now)
	if isUniqueViolation(err) {
		return apperr.Conflict("email already registered")
	}
	if err != nil {
		return apperr.Internal(fmt.Errorf("update email: %w", pg.SanitizeErr(err)))
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("user not found")
	}
	return nil
}

// UpdateProfile updates editable profile fields and returns the updated user.
func (r *Repository) UpdateProfile(ctx context.Context, userID uuid.UUID, firstName, lastName *string, phoneSet bool, phone *string, now time.Time) (domainuser.User, error) {
	const q = `
UPDATE hrd_users
SET first_name = COALESCE($2, first_name),
    last_name = COALESCE($3, last_name),
    phone = CASE WHEN $4 THEN $5 ELSE phone END,
    updated_at = $6
WHERE id = $1
RETURNING ` + userColumns
	u, err := scanUser(r.db.QueryRow(ctx, q, userID, firstName, lastName, phoneSet, phone, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainuser.User{}, apperr.NotFound("user not found")
	}
	if err != nil {
		return domainuser.User{}, apperr.Internal(fmt.Errorf("update profile: %w", pg.SanitizeErr(err)))
	}
	return u, nil
}

func scanUser(row pgx.Row) (domainuser.User, error) {
	var u domainuser.User
	var role, status string
	err := row.Scan(
		&u.ID, &u.Email, &u.EmailNormalized, &u.PasswordHash, &role, &status, &u.EmailVerifiedAt,
		&u.FirstName, &u.LastName, &u.Phone, &u.SecurityStamp, &u.FailedLoginCount, &u.LockedUntil,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return domainuser.User{}, err
	}
	u.Role = domainuser.Role(role)
	u.Status = domainuser.Status(status)
	return u, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
