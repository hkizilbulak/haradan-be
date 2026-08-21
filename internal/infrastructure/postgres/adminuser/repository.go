// Package adminuser provides PostgreSQL persistence for ADMIN-USER operations.
package adminuser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	appadminuser "github.com/hkizilbulak/haradan-be/internal/application/adminuser"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

type Querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Repository struct {
	pool *pgxpool.Pool
	db   Querier
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool, db: pool} }
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("begin admin user transaction: %w", pg.SanitizeErr(err)))
	}
	return tx, nil
}
func (r *Repository) WithTx(tx pgx.Tx) appadminuser.Repository {
	return &Repository{pool: r.pool, db: tx}
}

const userColumns = `id, email, email_normalized, password_hash, role, status, email_verified_at,
first_name, last_name, phone, security_stamp, failed_login_count, locked_until, created_at, updated_at`

func (r *Repository) ListUsers(ctx context.Context, status *domainuser.Status, role *domainuser.Role, query string, afterCreated *time.Time, afterID *uuid.UUID, limit int) ([]domainuser.User, error) {
	const q = `
SELECT ` + userColumns + `
FROM hrd_users
WHERE ($1::varchar IS NULL OR status = $1)
  AND ($2::varchar IS NULL OR role = $2)
  AND ($3::text = '' OR email ILIKE '%' || $3 || '%' OR first_name ILIKE '%' || $3 || '%' OR last_name ILIKE '%' || $3 || '%')
  AND ($4::timestamptz IS NULL OR (created_at, id) < ($4::timestamptz, $5::uuid))
ORDER BY created_at DESC, id DESC
LIMIT $6`
	var dbStatus, dbRole *string
	if status != nil {
		v := string(*status)
		dbStatus = &v
	}
	if role != nil {
		v := string(*role)
		dbRole = &v
	}
	rows, err := r.db.Query(ctx, q, dbStatus, dbRole, query, afterCreated, afterID, limit)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list admin users: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()
	out := make([]domainuser.User, 0, limit)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan admin user: %w", pg.SanitizeErr(err)))
		}
		out = append(out, user)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("list admin users rows: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

func (r *Repository) FindUser(ctx context.Context, userID uuid.UUID) (domainuser.User, error) {
	return r.findUser(ctx, userID, "")
}
func (r *Repository) FindUserForUpdate(ctx context.Context, userID uuid.UUID) (domainuser.User, error) {
	return r.findUser(ctx, userID, " FOR UPDATE")
}
func (r *Repository) findUser(ctx context.Context, userID uuid.UUID, suffix string) (domainuser.User, error) {
	user, err := scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM hrd_users WHERE id = $1`+suffix, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainuser.User{}, apperr.NotFound("user not found")
	}
	if err != nil {
		return domainuser.User{}, apperr.Internal(fmt.Errorf("find admin user: %w", pg.SanitizeErr(err)))
	}
	return user, nil
}

func (r *Repository) GetDetail(ctx context.Context, userID uuid.UUID, now time.Time) (appadminuser.Detail, error) {
	const q = `
SELECT ` + userColumns + `,
  (SELECT count(*) FROM hrd_auth_sessions
   WHERE user_id = u.id AND revoked_at IS NULL AND absolute_expires_at > $2 AND idle_expires_at > $2)
FROM hrd_users u WHERE id = $1`
	var count int
	user, err := scanUserWithCount(r.db.QueryRow(ctx, q, userID, now), &count)
	if errors.Is(err, pgx.ErrNoRows) {
		return appadminuser.Detail{}, apperr.NotFound("user not found")
	}
	if err != nil {
		return appadminuser.Detail{}, apperr.Internal(fmt.Errorf("get admin user detail: %w", pg.SanitizeErr(err)))
	}
	return appadminuser.Detail{User: user, ActiveSessionCount: count}, nil
}

func (r *Repository) ActiveSessionCount(ctx context.Context, userID uuid.UUID, now time.Time) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
SELECT count(*) FROM hrd_auth_sessions
WHERE user_id = $1 AND revoked_at IS NULL AND absolute_expires_at > $2 AND idle_expires_at > $2`, userID, now).Scan(&count)
	if err != nil {
		return 0, apperr.Internal(fmt.Errorf("count active sessions: %w", pg.SanitizeErr(err)))
	}
	return count, nil
}

