package auth

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

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	pg "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

// Querier is implemented by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository persists auth sessions, one-time credentials, and security events.
type Repository struct {
	pool *pgxpool.Pool
	db   Querier
}

// NewRepository constructs an auth repository bound to a pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, db: pool}
}

// WithTx returns a repository scoped to a transaction querier.
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{pool: r.pool, db: tx}
}

// BeginTx starts a read-write transaction.
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("begin auth tx: %w", pg.SanitizeErr(err)))
	}
	return tx, nil
}

const sessionColumns = `id, user_id, client_context, refresh_token_hash, family_id, replaced_by_session_id,
absolute_expires_at, idle_expires_at, revoked_at, revoke_reason, created_at, last_used_at, user_agent, ip_hash`

// CreateSession inserts a new auth session.
func (r *Repository) CreateSession(ctx context.Context, s domainauth.Session) error {
	const q = `
INSERT INTO hrd_auth_sessions (
  id, user_id, client_context, refresh_token_hash, family_id, replaced_by_session_id,
  absolute_expires_at, idle_expires_at, revoked_at, revoke_reason, created_at, last_used_at, user_agent, ip_hash
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14
)`
	_, err := r.db.Exec(ctx, q,
		s.ID, s.UserID, string(s.ClientContext), s.RefreshTokenHash, s.FamilyID, s.ReplacedBySessionID,
		s.AbsoluteExpiresAt, s.IdleExpiresAt, s.RevokedAt, s.RevokeReason, s.CreatedAt, s.LastUsedAt, s.UserAgent, s.IPHash,
	)
	if err != nil {
		return apperr.Internal(fmt.Errorf("create session: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// FindSessionByRefreshHashForUpdate locks and returns a session by refresh token hash.
func (r *Repository) FindSessionByRefreshHashForUpdate(ctx context.Context, hash string) (domainauth.Session, error) {
	const q = `SELECT ` + sessionColumns + ` FROM hrd_auth_sessions WHERE refresh_token_hash = $1 FOR UPDATE`
	s, err := scanSession(r.db.QueryRow(ctx, q, hash))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainauth.Session{}, apperr.NotFound("session not found")
	}
	if err != nil {
		return domainauth.Session{}, apperr.Internal(fmt.Errorf("find session by refresh hash: %w", pg.SanitizeErr(err)))
	}
	return s, nil
}

// FindSessionByID returns a session by id without locking.
func (r *Repository) FindSessionByID(ctx context.Context, id uuid.UUID) (domainauth.Session, error) {
	const q = `SELECT ` + sessionColumns + ` FROM hrd_auth_sessions WHERE id = $1`
	s, err := scanSession(r.db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainauth.Session{}, apperr.NotFound("session not found")
	}
	if err != nil {
		return domainauth.Session{}, apperr.Internal(fmt.Errorf("find session by id: %w", pg.SanitizeErr(err)))
	}
	return s, nil
}

// FindSessionByIDForUpdate locks a session row.
func (r *Repository) FindSessionByIDForUpdate(ctx context.Context, id uuid.UUID) (domainauth.Session, error) {
	const q = `SELECT ` + sessionColumns + ` FROM hrd_auth_sessions WHERE id = $1 FOR UPDATE`
	s, err := scanSession(r.db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainauth.Session{}, apperr.NotFound("session not found")
	}
	if err != nil {
		return domainauth.Session{}, apperr.Internal(fmt.Errorf("find session for update: %w", pg.SanitizeErr(err)))
	}
	return s, nil
}

// ListSessionsByUserID returns sessions for a user ordered by last_used_at DESC, id DESC.
func (r *Repository) ListSessionsByUserID(ctx context.Context, userID uuid.UUID, afterLastUsed *time.Time, afterID *uuid.UUID, limit int) ([]domainauth.Session, error) {
	const q = `
SELECT ` + sessionColumns + `
FROM hrd_auth_sessions
WHERE user_id = $1
  AND (
    $2::timestamptz IS NULL
    OR (last_used_at, id) < ($2::timestamptz, $3::uuid)
  )
ORDER BY last_used_at DESC, id DESC
LIMIT $4`
	rows, err := r.db.Query(ctx, q, userID, afterLastUsed, afterID, limit)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("list sessions: %w", pg.SanitizeErr(err)))
	}
	defer rows.Close()
	out := make([]domainauth.Session, 0, limit)
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, apperr.Internal(fmt.Errorf("scan session: %w", pg.SanitizeErr(err)))
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(fmt.Errorf("list sessions rows: %w", pg.SanitizeErr(err)))
	}
	return out, nil
}

