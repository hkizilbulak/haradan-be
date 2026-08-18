package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	"github.com/hkizilbulak/haradan-be/internal/platform/security/emailnorm"
	"github.com/hkizilbulak/haradan-be/internal/platform/security/password"
	"github.com/hkizilbulak/haradan-be/internal/platform/security/phone"
	"github.com/hkizilbulak/haradan-be/internal/platform/security/token"
)

const (
	registerSuccessMessage = "Kayıt alındı. E-posta doğrulama talimatları gönderildi."
	resendSuccessMessage   = "Doğrulama e-postası talimatları gönderildi."
	verifySuccessMessage   = "E-posta adresi doğrulandı."
	logoutSuccessMessage   = "Oturum kapatıldı."
	genericAuthFailure     = "E-posta veya parola hatalı."
	genericTokenFailure    = "Doğrulama jetonu geçersiz."
)

// PasswordHasher hashes and verifies passwords.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(encodedHash, password string) (bool, error)
}

// TokenManager issues access/refresh credentials.
type TokenManager interface {
	AccessTTLSeconds() int
	RefreshTTLs() (absolute, idle time.Duration)
	IssueAccessToken(principal domainauth.Principal) (string, time.Time, error)
	ParseAccessToken(tokenString string) (domainauth.Principal, error)
}

// Clock provides current time.
type Clock interface {
	Now() time.Time
}

// EmailSender delivers verification and password-reset emails after commit.
// fullName is a trimmed display name; empty means the sender must omit fullName
// from template variables rather than sending an empty string.
type EmailSender interface {
	SendRegistrationVerification(ctx context.Context, toEmail, plaintextToken, fullName string) error
	SendPasswordReset(ctx context.Context, toEmail, plaintextToken, fullName string) error
}

// NoopEmailSender acknowledges delivery without sending.
type NoopEmailSender struct{}

func (NoopEmailSender) SendRegistrationVerification(context.Context, string, string, string) error {
	return nil
}

func (NoopEmailSender) SendPasswordReset(context.Context, string, string, string) error {
	return nil
}

// EmailSenderFunc adapts a function to EmailSender (registration path only).
// Prefer a concrete fake when password-reset coverage is needed.
type EmailSenderFunc func(ctx context.Context, toEmail, plaintextToken, fullName string) error

func (f EmailSenderFunc) SendRegistrationVerification(ctx context.Context, toEmail, plaintextToken, fullName string) error {
	return f(ctx, toEmail, plaintextToken, fullName)
}

func (f EmailSenderFunc) SendPasswordReset(ctx context.Context, toEmail, plaintextToken, fullName string) error {
	return f(ctx, toEmail, plaintextToken, fullName)
}

func displayFullName(firstName, lastName string) string {
	return strings.TrimSpace(strings.TrimSpace(firstName) + " " + strings.TrimSpace(lastName))
}

// Service implements AUTH-01/02/03/04/05/06 use cases.
type Service struct {
	users             UserRepository
	sessions          SessionRepository
	userTx            UserRepositoryFactory
	hasher            PasswordHasher
	tokens            TokenManager
	clock             Clock
	email             EmailSender
	emailVerifyTTL    time.Duration
	dummyPasswordHash string
	autoVerifyEmail   bool
}

// Config wires auth application dependencies.
type Config struct {
	Users             UserRepository
	Sessions          SessionRepository
	UserTx            UserRepositoryFactory
	Hasher            PasswordHasher
	Tokens            TokenManager
	Clock             Clock
	EmailSender       EmailSender
	EmailVerifyTTL    time.Duration
	DummyPasswordHash string
	// AutoVerifyEmail skips the email verification step and marks new accounts
	// as verified immediately on registration. Use when email delivery is not
	// configured (e.g. EMAIL_PROVIDER=unconfigured).
	AutoVerifyEmail bool
}

// NewService constructs the auth application service.
func NewService(cfg Config) (*Service, error) {
	if cfg.Users == nil || cfg.Sessions == nil || cfg.UserTx == nil || cfg.Hasher == nil || cfg.Tokens == nil {
		return nil, fmt.Errorf("auth service dependencies are required")
	}
	clock := cfg.Clock
	if clock == nil {
		clock = token.SystemClock{}
	}
	email := cfg.EmailSender
	if email == nil {
		email = NoopEmailSender{}
	}
	if cfg.EmailVerifyTTL <= 0 {
		return nil, fmt.Errorf("email verification TTL must be greater than zero")
	}
	dummy := cfg.DummyPasswordHash
	if dummy == "" {
		if h, ok := cfg.Hasher.(*password.Hasher); ok {
			dummy = password.DummyHash(h)
		}
	}
	return &Service{
		users:             cfg.Users,
		sessions:          cfg.Sessions,
		userTx:            cfg.UserTx,
		hasher:            cfg.Hasher,
		tokens:            cfg.Tokens,
		clock:             clock,
		email:             email,
		emailVerifyTTL:    cfg.EmailVerifyTTL,
		dummyPasswordHash: dummy,
		autoVerifyEmail:   cfg.AutoVerifyEmail,
	}, nil
}

// RegisterInput is AUTH-01 input.
type RegisterInput struct {
	Email     string
	Password  string
	FirstName string
	LastName  string
	Phone     *string
	ClientIP  string
}

// RegisterResult is AUTH-01 output.
type RegisterResult struct {
	Message string
}

// LoginInput is AUTH-04 input.
type LoginInput struct {
	Email         string
	Password      string
	ClientContext domainauth.ClientContext
	UserAgent     string
	ClientIP      string
}

// TokenResult is AUTH-04/05 output.
type TokenResult struct {
	AccessToken   string
	RefreshToken  string
	TokenType     string
	ExpiresIn     int
	ClientContext domainauth.ClientContext
}

// RefreshInput is AUTH-05 input.
type RefreshInput struct {
	RefreshToken  string
	ClientContext domainauth.ClientContext
	UserAgent     string
	ClientIP      string
}

// LogoutInput is AUTH-06 input.
type LogoutInput struct {
	AccessToken string
}

// LogoutResult is AUTH-06 output.
type LogoutResult struct {
	Message string
}

