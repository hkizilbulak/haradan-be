package token_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	"github.com/hkizilbulak/haradan-be/internal/platform/security/token"
)

func newManager(t *testing.T, clock token.Clock) *token.Manager {
	t.Helper()
	m, err := token.NewManager(token.Config{
		JWTSecret:          "test-secret-for-jwt-signing",
		AccessTokenTTL:     time.Minute,
		RefreshAbsoluteTTL: 24 * time.Hour,
		RefreshIdleTTL:     time.Hour,
		Clock:              clock,
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestAccessTokenRoundTripAndTamper(t *testing.T) {
	clock := token.FixedClock{T: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	m := newManager(t, clock)
	p := domainauth.Principal{
		UserID: uuid.New(), SessionID: uuid.New(), Role: "user",
		ClientContext: domainauth.ClientContextPublicWeb, SecurityStamp: uuid.New(),
	}
	tok, _, err := m.IssueAccessToken(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.ParseAccessToken(tok)
	if err != nil || got.UserID != p.UserID || got.SessionID != p.SessionID {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if _, err := m.ParseAccessToken(tok + "x"); err == nil {
		t.Fatal("expected tamper failure")
	}
}

func TestAccessTokenExpiry(t *testing.T) {
	clock := &token.FixedClock{T: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	m := newManager(t, clock)
	tok, _, err := m.IssueAccessToken(domainauth.Principal{
		UserID: uuid.New(), SessionID: uuid.New(), Role: "user",
		ClientContext: domainauth.ClientContextMobile, SecurityStamp: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	clock.T = clock.T.Add(2 * time.Minute)
	if _, err := m.ParseAccessToken(tok); err == nil {
		t.Fatal("expected expiry")
	}
}

func TestAccessTokenWrongPurposeRejected(t *testing.T) {
	clock := token.FixedClock{T: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	m := newManager(t, clock)
	now := clock.Now()
	claims := token.AccessClaims{
		Purpose:       "refresh",
		SessionID:     uuid.NewString(),
		Role:          "user",
		ClientContext: string(domainauth.ClientContextPublicWeb),
		SecurityStamp: uuid.NewString(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte("test-secret-for-jwt-signing"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.ParseAccessToken(signed); err == nil {
		t.Fatal("expected purpose rejection")
	}
}

func TestAccessTokenRejectsNoneAndWrongAlg(t *testing.T) {
	m := newManager(t, token.FixedClock{T: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
	headerNone, _ := json.Marshal(map[string]any{"alg": "none", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]any{
		"purpose":        "access",
		"sub":            uuid.NewString(),
		"sid":            uuid.NewString(),
		"role":           "user",
		"client_context": "PUBLIC_WEB",
		"security_stamp": uuid.NewString(),
		"iat":            time.Now().Unix(),
		"exp":            time.Now().Add(time.Minute).Unix(),
	})
	noneTok := base64.RawURLEncoding.EncodeToString(headerNone) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + "."
	if _, err := m.ParseAccessToken(noneTok); err == nil {
		t.Fatal("expected none rejection")
	}

	headerHS384, _ := json.Marshal(map[string]any{"alg": "HS384", "typ": "JWT"})
	mac := hmac.New(sha256.New, []byte("test-secret-for-jwt-signing"))
	signingInput := base64.RawURLEncoding.EncodeToString(headerHS384) + "." + base64.RawURLEncoding.EncodeToString(payload)
	_, _ = mac.Write([]byte(signingInput))
	// Intentionally wrong algorithm label even with a signature blob.
	wrongAlg := signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if _, err := m.ParseAccessToken(wrongAlg); err == nil {
		t.Fatal("expected wrong alg rejection")
	}
}

func TestOpaqueRefreshNotAcceptedAsAccess(t *testing.T) {
	m := newManager(t, token.FixedClock{T: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
	plain, _, err := token.NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	_, parseErr := m.ParseAccessToken(plain)
	if parseErr == nil {
		t.Fatal("opaque refresh must not parse as access")
	}
	if strings.Contains(parseErr.Error(), plain) {
		t.Fatal("token leaked in error")
	}
}

func TestRefreshHash(t *testing.T) {
	plain, hash, err := token.NewRefreshToken()
	if err != nil || plain == "" || hash == "" || plain == hash {
		t.Fatalf("plain/hash err=%v", err)
	}
	if token.HashRefreshToken(plain) != hash {
		t.Fatal("hash mismatch")
	}
	if token.HashRefreshToken(plain+"x") == hash {
		t.Fatal("expected different hash")
	}
}

func TestEmptySecretRejected(t *testing.T) {
	_, err := token.NewManager(token.Config{
		JWTSecret:          "",
		AccessTokenTTL:     time.Minute,
		RefreshAbsoluteTTL: time.Hour,
		RefreshIdleTTL:     time.Minute,
	})
	if err == nil {
		t.Fatal("expected empty secret error")
	}
}
