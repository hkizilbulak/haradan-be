package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
)

// Clock provides the current time for token expiry checks.
type Clock interface {
	Now() time.Time
}

// SystemClock uses time.Now.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

// FixedClock is a test clock.
type FixedClock struct{ T time.Time }

func (c FixedClock) Now() time.Time { return c.T.UTC() }

// AccessClaims are JWT claims for short-lived access tokens.
type AccessClaims struct {
	Purpose       string `json:"purpose"`
	SessionID     string `json:"sid"`
	Role          string `json:"role"`
	ClientContext string `json:"client_context"`
	SecurityStamp string `json:"security_stamp"`
	jwt.RegisteredClaims
}

// Manager issues and validates access JWTs and opaque refresh tokens.
type Manager struct {
	secret         []byte
	accessTTL      time.Duration
	refreshAbsTTL  time.Duration
	refreshIdleTTL time.Duration
	clock          Clock
}

// Config configures the token manager.
type Config struct {
	JWTSecret          string
	AccessTokenTTL     time.Duration
	RefreshAbsoluteTTL time.Duration
	RefreshIdleTTL     time.Duration
	Clock              Clock
}

// NewManager constructs a Manager. Secret must be non-empty.
func NewManager(cfg Config) (*Manager, error) {
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("auth JWT secret must not be empty")
	}
	if cfg.AccessTokenTTL <= 0 || cfg.RefreshAbsoluteTTL <= 0 || cfg.RefreshIdleTTL <= 0 {
		return nil, fmt.Errorf("auth token TTLs must be greater than zero")
	}
	if cfg.RefreshIdleTTL > cfg.RefreshAbsoluteTTL {
		return nil, fmt.Errorf("refresh idle TTL must not exceed absolute TTL")
	}
	clock := cfg.Clock
	if clock == nil {
		clock = SystemClock{}
	}
	return &Manager{
		secret:         []byte(cfg.JWTSecret),
		accessTTL:      cfg.AccessTokenTTL,
		refreshAbsTTL:  cfg.RefreshAbsoluteTTL,
		refreshIdleTTL: cfg.RefreshIdleTTL,
		clock:          clock,
	}, nil
}

// AccessTTLSeconds returns access token lifetime in whole seconds.
func (m *Manager) AccessTTLSeconds() int {
	return int(m.accessTTL / time.Second)
}

// RefreshTTLs returns absolute and idle refresh lifetimes.
func (m *Manager) RefreshTTLs() (absolute, idle time.Duration) {
	return m.refreshAbsTTL, m.refreshIdleTTL
}

// IssueAccessToken creates a signed access JWT.
func (m *Manager) IssueAccessToken(principal domainauth.Principal) (string, time.Time, error) {
	now := m.clock.Now()
	exp := now.Add(m.accessTTL)
	claims := AccessClaims{
		Purpose:       domainauth.AccessTokenPurpose,
		SessionID:     principal.SessionID.String(),
		Role:          principal.Role,
		ClientContext: string(principal.ClientContext),
		SecurityStamp: principal.SecurityStamp.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   principal.UserID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        uuid.NewString(),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, exp, nil
}

// ParseAccessToken validates an access JWT and returns the principal.
func (m *Manager) ParseAccessToken(tokenString string) (domainauth.Principal, error) {
	var claims AccessClaims
	parsed, err := jwt.ParseWithClaims(
		tokenString,
		&claims,
		func(t *jwt.Token) (any, error) {
			if t.Method == nil || t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(func() time.Time { return m.clock.Now() }),
		jwt.WithExpirationRequired(),
	)
	if err != nil || parsed == nil || !parsed.Valid {
		return domainauth.Principal{}, fmt.Errorf("invalid access token")
	}
	if claims.Purpose != domainauth.AccessTokenPurpose {
		return domainauth.Principal{}, fmt.Errorf("invalid access token purpose")
	}
	if claims.Subject == "" || claims.ExpiresAt == nil || claims.IssuedAt == nil {
		return domainauth.Principal{}, fmt.Errorf("invalid access token claims")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return domainauth.Principal{}, fmt.Errorf("invalid access token subject")
	}
	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return domainauth.Principal{}, fmt.Errorf("invalid access token session")
	}
	stamp, err := uuid.Parse(claims.SecurityStamp)
	if err != nil {
		return domainauth.Principal{}, fmt.Errorf("invalid access token stamp")
	}
	ctx := domainauth.ClientContext(claims.ClientContext)
	if !ctx.Valid() {
		return domainauth.Principal{}, fmt.Errorf("invalid access token context")
	}
	return domainauth.Principal{
		UserID:        userID,
		SessionID:     sessionID,
		Role:          claims.Role,
		ClientContext: ctx,
		SecurityStamp: stamp,
	}, nil
}

// NewRefreshToken creates a high-entropy opaque refresh token and its hash.
func NewRefreshToken() (plaintext string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	plaintext = base64.RawURLEncoding.EncodeToString(buf)
	return plaintext, HashRefreshToken(plaintext), nil
}

// HashRefreshToken returns a SHA-256 hex fingerprint of a refresh token.
func HashRefreshToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// NewOpaqueToken creates a high-entropy opaque token and hash (one-time credentials).
func NewOpaqueToken() (plaintext string, hash string, err error) {
	return NewRefreshToken()
}

// HashOpaqueToken hashes one-time credential plaintext.
func HashOpaqueToken(plaintext string) string {
	return HashRefreshToken(plaintext)
}