// VerifyEmailInput is AUTH-02 input.
type VerifyEmailInput struct {
	Token string
}

// VerifyEmailResult is AUTH-02 output.
type VerifyEmailResult struct {
	Message string
}

// ResendVerificationInput is AUTH-03 input.
type ResendVerificationInput struct {
	Email    string
	ClientIP string
}

// ResendVerificationResult is AUTH-03 output.
type ResendVerificationResult struct {
	Message string
}

type RequestPasswordResetInput struct {
	Email    string
	ClientIP string
}
type ResetPasswordInput struct {
	Token       string
	NewPassword string
}
type RequestEmailChangeInput struct {
	UserID   uuid.UUID
	NewEmail string
	ClientIP string
}
type ConfirmEmailChangeInput struct{ Token string }
type ChangePasswordInput struct {
	UserID          uuid.UUID
	CurrentPassword string
	NewPassword     string
}

// Register implements AUTH-01.
func (s *Service) Register(ctx context.Context, in RegisterInput) (RegisterResult, error) {
	email := strings.TrimSpace(in.Email)
	first := strings.TrimSpace(in.FirstName)
	last := strings.TrimSpace(in.LastName)
	if !emailnorm.ValidFormat(email) {
		return RegisterResult{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "email", Message: "Geçerli bir e-posta girin."})
	}
	if len(in.Password) < 8 {
		return RegisterResult{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "password", Message: "Parola en az 8 karakter olmalıdır."})
	}
	if first == "" {
		return RegisterResult{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "firstName", Message: "Ad zorunludur."})
	}
	if last == "" {
		return RegisterResult{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "lastName", Message: "Soyad zorunludur."})
	}
	if firstNameTooLong(first) || lastNameTooLong(last) {
		return RegisterResult{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "firstName", Message: "Ad veya soyad çok uzun."})
	}
	normalizedPhone, err := phone.NormalizeOptional(in.Phone)
	if err != nil {
		return RegisterResult{}, err
	}

	normalized := emailnorm.Normalize(email)
	now := s.clock.Now()

	existing, err := s.users.FindByNormalizedEmail(ctx, normalized)
	if err == nil && existing.ID != uuid.Nil {
		return RegisterResult{Message: registerSuccessMessage}, nil
	}
	if err != nil {
		if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindNotFound {
			return RegisterResult{}, err
		}
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return RegisterResult{}, apperr.Internal(fmt.Errorf("hash password: %w", err))
	}
	verifyPlain, verifyHash, err := token.NewOpaqueToken()
	if err != nil {
		return RegisterResult{}, apperr.Internal(err)
	}

	var verifiedAt *time.Time
	if s.autoVerifyEmail {
		t := now
		verifiedAt = &t
	}
	user := domainuser.User{
		ID:              uuid.New(),
		Email:           email,
		EmailNormalized: normalized,
		PasswordHash:    hash,
		Role:            domainuser.RoleUser,
		Status:          domainuser.StatusActive,
		FirstName:       first,
		LastName:        last,
		Phone:           normalizedPhone,
		SecurityStamp:   uuid.New(),
		EmailVerifiedAt: verifiedAt,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if s.autoVerifyEmail {
		if err := s.withTx(ctx, func(ctx context.Context, users UserRepository, _ SessionRepository) error {
			return users.Create(ctx, user)
		}); err != nil {
			if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindConflict {
				return RegisterResult{Message: registerSuccessMessage}, nil
			}
			return RegisterResult{}, err
		}
		return RegisterResult{Message: registerSuccessMessage}, nil
	}

	cred := domainauth.OneTimeCredential{
		ID:                    uuid.New(),
		UserID:                user.ID,
		Purpose:               domainauth.PurposeEmailVerification,
		TokenHash:             verifyHash,
		TargetEmail:           email,
		TargetEmailNormalized: normalized,
		ExpiresAt:             now.Add(s.emailVerifyTTL),
		CreatedAt:             now,
		RequestIPHash:         hashIP(in.ClientIP),
	}

	if err := s.withTx(ctx, func(ctx context.Context, users UserRepository, sessions SessionRepository) error {
		if err := users.Create(ctx, user); err != nil {
			return err
		}
		if err := sessions.InvalidateActiveOneTimeCredentials(ctx, user.ID, domainauth.PurposeEmailVerification, now); err != nil {
			return err
		}
		return sessions.CreateOneTimeCredential(ctx, cred)
	}); err != nil {
		if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindConflict {
			return RegisterResult{Message: registerSuccessMessage}, nil
		}
		return RegisterResult{}, err
	}

	if err := s.email.SendRegistrationVerification(ctx, email, verifyPlain, displayFullName(first, last)); err != nil {
		return RegisterResult{}, apperr.DependencyUnavailable("E-posta servisi şu anda kullanılamıyor.")
	}
	return RegisterResult{Message: registerSuccessMessage}, nil
}

// VerifyEmail implements AUTH-02.
func (s *Service) VerifyEmail(ctx context.Context, in VerifyEmailInput) (VerifyEmailResult, error) {
	plain := strings.TrimSpace(in.Token)
	if plain == "" {
		return VerifyEmailResult{}, apperr.TokenInvalid(genericTokenFailure)
	}
	hash := token.HashOpaqueToken(plain)
	now := s.clock.Now()

	var subjectUserID uuid.UUID
	err := s.withTx(ctx, func(ctx context.Context, users UserRepository, sessions SessionRepository) error {
		cred, err := sessions.FindOneTimeCredentialByHashForUpdate(ctx, hash)
		if err != nil {
			if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
				return apperr.TokenInvalid(genericTokenFailure)
			}
			return err
		}
		if cred.Purpose != domainauth.PurposeEmailVerification {
			return apperr.TokenInvalid(genericTokenFailure)
		}
		if cred.InvalidatedAt != nil {
			return apperr.TokenInvalid(genericTokenFailure)
		}

		user, err := users.FindByIDForUpdate(ctx, cred.UserID)
		if err != nil {
			if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
				return apperr.TokenInvalid(genericTokenFailure)
			}
			return err
		}
		subjectUserID = user.ID

		if cred.ConsumedAt != nil {
			if user.EmailVerifiedAt != nil {
				// Conditional idempotent success when already verified.
				return nil
			}
			return apperr.TokenAlreadyUsed("Doğrulama jetonu zaten kullanılmış.")
		}
		if cred.IsExpired(now) {
			return apperr.TokenExpired("Doğrulama jetonunun süresi dolmuş.")
		}

		if err := sessions.ConsumeOneTimeCredential(ctx, cred.ID, now); err != nil {
			if ae, ok := apperr.As(err); ok && ae.Code == apperr.CodeTokenAlreadyUsed {
				if user.EmailVerifiedAt != nil {
					return nil
				}
			}
			return err
		}
		return users.MarkEmailVerified(ctx, user.ID, now)
	})
	if err != nil {
		return VerifyEmailResult{}, err
	}

	uid := subjectUserID
	s.bestEffortEvent(ctx, domainauth.SecurityEvent{
		ID:            uuid.New(),
		SubjectUserID: &uid,
		ActorUserID:   &uid,
		EventType:     domainauth.EventEmailVerification,
		Metadata:      map[string]any{},
		CreatedAt:     now,
	})
	return VerifyEmailResult{Message: verifySuccessMessage}, nil
}