func (r *Repository) UpdateRole(ctx context.Context, userID uuid.UUID, role domainuser.Role, securityStamp uuid.UUID, now time.Time) (domainuser.User, error) {
	return r.updateUser(ctx, `role = $2`, userID, role, securityStamp, now)
}
func (r *Repository) UpdateStatus(ctx context.Context, userID uuid.UUID, status domainuser.Status, securityStamp uuid.UUID, now time.Time) (domainuser.User, error) {
	return r.updateUser(ctx, `status = $2`, userID, status, securityStamp, now)
}
func (r *Repository) UpdateProfile(ctx context.Context, userID uuid.UUID, firstName, lastName string, phone *string, now time.Time) (domainuser.User, error) {
	user, err := scanUser(r.db.QueryRow(ctx, `
UPDATE hrd_users
SET first_name = $2, last_name = $3, phone = $4, updated_at = $5
WHERE id = $1
RETURNING `+userColumns, userID, firstName, lastName, phone, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainuser.User{}, apperr.NotFound("user not found")
	}
	if err != nil {
		return domainuser.User{}, apperr.Internal(fmt.Errorf("update admin user profile: %w", pg.SanitizeErr(err)))
	}
	return user, nil
}

func (r *Repository) UpdateEmail(ctx context.Context, userID uuid.UUID, email, emailNormalized string, securityStamp uuid.UUID, now time.Time) (domainuser.User, error) {
	user, err := scanUser(r.db.QueryRow(ctx, `UPDATE hrd_users SET email = $2, email_normalized = $3, email_verified_at = $4, security_stamp = $5, updated_at = $4 WHERE id = $1 RETURNING `+userColumns, userID, email, emailNormalized, now, securityStamp))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainuser.User{}, apperr.NotFound("user not found")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return domainuser.User{}, apperr.Conflict("Bu e-posta adresi zaten kayıtlı.")
		}
		return domainuser.User{}, apperr.Internal(fmt.Errorf("update admin user email: %w", pg.SanitizeErr(err)))
	}
	return user, nil
}

func (r *Repository) FindUserByNormalizedEmail(ctx context.Context, normalized string) (domainuser.User, error) {
	user, err := scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM hrd_users WHERE email_normalized = $1`, normalized))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainuser.User{}, apperr.NotFound("user not found")
	}
	if err != nil {
		return domainuser.User{}, apperr.Internal(fmt.Errorf("find user by email: %w", pg.SanitizeErr(err)))
	}
	return user, nil
}
func (r *Repository) updateUser(ctx context.Context, assignment string, userID uuid.UUID, value any, securityStamp uuid.UUID, now time.Time) (domainuser.User, error) {
	user, err := scanUser(r.db.QueryRow(ctx, `UPDATE hrd_users SET `+assignment+`, security_stamp = $3, updated_at = $4 WHERE id = $1 RETURNING `+userColumns, userID, value, securityStamp, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainuser.User{}, apperr.NotFound("user not found")
	}
	if err != nil {
		return domainuser.User{}, apperr.Internal(fmt.Errorf("update admin user: %w", pg.SanitizeErr(err)))
	}
	return user, nil
}

func (r *Repository) RevokeAllSessions(ctx context.Context, userID uuid.UUID, now time.Time, reason string) error {
	_, err := r.db.Exec(ctx, `UPDATE hrd_auth_sessions SET revoked_at = $2, revoke_reason = $3 WHERE user_id = $1 AND revoked_at IS NULL`, userID, now, reason)
	if err != nil {
		return apperr.Internal(fmt.Errorf("revoke admin user sessions: %w", pg.SanitizeErr(err)))
	}
	return nil
}

func (r *Repository) InsertSecurityEvent(ctx context.Context, event domainauth.SecurityEvent) error {
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return apperr.Internal(fmt.Errorf("marshal security event: %w", err))
	}
	var clientContext *string
	if event.ClientContext != nil {
		v := string(*event.ClientContext)
		clientContext = &v
	}
	_, err = r.db.Exec(ctx, `
INSERT INTO hrd_security_events (id, subject_user_id, actor_user_id, event_type, client_context, metadata, created_at)
VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7)`,
		event.ID, event.SubjectUserID, event.ActorUserID, string(event.EventType), clientContext, raw, event.CreatedAt)
	if err != nil {
		return apperr.Internal(fmt.Errorf("insert admin security event: %w", pg.SanitizeErr(err)))
	}
	return nil
}

func (r *Repository) ListSecurityEvents(ctx context.Context, userID uuid.UUID, eventType *domainauth.SecurityEventType, afterCreated *time.Time, afterID *uuid.UUID, limit int) ([]domainauth.SecurityEvent, error) {
	const q = `
SELECT id, subject_user_id, actor_user_id, event_type, client_context, metadata, created_at
FROM hrd_security_events
WHERE subject_user_id = $1
  AND ($2::varchar IS NULL OR event_type = $2)
  AND ($3::timestamptz IS NULL OR (created_at, id) < ($3::timestamptz, $4::uuid))
