package auth

import (
	"time"

	"github.com/google/uuid"
)

// ClientContext is the server-validated session client context.
type ClientContext string

const (
	ClientContextPublicWeb ClientContext = "PUBLIC_WEB"
	ClientContextMobile    ClientContext = "MOBILE"
	ClientContextAdminBO   ClientContext = "ADMIN_BO"
)

// Valid reports whether the client context is in the allowlist.
func (c ClientContext) Valid() bool {
	switch c {
	case ClientContextPublicWeb, ClientContextMobile, ClientContextAdminBO:
		return true
	default:
		return false
	}
}

// Session is a server-side refresh session.
type Session struct {
	ID                  uuid.UUID
	UserID              uuid.UUID
	ClientContext       ClientContext
	RefreshTokenHash    string
	FamilyID            uuid.UUID
	ReplacedBySessionID *uuid.UUID
	AbsoluteExpiresAt   time.Time
	IdleExpiresAt       time.Time
	RevokedAt           *time.Time
	RevokeReason        *string
	CreatedAt           time.Time
	LastUsedAt          time.Time
	UserAgent           *string
	IPHash              *string
}

// IsRevoked reports whether the session has been revoked.
func (s Session) IsRevoked() bool {
	return s.RevokedAt != nil
}

// IsExpired reports absolute or idle expiry at now.
func (s Session) IsExpired(now time.Time) bool {
	return !now.Before(s.AbsoluteExpiresAt) || !now.Before(s.IdleExpiresAt)
}

// IsUsable reports whether the session may be refreshed.
func (s Session) IsUsable(now time.Time) bool {
	return !s.IsRevoked() && !s.IsExpired(now)
}

// SecurityEventType matches hrd_security_events.event_type checks.
type SecurityEventType string

const (
	EventLoginSuccess          SecurityEventType = "LOGIN_SUCCESS"
	EventLoginFailure          SecurityEventType = "LOGIN_FAILURE"
	EventLogout                SecurityEventType = "LOGOUT"
	EventSessionRevoked        SecurityEventType = "SESSION_REVOKED"
	EventRefreshReplayDetected SecurityEventType = "REFRESH_REPLAY_DETECTED"
	EventEmailVerification     SecurityEventType = "EMAIL_VERIFICATION"
	EventBOContextRejected     SecurityEventType = "BO_CONTEXT_REJECTED"
)

// SecurityEvent is an append-only auth/security audit record.
type SecurityEvent struct {
	ID            uuid.UUID
	SubjectUserID *uuid.UUID
	ActorUserID   *uuid.UUID
	EventType     SecurityEventType
	ClientContext *ClientContext
	Metadata      map[string]any
	CreatedAt     time.Time
}

// OneTimePurpose matches hrd_one_time_credentials.purpose.
type OneTimePurpose string

const (
	PurposeEmailVerification OneTimePurpose = "EMAIL_VERIFICATION"
)

// OneTimeCredential is a hashed one-time credential.
type OneTimeCredential struct {
	ID                    uuid.UUID
	UserID                uuid.UUID
	Purpose               OneTimePurpose
	TokenHash             string
	TargetEmail           string
	TargetEmailNormalized string
	ExpiresAt             time.Time
	ConsumedAt            *time.Time
	InvalidatedAt         *time.Time
	CreatedAt             time.Time
	RequestIPHash         *string
}

// IsActive reports whether the credential is unused and not invalidated.
func (c OneTimeCredential) IsActive() bool {
	return c.ConsumedAt == nil && c.InvalidatedAt == nil
}

// IsExpired reports whether the credential is expired at now.
func (c OneTimeCredential) IsExpired(now time.Time) bool {
	return !now.Before(c.ExpiresAt)
}

// AccessTokenPurpose distinguishes access JWTs from other credentials.
const AccessTokenPurpose = "access"

// Principal is the authenticated request identity derived from an access token.
type Principal struct {
	UserID        uuid.UUID
	SessionID     uuid.UUID
	Role          string
	ClientContext ClientContext
	SecurityStamp uuid.UUID
}