// ResendVerification implements AUTH-03 (enumeration-safe).
func (s *Service) ResendVerification(ctx context.Context, in ResendVerificationInput) (ResendVerificationResult, error) {
	email := strings.TrimSpace(in.Email)
	if !emailnorm.ValidFormat(email) {
		return ResendVerificationResult{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "email", Message: "Geçerli bir e-posta girin."})
	}
	normalized := emailnorm.Normalize(email)
	now := s.clock.Now()

	user, err := s.users.FindByNormalizedEmail(ctx, normalized)
	if err != nil {
		if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
			return ResendVerificationResult{Message: resendSuccessMessage}, nil
		}
		return ResendVerificationResult{}, err
	}
	// Already verified or non-ACTIVE: same external success, no credential/email.
	if user.EmailVerifiedAt != nil || !user.IsActive() {
		return ResendVerificationResult{Message: resendSuccessMessage}, nil
	}

	verifyPlain, verifyHash, err := token.NewOpaqueToken()
	if err != nil {
		return ResendVerificationResult{}, apperr.Internal(err)
	}
	cred := domainauth.OneTimeCredential{
		ID:                    uuid.New(),
		UserID:                user.ID,
		Purpose:               domainauth.PurposeEmailVerification,
		TokenHash:             verifyHash,
		TargetEmail:           user.Email,
		TargetEmailNormalized: user.EmailNormalized,
		ExpiresAt:             now.Add(s.emailVerifyTTL),
		CreatedAt:             now,
		RequestIPHash:         hashIP(in.ClientIP),
	}

	created := false
	if err := s.withTx(ctx, func(ctx context.Context, _ UserRepository, sessions SessionRepository) error {
		if err := sessions.InvalidateActiveOneTimeCredentials(ctx, user.ID, domainauth.PurposeEmailVerification, now); err != nil {
			return err
		}
		if err := sessions.CreateOneTimeCredential(ctx, cred); err != nil {
			if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindConflict {
				// Concurrent resend won the one-active slot; stay enumeration-safe.
				return nil
			}
			return err
		}
		created = true
		return nil
	}); err != nil {
		return ResendVerificationResult{}, err
	}
	if !created {
		return ResendVerificationResult{Message: resendSuccessMessage}, nil
	}

	if err := s.email.SendRegistrationVerification(ctx, user.Email, verifyPlain, displayFullName(user.FirstName, user.LastName)); err != nil {
		return ResendVerificationResult{}, apperr.DependencyUnavailable("E-posta servisi şu anda kullanılamıyor.")
	}
	return ResendVerificationResult{Message: resendSuccessMessage}, nil
}

// RequestPasswordReset implements AUTH-10 without revealing account existence.
func (s *Service) RequestPasswordReset(ctx context.Context, in RequestPasswordResetInput) (ResendVerificationResult, error) {
	email := strings.TrimSpace(in.Email)
	if !emailnorm.ValidFormat(email) {
		return ResendVerificationResult{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "email", Message: "Geçerli bir e-posta girin."})
	}
	user, err := s.users.FindByNormalizedEmail(ctx, emailnorm.Normalize(email))
	if err != nil {
		if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
			return ResendVerificationResult{Message: resendSuccessMessage}, nil
		}
		return ResendVerificationResult{}, err
	}
	if !user.IsActive() {
		return ResendVerificationResult{Message: resendSuccessMessage}, nil
	}
	plain, hash, err := token.NewOpaqueToken()
	if err != nil {
		return ResendVerificationResult{}, apperr.Internal(err)
	}
	now := s.clock.Now()
	cred := domainauth.OneTimeCredential{ID: uuid.New(), UserID: user.ID, Purpose: domainauth.PurposePasswordReset, TokenHash: hash, ExpiresAt: now.Add(s.emailVerifyTTL), CreatedAt: now, RequestIPHash: hashIP(in.ClientIP)}
	if err := s.withTx(ctx, func(ctx context.Context, _ UserRepository, sessions SessionRepository) error {
		if err := sessions.InvalidateActiveOneTimeCredentials(ctx, user.ID, cred.Purpose, now); err != nil {
			return err
		}
		return sessions.CreateOneTimeCredential(ctx, cred)
	}); err != nil {
		return ResendVerificationResult{}, err
	}
	if err := s.email.SendPasswordReset(ctx, user.Email, plain, displayFullName(user.FirstName, user.LastName)); err != nil {
		return ResendVerificationResult{}, apperr.DependencyUnavailable("E-posta servisi şu anda kullanılamıyor.")
	}
	return ResendVerificationResult{Message: resendSuccessMessage}, nil
}