// RevokeSession marks a session revoked if not already revoked.
func (r *Repository) RevokeSession(ctx context.Context, id uuid.UUID, now time.Time, reason string, replacedBy *uuid.UUID) error {
	const q = `
UPDATE hrd_auth_sessions
SET revoked_at = COALESCE(revoked_at, $2),
    revoke_reason = COALESCE(revoke_reason, $3),
    replaced_by_session_id = COALESCE(replaced_by_session_id, $4)
WHERE id = $1`
	_, err := r.db.Exec(ctx, q, id, now, reason, replacedBy)
	if err != nil {
		return apperr.Internal(fmt.Errorf("revoke session: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// RevokeSessionForUser revokes a session owned by userID.
// Cross-user IDs return NOT_FOUND to avoid confirming another user's session exists.
func (r *Repository) RevokeSessionForUser(ctx context.Context, userID, sessionID uuid.UUID, now time.Time, reason string) (domainauth.Session, bool, error) {
	s, err := r.FindSessionByIDForUpdate(ctx, sessionID)
	if err != nil {
		return domainauth.Session{}, false, err
	}
	if s.UserID != userID {
		return domainauth.Session{}, false, apperr.NotFound("session not found")
	}
	if s.IsRevoked() {
		return s, false, nil
	}
	if err := r.RevokeSession(ctx, sessionID, now, reason, nil); err != nil {
		return domainauth.Session{}, false, err
	}
	s.RevokedAt = &now
	s.RevokeReason = &reason
	return s, true, nil
}

// RevokeAllSessionsForUser revokes every non-revoked session for the user.
func (r *Repository) RevokeAllSessionsForUser(ctx context.Context, userID uuid.UUID, now time.Time, reason string) error {
	const q = `
UPDATE hrd_auth_sessions
SET revoked_at = $2,
    revoke_reason = $3
WHERE user_id = $1
  AND revoked_at IS NULL`
	_, err := r.db.Exec(ctx, q, userID, now, reason)
	if err != nil {
		return apperr.Internal(fmt.Errorf("revoke all sessions: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// RevokeFamily revokes all non-revoked sessions in a rotation family.
func (r *Repository) RevokeFamily(ctx context.Context, familyID uuid.UUID, now time.Time, reason string) error {
	const q = `
UPDATE hrd_auth_sessions
SET revoked_at = $2,
    revoke_reason = $3
WHERE family_id = $1
  AND revoked_at IS NULL`
	_, err := r.db.Exec(ctx, q, familyID, now, reason)
	if err != nil {
		return apperr.Internal(fmt.Errorf("revoke session family: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// CreateOneTimeCredential inserts a one-time credential.
func (r *Repository) CreateOneTimeCredential(ctx context.Context, c domainauth.OneTimeCredential) error {
	const q = `
INSERT INTO hrd_one_time_credentials (
  id, user_id, purpose, token_hash, target_email, target_email_normalized,
  expires_at, consumed_at, invalidated_at, created_at, request_ip_hash
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11
)`
	_, err := r.db.Exec(ctx, q,
		c.ID, c.UserID, string(c.Purpose), c.TokenHash, c.TargetEmail, c.TargetEmailNormalized,
		c.ExpiresAt, c.ConsumedAt, c.InvalidatedAt, c.CreatedAt, c.RequestIPHash,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict("one-time credential conflict")
		}
		return apperr.Internal(fmt.Errorf("create one-time credential: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// FindOneTimeCredentialByHashForUpdate locks a credential by token hash.
func (r *Repository) FindOneTimeCredentialByHashForUpdate(ctx context.Context, tokenHash string) (domainauth.OneTimeCredential, error) {
	const q = `
SELECT id, user_id, purpose, token_hash, target_email, target_email_normalized,
       expires_at, consumed_at, invalidated_at, created_at, request_ip_hash
FROM hrd_one_time_credentials
WHERE token_hash = $1
FOR UPDATE`
	c, err := scanOTC(r.db.QueryRow(ctx, q, tokenHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return domainauth.OneTimeCredential{}, apperr.NotFound("credential not found")
	}
	if err != nil {
		return domainauth.OneTimeCredential{}, apperr.Internal(fmt.Errorf("find one-time credential: %w", pg.SanitizeErr(err)))
	}
	return c, nil
}

// ConsumeOneTimeCredential marks an active credential as consumed.
func (r *Repository) ConsumeOneTimeCredential(ctx context.Context, id uuid.UUID, now time.Time) error {
	const q = `
UPDATE hrd_one_time_credentials
SET consumed_at = $2
WHERE id = $1
  AND consumed_at IS NULL
  AND invalidated_at IS NULL`
	tag, err := r.db.Exec(ctx, q, id, now)
	if err != nil {
		return apperr.Internal(fmt.Errorf("consume one-time credential: %w", pg.SanitizeErr(err)))
	}
	if tag.RowsAffected() == 0 {
		return apperr.TokenAlreadyUsed("Doğrulama jetonu zaten kullanılmış.")
	}
	return nil
}

// InvalidateActiveOneTimeCredentials invalidates active credentials for user+purpose.
func (r *Repository) InvalidateActiveOneTimeCredentials(ctx context.Context, userID uuid.UUID, purpose domainauth.OneTimePurpose, now time.Time) error {
	const q = `
UPDATE hrd_one_time_credentials
SET invalidated_at = $3
WHERE user_id = $1
  AND purpose = $2
  AND consumed_at IS NULL
  AND invalidated_at IS NULL`
	_, err := r.db.Exec(ctx, q, userID, string(purpose), now)
	if err != nil {
		return apperr.Internal(fmt.Errorf("invalidate one-time credentials: %w", pg.SanitizeErr(err)))
	}
	return nil
}

// InsertSecurityEvent appends a security event.
func (r *Repository) InsertSecurityEvent(ctx context.Context, e domainauth.SecurityEvent) error {
	metadata := e.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return apperr.Internal(fmt.Errorf("marshal security metadata: %w", err))
	}
	var clientContext *string
	if e.ClientContext != nil {
		v := string(*e.ClientContext)
		clientContext = &v
	}
	const q = `
INSERT INTO hrd_security_events (
  id, subject_user_id, actor_user_id, event_type, client_context, metadata, created_at
) VALUES (
  $1,$2,$3,$4,$5,$6::jsonb,$7
)`
	_, err = r.db.Exec(ctx, q,
		e.ID, e.SubjectUserID, e.ActorUserID, string(e.EventType), clientContext, metaJSON, e.CreatedAt,
	)
	if err != nil {
		return apperr.Internal(fmt.Errorf("insert security event: %w", pg.SanitizeErr(err)))
	}
	return nil
}

func scanSession(row pgx.Row) (domainauth.Session, error) {
	var s domainauth.Session
	var clientContext string
	err := row.Scan(
		&s.ID, &s.UserID, &clientContext, &s.RefreshTokenHash, &s.FamilyID, &s.ReplacedBySessionID,
		&s.AbsoluteExpiresAt, &s.IdleExpiresAt, &s.RevokedAt, &s.RevokeReason, &s.CreatedAt, &s.LastUsedAt,
		&s.UserAgent, &s.IPHash,
	)
	if err != nil {
		return domainauth.Session{}, err
	}
	s.ClientContext = domainauth.ClientContext(clientContext)
	return s, nil
}

func scanOTC(row pgx.Row) (domainauth.OneTimeCredential, error) {
	var c domainauth.OneTimeCredential
	var purpose string
	var targetEmail, targetEmailNormalized *string
	err := row.Scan(
		&c.ID, &c.UserID, &purpose, &c.TokenHash, &targetEmail, &targetEmailNormalized,
		&c.ExpiresAt, &c.ConsumedAt, &c.InvalidatedAt, &c.CreatedAt, &c.RequestIPHash,
	)
	if err != nil {
		return domainauth.OneTimeCredential{}, err
	}
	c.Purpose = domainauth.OneTimePurpose(purpose)
	if targetEmail != nil {
		c.TargetEmail = *targetEmail
	}
	if targetEmailNormalized != nil {
		c.TargetEmailNormalized = *targetEmailNormalized
	}
	return c, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
