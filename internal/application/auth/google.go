package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	"github.com/hkizilbulak/haradan-be/internal/platform/security/emailnorm"
)

// GoogleLoginInput is the input for Google authentication.
type GoogleLoginInput struct {
	IDToken       string
	Credential    string
	Code          string
	RedirectURI   string
	ClientContext domainauth.ClientContext
	UserAgent     string
	ClientIP      string
}

type googleTokenClaims struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Audience      string `json:"aud"`
	Error         string `json:"error"`
	ErrorDesc     string `json:"error_description"`
}

func (c *googleTokenClaims) isVerified() bool {
	switch v := c.EmailVerified.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	default:
		return false
	}
}

// GoogleLogin authenticates or registers a user using Google OAuth / ID Token.
func (s *Service) GoogleLogin(ctx context.Context, in GoogleLoginInput) (TokenResult, error) {
	if !in.ClientContext.Valid() {
		return TokenResult{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "clientContext", Message: "Geçersiz istemci bağlamı."})
	}
	if in.ClientContext == domainauth.ClientContextAdminBO {
		return TokenResult{}, apperr.Forbidden(apperr.CodeForbidden, "Google ile giriş sadece kullanıcı arayüzünde (FE) kullanılabilir.")
	}

	rawToken := strings.TrimSpace(in.IDToken)
	if rawToken == "" {
		rawToken = strings.TrimSpace(in.Credential)
	}

	var claims googleTokenClaims
	httpClient := &http.Client{Timeout: 15 * time.Second}

	if in.Code != "" && rawToken == "" {
		data := url.Values{}
		data.Set("code", strings.TrimSpace(in.Code))
		data.Set("client_id", s.googleClientID)
		data.Set("client_secret", s.googleClientSecret)
		if in.RedirectURI != "" {
			data.Set("redirect_uri", in.RedirectURI)
		}
		data.Set("grant_type", "authorization_code")

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
		if err != nil {
			return TokenResult{}, apperr.Internal(fmt.Errorf("google token request: %w", err))
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := httpClient.Do(req)
		if err != nil {
			return TokenResult{}, apperr.DependencyUnavailable("Google servisine ulaşılamıyor.")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return TokenResult{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Google yetkilendirme kodu geçersiz.")
		}

		var tokenResp struct {
			IDToken string `json:"id_token"`
		}
		if err := json.Unmarshal(body, &tokenResp); err != nil || tokenResp.IDToken == "" {
			return TokenResult{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Google kimlik jetonu alınamadı.")
		}
		rawToken = tokenResp.IDToken
	}

	if rawToken == "" {
		return TokenResult{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "idToken", Message: "Google jetonu gereklidir."})
	}

	tokenInfoURL := "https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(rawToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenInfoURL, nil)
	if err != nil {
		return TokenResult{}, apperr.Internal(fmt.Errorf("google tokeninfo req: %w", err))
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return TokenResult{}, apperr.DependencyUnavailable("Google servisine ulaşılamıyor.")
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return TokenResult{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Google oturumu doğrulanamadı.")
	}

	if err := json.Unmarshal(body, &claims); err != nil {
		return TokenResult{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Google yanıtı çözümlenemedi.")
	}

	if claims.Error != "" || claims.Email == "" {
		return TokenResult{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Google hesabı doğrulanamadı.")
	}

	if s.googleClientID != "" && claims.Audience != s.googleClientID {
		return TokenResult{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Google istemci kimliği eşleşmedi.")
	}

	if !claims.isVerified() {
		return TokenResult{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Google e-posta adresi doğrulanmamış.")
	}

	email := strings.TrimSpace(claims.Email)
	if !emailnorm.ValidFormat(email) {
		return TokenResult{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Geçersiz e-posta adresi.")
	}

	normalized := emailnorm.Normalize(email)
	now := s.clock.Now()

	user, err := s.users.FindByNormalizedEmail(ctx, normalized)
	if err != nil {
		ae, isAppErr := apperr.As(err)
		if !isAppErr || ae.Kind != apperr.KindNotFound {
			return TokenResult{}, err
		}

		first := strings.TrimSpace(claims.GivenName)
		last := strings.TrimSpace(claims.FamilyName)
		if first == "" && last == "" {
			parts := strings.Fields(strings.TrimSpace(claims.Name))
			if len(parts) > 1 {
				first = parts[0]
				last = strings.Join(parts[1:], " ")
			} else if len(parts) == 1 {
				first = parts[0]
				last = "Google"
			}
		}
		if first == "" {
			first = "Kullanıcı"
		}
		if last == "" {
			last = "Google"
		}

		randomBytes := make([]byte, 24)
		_, _ = rand.Read(randomBytes)
		randomPassword := hex.EncodeToString(randomBytes)
		pwdHash, err := s.hasher.Hash(randomPassword)
		if err != nil {
			return TokenResult{}, apperr.Internal(fmt.Errorf("hash password: %w", err))
		}

		verifiedAt := now
		newUser := domainuser.User{
			ID:              uuid.New(),
			Email:           email,
			EmailNormalized: normalized,
			PasswordHash:    pwdHash,
			Role:            domainuser.RoleUser,
			Status:          domainuser.StatusActive,
			Channel:         domainuser.ChannelGoogle,
			EmailVerifiedAt: &verifiedAt,
			FirstName:       first,
			LastName:        last,
			Phone:           nil,
			SecurityStamp:   uuid.New(),
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		if err := s.withTx(ctx, func(ctx context.Context, users UserRepository, _ SessionRepository) error {
			return users.Create(ctx, newUser)
		}); err != nil {
			return TokenResult{}, err
		}
		user = newUser
	} else {
		if !user.IsActive() {
			return TokenResult{}, apperr.Forbidden(apperr.CodeAccountInactive, "Hesap aktif değil.")
		}
		if user.IsLocked(now) {
			return TokenResult{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, genericAuthFailure)
		}

		_ = s.withTx(ctx, func(ctx context.Context, users UserRepository, _ SessionRepository) error {
			if user.EmailVerifiedAt == nil {
				_ = users.MarkEmailVerified(ctx, user.ID, now)
			}
			return users.UpdateChannel(ctx, user.ID, domainuser.ChannelGoogle, now)
		})
	}

	var result TokenResult
	err = s.withTx(ctx, func(ctx context.Context, users UserRepository, sessions SessionRepository) error {
		if err := users.ResetFailedLogin(ctx, user.ID, now); err != nil {
			return err
		}
		tr, err := s.createSessionTokens(ctx, sessions, user, in.ClientContext, in.UserAgent, in.ClientIP, now, uuid.New())
		if err != nil {
			return err
		}
		result = tr
		return nil
	})
	if err != nil {
		return TokenResult{}, err
	}

	uid := user.ID
	s.bestEffortEvent(ctx, domainauth.SecurityEvent{
		ID:            uuid.New(),
		SubjectUserID: &uid,
		ActorUserID:   &uid,
		EventType:     domainauth.EventLoginSuccess,
		ClientContext: &in.ClientContext,
		Metadata:      map[string]any{"channel": "GOOGLE", "email": user.Email},
		CreatedAt:     now,
	})

	return result, nil
}