// ResetPassword implements AUTH-11 and revokes every existing session.
func (s *Service) ResetPassword(ctx context.Context, in ResetPasswordInput) (LogoutResult, error) {
	if strings.TrimSpace(in.Token) == "" {
		return LogoutResult{}, apperr.TokenInvalid(genericTokenFailure)
	}
	if len(in.NewPassword) < 8 {
		return LogoutResult{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "newPassword", Message: "Parola en az 8 karakter olmalıdır."})
	}
	hash, err := s.hasher.Hash(in.NewPassword)
	if err != nil {
		return LogoutResult{}, apperr.Internal(err)
	}
	now := s.clock.Now()
	var uid uuid.UUID
	err = s.withTx(ctx, func(ctx context.Context, users UserRepository, sessions SessionRepository) error {
		cred, err := sessions.FindOneTimeCredentialByHashForUpdate(ctx, token.HashOpaqueToken(strings.TrimSpace(in.Token)))
		if err != nil {
			return apperr.TokenInvalid(genericTokenFailure)
		}
		if cred.Purpose != domainauth.PurposePasswordReset || !cred.IsActive() {
			return apperr.TokenInvalid(genericTokenFailure)
		}
		if cred.IsExpired(now) {
			return apperr.TokenExpired("Doğrulama jetonunun süresi dolmuş.")
		}
		user, err := users.FindByIDForUpdate(ctx, cred.UserID)
		if err != nil {
			return apperr.TokenInvalid(genericTokenFailure)
		}
		uid = user.ID
		if err := sessions.ConsumeOneTimeCredential(ctx, cred.ID, now); err != nil {
			return err
		}
		if err := users.UpdatePasswordHash(ctx, user.ID, hash, uuid.New(), now); err != nil {
			return err
		}
		// Invitation/password-setup link proves mailbox possession.
		if user.EmailVerifiedAt == nil {
			if err := users.MarkEmailVerified(ctx, user.ID, now); err != nil {
				return err
			}
		}
		return sessions.RevokeAllSessionsForUser(ctx, user.ID, now, "PASSWORD_RESET")
	})
	if err != nil {
		return LogoutResult{}, err
	}
	s.bestEffortEvent(ctx, domainauth.SecurityEvent{ID: uuid.New(), SubjectUserID: &uid, ActorUserID: &uid, EventType: domainauth.EventPasswordReset, Metadata: map[string]any{}, CreatedAt: now})
	return LogoutResult{Message: "Parola güncellendi."}, nil
}

func (s *Service) RequestEmailChange(ctx context.Context, in RequestEmailChangeInput) (LogoutResult, error) {
	email := strings.TrimSpace(in.NewEmail)
	if !emailnorm.ValidFormat(email) {
		return LogoutResult{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "newEmail", Message: "Geçerli bir e-posta girin."})
	}
	normalized := emailnorm.Normalize(email)
	now := s.clock.Now()
	user, err := s.requireActiveUser(ctx, in.UserID)
	if err != nil {
		return LogoutResult{}, err
	}
	if user.EmailVerifiedAt == nil {
		return LogoutResult{}, apperr.Forbidden(apperr.CodeForbidden, "E-posta doğrulanmamış.")
	}
	if user.EmailNormalized == normalized {
		return LogoutResult{}, apperr.Conflict("email already registered")
	}
	if _, err := s.users.FindByNormalizedEmail(ctx, normalized); err == nil {
		return LogoutResult{}, apperr.Conflict("email already registered")
	} else if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindNotFound {
		return LogoutResult{}, err
	}
	plain, hash, err := token.NewOpaqueToken()
	if err != nil {
		return LogoutResult{}, apperr.Internal(err)
	}
	cred := domainauth.OneTimeCredential{ID: uuid.New(), UserID: user.ID, Purpose: domainauth.PurposeEmailChangeVerification, TokenHash: hash, TargetEmail: email, TargetEmailNormalized: normalized, ExpiresAt: now.Add(s.emailVerifyTTL), CreatedAt: now, RequestIPHash: hashIP(in.ClientIP)}
	if err := s.withTx(ctx, func(ctx context.Context, _ UserRepository, sessions SessionRepository) error {
		if err := sessions.InvalidateActiveOneTimeCredentials(ctx, user.ID, cred.Purpose, now); err != nil {
			return err
		}
		return sessions.CreateOneTimeCredential(ctx, cred)
	}); err != nil {
		return LogoutResult{}, err
	}
	if err := s.email.SendRegistrationVerification(ctx, email, plain, displayFullName(user.FirstName, user.LastName)); err != nil {
		return LogoutResult{}, apperr.DependencyUnavailable("E-posta servisi şu anda kullanılamıyor.")
	}
	return LogoutResult{Message: resendSuccessMessage}, nil
}

func (s *Service) ConfirmEmailChange(ctx context.Context, in ConfirmEmailChangeInput) (LogoutResult, error) {
	if strings.TrimSpace(in.Token) == "" {
		return LogoutResult{}, apperr.TokenInvalid(genericTokenFailure)
	}
	now := s.clock.Now()
	var uid uuid.UUID
	err := s.withTx(ctx, func(ctx context.Context, users UserRepository, sessions SessionRepository) error {
		cred, err := sessions.FindOneTimeCredentialByHashForUpdate(ctx, token.HashOpaqueToken(strings.TrimSpace(in.Token)))
		if err != nil {
			return apperr.TokenInvalid(genericTokenFailure)
		}
		if cred.Purpose != domainauth.PurposeEmailChangeVerification || !cred.IsActive() {
			return apperr.TokenInvalid(genericTokenFailure)
		}
		if cred.IsExpired(now) {
			return apperr.TokenExpired("Doğrulama jetonunun süresi dolmuş.")
		}
		user, err := users.FindByIDForUpdate(ctx, cred.UserID)
		if err != nil {
			return apperr.TokenInvalid(genericTokenFailure)
		}
		uid = user.ID
		if err := sessions.ConsumeOneTimeCredential(ctx, cred.ID, now); err != nil {
			return err
		}
		if err := users.UpdateEmail(ctx, user.ID, cred.TargetEmail, cred.TargetEmailNormalized, uuid.New(), now); err != nil {
			return err
		}
		// Token delivery to the new address proves possession.
		if err := users.MarkEmailVerified(ctx, user.ID, now); err != nil {
			return err
		}
		return sessions.RevokeAllSessionsForUser(ctx, user.ID, now, "EMAIL_CHANGE")
	})
	if err != nil {
		return LogoutResult{}, err
	}
	s.bestEffortEvent(ctx, domainauth.SecurityEvent{ID: uuid.New(), SubjectUserID: &uid, ActorUserID: &uid, EventType: domainauth.EventEmailChange, Metadata: map[string]any{}, CreatedAt: now})
	return LogoutResult{Message: "E-posta adresi güncellendi."}, nil
}