ORDER BY created_at DESC, id DESC
LIMIT $5`
	var dbType *string
	if eventType != nil {
		v := string(*eventType)
		dbType = &v
	}
	rows, err := r.db.Query(ctx, q, userID, dbType, afterCreated, afterID, limit)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list security events: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()
	out := make([]domainauth.SecurityEvent, 0, limit)
	for rows.Next() {
		var event domainauth.SecurityEvent
		var eventTypeValue string
		var contextValue *string
		var metadata []byte
		if err := rows.Scan(&event.ID, &event.SubjectUserID, &event.ActorUserID, &eventTypeValue, &contextValue, &metadata, &event.CreatedAt); err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan security event: %w", pg.SanitizeErr(err)))
		}
		event.EventType = domainauth.SecurityEventType(eventTypeValue)
		if contextValue != nil {
			v := domainauth.ClientContext(*contextValue)
			event.ClientContext = &v
		}
		if err := json.Unmarshal(metadata, &event.Metadata); err != nil {
			return nil, apperr.Internal(fmt.Errorf("decode security event metadata: %w", err))
		}
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("list security events rows: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

func (r *Repository) CreateUser(ctx context.Context, user domainuser.User) error {
	const q = `
INSERT INTO hrd_users (
  id, email, email_normalized, password_hash, role, status, email_verified_at,
  first_name, last_name, phone, security_stamp, failed_login_count, locked_until, created_at, updated_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15
)`
	_, err := r.db.Exec(ctx, q,
		user.ID, user.Email, user.EmailNormalized, user.PasswordHash, string(user.Role), string(user.Status), user.EmailVerifiedAt,
		user.FirstName, user.LastName, user.Phone, user.SecurityStamp, user.FailedLoginCount, user.LockedUntil, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict("Bu e-posta adresi zaten kayıtlı.")
		}
		return apperr.Internal(fmt.Errorf("create admin user: %w", pg.SanitizeErr(err)))
	}
	return nil
}

func (r *Repository) CountActiveAdmins(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
SELECT count(*) FROM hrd_users
WHERE role = 'admin' AND status = 'ACTIVE'`).Scan(&count)
	if err != nil {
		return 0, apperr.Internal(fmt.Errorf("count active admins: %w", pg.SanitizeErr(err)))
	}
	return count, nil
}

// LockActiveAdminGuard takes a transaction-scoped advisory lock so concurrent
// demote/disable paths cannot both observe count>1 and leave zero ACTIVE admins.
func (r *Repository) LockActiveAdminGuard(ctx context.Context) error {
	// Stable key derived from a fixed namespace string (must stay constant).
	const key int64 = 0x6872645f61646d6e // "hrd_admn"
	if _, err := r.db.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, key); err != nil {
		return apperr.Internal(fmt.Errorf("lock active admin guard: %w", pg.SanitizeErr(err)))
	}
	return nil
}

func (r *Repository) InvalidateActiveOneTimeCredentials(ctx context.Context, userID uuid.UUID, purpose domainauth.OneTimePurpose, now time.Time) error {
	_, err := r.db.Exec(ctx, `
UPDATE hrd_one_time_credentials
SET invalidated_at = $3
WHERE user_id = $1
  AND purpose = $2
  AND consumed_at IS NULL
  AND invalidated_at IS NULL`, userID, string(purpose), now)
	if err != nil {
		return apperr.Internal(fmt.Errorf("invalidate admin invitation credentials: %w", pg.SanitizeErr(err)))
	}
	return nil
}

func (r *Repository) CreateOneTimeCredential(ctx context.Context, cred domainauth.OneTimeCredential) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO hrd_one_time_credentials (
  id, user_id, purpose, token_hash, target_email, target_email_normalized,
  expires_at, consumed_at, invalidated_at, created_at, request_ip_hash
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11
)`,
		cred.ID, cred.UserID, string(cred.Purpose), cred.TokenHash,
		pg.NullIfEmpty(cred.TargetEmail), pg.NullIfEmpty(cred.TargetEmailNormalized),
		cred.ExpiresAt, cred.ConsumedAt, cred.InvalidatedAt, cred.CreatedAt, cred.RequestIPHash,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict("Davet jetonu çakışması.")
		}
		return apperr.Internal(fmt.Errorf("create admin invitation credential: %w", pg.SanitizeErr(err)))
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func scanUser(row pgx.Row) (domainuser.User, error) {
	var user domainuser.User
	var role, status string
	err := row.Scan(&user.ID, &user.Email, &user.EmailNormalized, &user.PasswordHash, &role, &status, &user.EmailVerifiedAt,
		&user.FirstName, &user.LastName, &user.Phone, &user.SecurityStamp, &user.FailedLoginCount, &user.LockedUntil, &user.CreatedAt, &user.UpdatedAt)
	user.Role, user.Status = domainuser.Role(role), domainuser.Status(status)
	return user, err
}

func scanUserWithCount(row pgx.Row, count *int) (domainuser.User, error) {
	var user domainuser.User
	var role, status string
	err := row.Scan(&user.ID, &user.Email, &user.EmailNormalized, &user.PasswordHash, &role, &status, &user.EmailVerifiedAt,
		&user.FirstName, &user.LastName, &user.Phone, &user.SecurityStamp, &user.FailedLoginCount, &user.LockedUntil, &user.CreatedAt, &user.UpdatedAt, count)
	user.Role, user.Status = domainuser.Role(role), domainuser.Status(status)
	return user, err
}

var _ appadminuser.Repository = (*Repository)(nil)