func (s *Service) ChangePassword(ctx context.Context, in ChangePasswordInput) (LogoutResult, error) {
	if len(in.NewPassword) < 8 {
		return LogoutResult{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "newPassword", Message: "Parola en az 8 karakter olmalıdır."})
	}
	now := s.clock.Now()
	var uid uuid.UUID
	err := s.withTx(ctx, func(ctx context.Context, users UserRepository, sessions SessionRepository) error {
		user, err := users.FindByIDForUpdate(ctx, in.UserID)
		if err != nil {
			return err
		}
		if !user.IsActive() {
			return apperr.Forbidden(apperr.CodeAccountInactive, "Hesap aktif değil.")
		}
		ok, err := s.hasher.Verify(user.PasswordHash, in.CurrentPassword)
		if err != nil || !ok {
			return apperr.Unauthenticated(apperr.CodeUnauthenticated, genericAuthFailure)
		}
		hash, err := s.hasher.Hash(in.NewPassword)
		if err != nil {
			return apperr.Internal(err)
		}
		uid = user.ID
		if err := users.UpdatePasswordHash(ctx, user.ID, hash, uuid.New(), now); err != nil {
			return err
		}
		return sessions.RevokeAllSessionsForUser(ctx, user.ID, now, "PASSWORD_CHANGE")
	})
	if err != nil {
		return LogoutResult{}, err
	}
	s.bestEffortEvent(ctx, domainauth.SecurityEvent{ID: uuid.New(), SubjectUserID: &uid, ActorUserID: &uid, EventType: domainauth.EventPasswordChange, Metadata: map[string]any{}, CreatedAt: now})
	return LogoutResult{Message: "Parola güncellendi."}, nil
}

// Login implements AUTH-04.
func (s *Service) Login(ctx context.Context, in LoginInput) (TokenResult, error) {
	if !in.ClientContext.Valid() {
		return TokenResult{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "clientContext", Message: "Geçersiz istemci bağlamı."})
	}
	email := strings.TrimSpace(in.Email)
	if !emailnorm.ValidFormat(email) || in.Password == "" {
		return TokenResult{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "email", Message: "Geçerli kimlik bilgileri girin."})
	}
	normalized := emailnorm.Normalize(email)
	now := s.clock.Now()

	user, err := s.users.FindByNormalizedEmail(ctx, normalized)
	if err != nil {
		if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
			_, _ = s.hasher.Verify(s.dummyPasswordHash, in.Password)
			s.bestEffortEvent(ctx, domainauth.SecurityEvent{
				ID:            uuid.New(),
				EventType:     domainauth.EventLoginFailure,
				ClientContext: &in.ClientContext,
				Metadata:      map[string]any{"reason": "unknown_user"},
				CreatedAt:     now,
			})
			return TokenResult{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, genericAuthFailure)
		}
		return TokenResult{}, err
	}

	ok, verifyErr := s.hasher.Verify(user.PasswordHash, in.Password)
	if verifyErr != nil || !ok {
		_ = s.withTx(ctx, func(ctx context.Context, users UserRepository, _ SessionRepository) error {
			return users.RecordFailedLogin(ctx, user.ID, now)
		})
		uid := user.ID
		s.bestEffortEvent(ctx, domainauth.SecurityEvent{
			ID:            uuid.New(),
			SubjectUserID: &uid,
			EventType:     domainauth.EventLoginFailure,
			ClientContext: &in.ClientContext,
			Metadata:      map[string]any{"reason": "bad_password"},
			CreatedAt:     now,
		})
		return TokenResult{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, genericAuthFailure)
	}

	if !user.IsActive() {
		return TokenResult{}, apperr.Forbidden(apperr.CodeAccountInactive, "Hesap aktif değil.")
	}
	if user.IsLocked(now) {
		return TokenResult{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, genericAuthFailure)
	}
	if in.ClientContext == domainauth.ClientContextAdminBO && user.Role != domainuser.RoleAdmin {
		uid := user.ID
		s.bestEffortEvent(ctx, domainauth.SecurityEvent{
			ID:            uuid.New(),
			SubjectUserID: &uid,
			ActorUserID:   &uid,
			EventType:     domainauth.EventBOContextRejected,
			ClientContext: &in.ClientContext,
			Metadata:      map[string]any{},
			CreatedAt:     now,
		})
		return TokenResult{}, apperr.Forbidden(apperr.CodeForbidden, "Bu işlem için yetkiniz yok.")
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
		Metadata:      map[string]any{},
		CreatedAt:     now,
	})
	return result, nil
}

// Refresh implements AUTH-05 with atomic rotation.
func (s *Service) Refresh(ctx context.Context, in RefreshInput) (TokenResult, error) {
	if !in.ClientContext.Valid() {
		return TokenResult{}, apperr.Unauthenticated(apperr.CodeTokenInvalid, "Oturum yenilenemedi.")
	}
	if strings.TrimSpace(in.RefreshToken) == "" {
		return TokenResult{}, apperr.Unauthenticated(apperr.CodeTokenInvalid, "Oturum yenilenemedi.")
	}
	hash := token.HashRefreshToken(in.RefreshToken)
	now := s.clock.Now()

	var result TokenResult
	err := s.withTx(ctx, func(ctx context.Context, users UserRepository, sessions SessionRepository) error {
		sess, err := sessions.FindSessionByRefreshHashForUpdate(ctx, hash)
		if err != nil {
			if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
				return apperr.Unauthenticated(apperr.CodeTokenInvalid, "Oturum yenilenemedi.")
			}
			return err
		}
		if sess.ClientContext != in.ClientContext {
			return apperr.Unauthenticated(apperr.CodeTokenInvalid, "Oturum yenilenemedi.")
		}
		if sess.IsRevoked() {
			if err := sessions.RevokeFamily(ctx, sess.FamilyID, now, "REFRESH_REPLAY"); err != nil {
				return err
			}
			uid := sess.UserID
			ctxCopy := sess.ClientContext
			if err := sessions.InsertSecurityEvent(ctx, domainauth.SecurityEvent{
				ID:            uuid.New(),
				SubjectUserID: &uid,
				ActorUserID:   &uid,
				EventType:     domainauth.EventRefreshReplayDetected,
				ClientContext: &ctxCopy,
				Metadata:      map[string]any{},
				CreatedAt:     now,
			}); err != nil {
				return err
			}
			return apperr.Unauthenticated(apperr.CodeRefreshReplayDetected, "Yenileme jetonu yeniden kullanıldı.")
		}
		if sess.IsExpired(now) {
			_ = sessions.RevokeSession(ctx, sess.ID, now, "EXPIRED", nil)
			return apperr.Unauthenticated(apperr.CodeTokenInvalid, "Oturum yenilenemedi.")
		}

		user, err := users.FindByID(ctx, sess.UserID)
		if err != nil {
			return err
		}
		if !user.IsActive() {
			_ = sessions.RevokeFamily(ctx, sess.FamilyID, now, "ACCOUNT_INACTIVE")
			return apperr.Forbidden(apperr.CodeAccountInactive, "Hesap aktif değil.")
		}

		refreshPlain, refreshHash, err := token.NewRefreshToken()
		if err != nil {
			return apperr.Internal(err)
		}
		_, idleTTL := s.tokens.RefreshTTLs()
		newSession := domainauth.Session{
			ID:                uuid.New(),
			UserID:            user.ID,
			ClientContext:     sess.ClientContext,
			RefreshTokenHash:  refreshHash,
			FamilyID:          sess.FamilyID,
			AbsoluteExpiresAt: sess.AbsoluteExpiresAt,
			IdleExpiresAt:     now.Add(idleTTL),
			CreatedAt:         now,
			LastUsedAt:        now,
			UserAgent:         truncateUA(in.UserAgent),
			IPHash:            hashIP(in.ClientIP),
		}
		if !now.Before(newSession.AbsoluteExpiresAt) {
			return apperr.Unauthenticated(apperr.CodeTokenInvalid, "Oturum yenilenemedi.")
		}
		if newSession.IdleExpiresAt.After(newSession.AbsoluteExpiresAt) {
			newSession.IdleExpiresAt = newSession.AbsoluteExpiresAt
		}

		if err := sessions.CreateSession(ctx, newSession); err != nil {
			return err
		}
		if err := sessions.RevokeSession(ctx, sess.ID, now, "ROTATED", &newSession.ID); err != nil {
			return err
		}
		access, _, err := s.tokens.IssueAccessToken(domainauth.Principal{
			UserID:        user.ID,
			SessionID:     newSession.ID,
			Role:          string(user.Role),
			ClientContext: newSession.ClientContext,
			SecurityStamp: user.SecurityStamp,
		})
		if err != nil {
			return apperr.Internal(err)
		}
		result = TokenResult{
			AccessToken:   access,
			RefreshToken:  refreshPlain,
			TokenType:     "Bearer",
			ExpiresIn:     s.tokens.AccessTTLSeconds(),
			ClientContext: newSession.ClientContext,
		}
		return nil
	})
	if err != nil {
		return TokenResult{}, err
	}
	return result, nil
}

// Logout implements AUTH-06 (idempotent).
func (s *Service) Logout(ctx context.Context, in LogoutInput) (LogoutResult, error) {
	principal, err := s.tokens.ParseAccessToken(strings.TrimSpace(in.AccessToken))
	if err != nil {
		return LogoutResult{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli.")
	}
	now := s.clock.Now()

	user, err := s.users.FindByID(ctx, principal.UserID)
	if err != nil {
		if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
			return LogoutResult{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli.")
		}
		return LogoutResult{}, err
	}
	if !user.IsActive() {
		return LogoutResult{}, apperr.Forbidden(apperr.CodeAccountInactive, "Hesap aktif değil.")
	}
	if user.SecurityStamp != principal.SecurityStamp {
		return LogoutResult{}, apperr.Unauthenticated(apperr.CodeSessionRevoked, "Oturum geçersiz.")
	}

	err = s.withTx(ctx, func(ctx context.Context, _ UserRepository, sessions SessionRepository) error {
		sess, err := sessions.FindSessionByIDForUpdate(ctx, principal.SessionID)
		if err != nil {
			if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
				return apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli.")
			}
			return err
		}
		if sess.UserID != principal.UserID {
			return apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli.")
		}
		if sess.IsRevoked() {
			return nil
		}
		return sessions.RevokeSession(ctx, sess.ID, now, "LOGOUT", nil)
	})
	if err != nil {
		return LogoutResult{}, err
	}

	uid := user.ID
	ctxCopy := principal.ClientContext
	s.bestEffortEvent(ctx, domainauth.SecurityEvent{
		ID:            uuid.New(),
		SubjectUserID: &uid,
		ActorUserID:   &uid,
		EventType:     domainauth.EventLogout,
		ClientContext: &ctxCopy,
		Metadata:      map[string]any{},
		CreatedAt:     now,
	})
	return LogoutResult{Message: logoutSuccessMessage}, nil
}

const (
	logoutAllSuccessMessage     = "Tüm oturumlar kapatıldı."
	revokeSessionSuccessMessage = "Oturum iptal edildi."
	minSessionPageLimit         = 1
	maxSessionPageLimit         = 100
	defaultSessionPageLimit     = 20
)

// ProfileView is ACCOUNT-01/02 output without secrets.
type ProfileView struct {
	ID            uuid.UUID
	Email         string
	EmailVerified bool
	FirstName     string
	LastName      string
	Phone         *string
	Role          domainuser.Role
	Status        domainuser.Status
}

// SessionView is AUTH-08 list item without hashes/tokens.
type SessionView struct {
	ID            uuid.UUID
	ClientContext domainauth.ClientContext
	CreatedAt     time.Time
	LastUsedAt    time.Time
	RevokedAt     *time.Time
	IsCurrent     bool
}

// SessionListResult is AUTH-08 output.
type SessionListResult struct {
	Items      []SessionView
	NextCursor *string
	HasMore    bool
}

// GetMyProfile implements ACCOUNT-01.
func (s *Service) GetMyProfile(ctx context.Context, userID uuid.UUID) (ProfileView, error) {
	user, err := s.requireActiveUser(ctx, userID)
	if err != nil {
		return ProfileView{}, err
	}
	return mapProfile(user), nil
}

// UpdateMyProfile implements ACCOUNT-02.
func (s *Service) UpdateMyProfile(ctx context.Context, userID uuid.UUID, patch ProfilePatch) (ProfileView, error) {
	patch, err := normalizeProfilePatch(patch)
	if err != nil {
		return ProfileView{}, err
	}
	now := s.clock.Now()
	var updated domainuser.User
	err = s.withTx(ctx, func(ctx context.Context, users UserRepository, _ SessionRepository) error {
		user, err := users.FindByIDForUpdate(ctx, userID)
		if err != nil {
			if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
				return apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli.")
			}
			return err
		}
		if !user.IsActive() {
			return apperr.Forbidden(apperr.CodeAccountInactive, "Hesap aktif değil.")
		}
		u, err := users.UpdateProfile(ctx, userID, patch, now)
		if err != nil {
			return err
		}
		updated = u
		return nil
	})
	if err != nil {
		return ProfileView{}, err
	}
	return mapProfile(updated), nil
}

// LogoutAllSessions implements AUTH-07 (idempotent).
func (s *Service) LogoutAllSessions(ctx context.Context, principal domainauth.Principal) (LogoutResult, error) {
	if _, err := s.requireActiveUser(ctx, principal.UserID); err != nil {
		return LogoutResult{}, err
	}
	now := s.clock.Now()
	err := s.withTx(ctx, func(ctx context.Context, _ UserRepository, sessions SessionRepository) error {
		return sessions.RevokeAllSessionsForUser(ctx, principal.UserID, now, "LOGOUT_ALL")
	})
	if err != nil {
		return LogoutResult{}, err
	}
	uid := principal.UserID
	ctxCopy := principal.ClientContext
	s.bestEffortEvent(ctx, domainauth.SecurityEvent{
		ID:            uuid.New(),
		SubjectUserID: &uid,
		ActorUserID:   &uid,
		EventType:     domainauth.EventAllSessionsRevoked,
		ClientContext: &ctxCopy,
		Metadata:      map[string]any{},
		CreatedAt:     now,
	})
	return LogoutResult{Message: logoutAllSuccessMessage}, nil
}

// ListMySessions implements AUTH-08.
func (s *Service) ListMySessions(ctx context.Context, userID, currentSessionID uuid.UUID, cursor *string, limit *int) (SessionListResult, error) {
	if _, err := s.requireActiveUser(ctx, userID); err != nil {
		return SessionListResult{}, err
	}
	lim, err := resolveSessionLimit(limit)
	if err != nil {
		return SessionListResult{}, err
	}
	var afterLastUsed *time.Time
	var afterID *uuid.UUID
	if cursor != nil && strings.TrimSpace(*cursor) != "" {
		t, id, err := decodeSessionCursor(strings.TrimSpace(*cursor))
		if err != nil {
			return SessionListResult{}, apperr.BadRequest(apperr.CodeValidation, "Geçersiz cursor.")
		}
		afterLastUsed = &t
		afterID = &id
	}
	rows, err := s.sessions.ListSessionsByUserID(ctx, userID, afterLastUsed, afterID, lim+1)
	if err != nil {
		return SessionListResult{}, err
	}
	hasMore := len(rows) > lim
	if hasMore {
		rows = rows[:lim]
	}
	items := make([]SessionView, 0, len(rows))
	for _, sess := range rows {
		items = append(items, SessionView{
			ID:            sess.ID,
			ClientContext: sess.ClientContext,
			CreatedAt:     sess.CreatedAt,
			LastUsedAt:    sess.LastUsedAt,
			RevokedAt:     sess.RevokedAt,
			IsCurrent:     sess.ID == currentSessionID,
		})
	}
	var next *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		c := encodeSessionCursor(last.LastUsedAt, last.ID)
		next = &c
	}
	return SessionListResult{Items: items, NextCursor: next, HasMore: hasMore}, nil
}

// RevokeMySession implements AUTH-09 (idempotent).
func (s *Service) RevokeMySession(ctx context.Context, principal domainauth.Principal, sessionID uuid.UUID) (LogoutResult, error) {
	if _, err := s.requireActiveUser(ctx, principal.UserID); err != nil {
		return LogoutResult{}, err
	}
	now := s.clock.Now()
	var newlyRevoked bool
	err := s.withTx(ctx, func(ctx context.Context, _ UserRepository, sessions SessionRepository) error {
		_, newly, err := sessions.RevokeSessionForUser(ctx, principal.UserID, sessionID, now, "USER_REVOKE")
		if err != nil {
			return err
		}
		newlyRevoked = newly
		return nil
	})
	if err != nil {
		return LogoutResult{}, err
	}
	if newlyRevoked {
		uid := principal.UserID
		ctxCopy := principal.ClientContext
		s.bestEffortEvent(ctx, domainauth.SecurityEvent{
			ID:            uuid.New(),
			SubjectUserID: &uid,
			ActorUserID:   &uid,
			EventType:     domainauth.EventSessionRevoked,
			ClientContext: &ctxCopy,
			Metadata:      map[string]any{},
			CreatedAt:     now,
		})
	}
	return LogoutResult{Message: revokeSessionSuccessMessage}, nil
}

func (s *Service) requireActiveUser(ctx context.Context, userID uuid.UUID) (domainuser.User, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
			return domainuser.User{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli.")
		}
		return domainuser.User{}, err
	}
	if !user.IsActive() {
		return domainuser.User{}, apperr.Forbidden(apperr.CodeAccountInactive, "Hesap aktif değil.")
	}
	return user, nil
}

func mapProfile(u domainuser.User) ProfileView {
	return ProfileView{
		ID:            u.ID,
		Email:         u.Email,
		EmailVerified: u.EmailVerifiedAt != nil,
		FirstName:     u.FirstName,
		LastName:      u.LastName,
		Phone:         u.Phone,
		Role:          u.Role,
		Status:        u.Status,
	}
}

func normalizeProfilePatch(patch ProfilePatch) (ProfilePatch, error) {
	if patch.FirstName == nil && patch.LastName == nil && !patch.PhoneSet {
		return patch, nil
	}
	if patch.FirstName != nil {
		v := strings.TrimSpace(*patch.FirstName)
		if v == "" || firstNameTooLong(v) {
			return ProfilePatch{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "firstName", Message: "Ad geçersiz."})
		}
		patch.FirstName = &v
	}
	if patch.LastName != nil {
		v := strings.TrimSpace(*patch.LastName)
		if v == "" || lastNameTooLong(v) {
			return ProfilePatch{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "lastName", Message: "Soyad geçersiz."})
		}
		patch.LastName = &v
	}
	if patch.PhoneSet && patch.PhoneValue != nil {
		normalized, err := phone.NormalizeOptional(patch.PhoneValue)
		if err != nil {
			return ProfilePatch{}, err
		}
		patch.PhoneValue = normalized
	}
	return patch, nil
}

func resolveSessionLimit(limit *int) (int, error) {
	if limit == nil {
		return defaultSessionPageLimit, nil
	}
	if *limit < minSessionPageLimit || *limit > maxSessionPageLimit {
		return 0, apperr.BadRequest(apperr.CodeValidation, "limit 1 ile 100 arasında olmalıdır")
	}
	return *limit, nil
}

func encodeSessionCursor(lastUsed time.Time, id uuid.UUID) string {
	raw := lastUsed.UTC().Format(time.RFC3339Nano) + "|" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeSessionCursor(cursor string) (time.Time, uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, fmt.Errorf("bad cursor")
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	return t, id, nil
}

// AuthenticateAccessToken validates an access token for middleware/handler use.
func (s *Service) AuthenticateAccessToken(ctx context.Context, accessToken string) (domainauth.Principal, error) {
	principal, err := s.tokens.ParseAccessToken(strings.TrimSpace(accessToken))
	if err != nil {
		return domainauth.Principal{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli.")
	}
	user, err := s.users.FindByID(ctx, principal.UserID)
	if err != nil {
		if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
			return domainauth.Principal{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli.")
		}
		return domainauth.Principal{}, err
	}
	if !user.IsActive() {
		return domainauth.Principal{}, apperr.Forbidden(apperr.CodeAccountInactive, "Hesap aktif değil.")
	}
	if user.SecurityStamp != principal.SecurityStamp {
		return domainauth.Principal{}, apperr.Unauthenticated(apperr.CodeSessionRevoked, "Oturum geçersiz.")
	}
	sess, err := s.sessions.FindSessionByID(ctx, principal.SessionID)
	if err != nil {
		if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
			return domainauth.Principal{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli.")
		}
		return domainauth.Principal{}, err
	}
	if sess.UserID != principal.UserID || sess.IsRevoked() {
		return domainauth.Principal{}, apperr.Unauthenticated(apperr.CodeSessionRevoked, "Oturum geçersiz.")
	}
	if sess.IsExpired(s.clock.Now()) {
		return domainauth.Principal{}, apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli.")
	}
	return principal, nil
}

func (s *Service) createSessionTokens(
	ctx context.Context,
	sessions SessionRepository,
	user domainuser.User,
	clientContext domainauth.ClientContext,
	userAgent, clientIP string,
	now time.Time,
	familyID uuid.UUID,
) (TokenResult, error) {
	refreshPlain, refreshHash, err := token.NewRefreshToken()
	if err != nil {
		return TokenResult{}, apperr.Internal(err)
	}
	absTTL, idleTTL := s.tokens.RefreshTTLs()
	session := domainauth.Session{
		ID:                uuid.New(),
		UserID:            user.ID,
		ClientContext:     clientContext,
		RefreshTokenHash:  refreshHash,
		FamilyID:          familyID,
		AbsoluteExpiresAt: now.Add(absTTL),
		IdleExpiresAt:     now.Add(idleTTL),
		CreatedAt:         now,
		LastUsedAt:        now,
		UserAgent:         truncateUA(userAgent),
		IPHash:            hashIP(clientIP),
	}
	if session.IdleExpiresAt.After(session.AbsoluteExpiresAt) {
		session.IdleExpiresAt = session.AbsoluteExpiresAt
	}
	if err := sessions.CreateSession(ctx, session); err != nil {
		return TokenResult{}, err
	}
	access, _, err := s.tokens.IssueAccessToken(domainauth.Principal{
		UserID:        user.ID,
		SessionID:     session.ID,
		Role:          string(user.Role),
		ClientContext: clientContext,
		SecurityStamp: user.SecurityStamp,
	})
	if err != nil {
		return TokenResult{}, apperr.Internal(err)
	}
	return TokenResult{
		AccessToken:   access,
		RefreshToken:  refreshPlain,
		TokenType:     "Bearer",
		ExpiresIn:     s.tokens.AccessTTLSeconds(),
		ClientContext: clientContext,
	}, nil
}

func (s *Service) withTx(ctx context.Context, fn func(context.Context, UserRepository, SessionRepository) error) error {
	tx, err := s.sessions.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(ctx, s.userTx(tx), s.sessions.WithTx(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		if errors.Is(err, pgx.ErrTxClosed) {
			return apperr.Internal(err)
		}
		return apperr.Internal(fmt.Errorf("commit tx: %w", err))
	}
	return nil
}

func (s *Service) bestEffortEvent(ctx context.Context, e domainauth.SecurityEvent) {
	if e.Metadata == nil {
		e.Metadata = map[string]any{}
	}
	_ = s.sessions.InsertSecurityEvent(ctx, e)
}

func hashIP(ip string) *string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(ip))
	h := hex.EncodeToString(sum[:])
	return &h
}

func truncateUA(ua string) *string {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return nil
	}
	if len(ua) > 512 {
		ua = ua[:512]
	}
	return &ua
}

func firstNameTooLong(v string) bool { return len([]rune(v)) > 100 }
func lastNameTooLong(v string) bool  { return len([]rune(v)) > 100 }
