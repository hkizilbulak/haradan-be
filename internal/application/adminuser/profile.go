package adminuser

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	"github.com/hkizilbulak/haradan-be/internal/platform/security/emailnorm"
	"github.com/hkizilbulak/haradan-be/internal/platform/security/phone"
)

type UpdateProfileInput struct {
	ActorUserID       uuid.UUID
	UserID            uuid.UUID
	ExpectedUpdatedAt time.Time
	FirstName         string
	LastName          string
	Phone             *string
	PhoneSet          bool
}

type RequestEmailChangeInput struct {
	ActorUserID uuid.UUID
	UserID      uuid.UUID
	NewEmail    string
}

// UpdateProfile updates firstName/lastName/phone under row lock + updatedAt CAS.
// Email is never changed here.
func (s *Service) UpdateProfile(ctx context.Context, in UpdateProfileInput) (Detail, error) {
	firstName := strings.TrimSpace(in.FirstName)
	lastName := strings.TrimSpace(in.LastName)
	if firstName == "" || lastName == "" {
		return Detail{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "name", Message: "Ad ve soyad zorunludur."})
	}
	var phoneValue *string
	if in.PhoneSet {
		normalized, err := phone.NormalizeOptional(in.Phone)
		if err != nil {
			return Detail{}, err
		}
		phoneValue = normalized
	}
	now := s.clock.Now()
	var out Detail
	err := s.withTx(ctx, func(repo Repository) error {
		user, err := repo.FindUserForUpdate(ctx, in.UserID)
		if err != nil {
			return err
		}
		if !user.UpdatedAt.Equal(in.ExpectedUpdatedAt.UTC()) {
			return apperr.Conflict("Kullanıcı başka bir işlem tarafından güncellendi.")
		}
		if !in.PhoneSet {
			phoneValue = user.Phone
		}
		updated, err := repo.UpdateProfile(ctx, user.ID, firstName, lastName, phoneValue, now)
		if err != nil {
			return err
		}
		sessions, err := repo.ActiveSessionCount(ctx, updated.ID, now)
		if err != nil {
			return err
		}
		out = Detail{User: updated, ActiveSessionCount: sessions}
		return nil
	})
	return out, err
}

// RequestEmailChange directly updates the target user's email without requiring email verification.
func (s *Service) RequestEmailChange(ctx context.Context, in RequestEmailChangeInput) error {
	email := strings.TrimSpace(in.NewEmail)
	if !emailnorm.ValidFormat(email) {
		return apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "newEmail", Message: "Geçerli bir e-posta girin."})
	}
	normalized := emailnorm.Normalize(email)
	now := s.clock.Now()

	user, err := s.repo.FindUser(ctx, in.UserID)
	if err != nil {
		return err
	}
	if user.Status != domainuser.StatusActive {
		return apperr.Conflict("Yalnız aktif kullanıcılar için e-posta değişikliği başlatılabilir.")
	}
	if user.EmailNormalized == normalized {
		return apperr.Conflict("Yeni e-posta mevcut adresle aynı.")
	}
	if existing, err := s.repo.FindUserByNormalizedEmail(ctx, normalized); err == nil && existing.ID != uuid.Nil {
		return apperr.Conflict("Bu e-posta adresi zaten kayıtlı.")
	} else if err != nil {
		if ae, ok := apperr.As(err); !ok || ae.Kind != apperr.KindNotFound {
			return err
		}
	}

	return s.withTx(ctx, func(repo Repository) error {
		locked, err := repo.FindUserForUpdate(ctx, user.ID)
		if err != nil {
			return err
		}
		if locked.Status != domainuser.StatusActive {
			return apperr.Conflict("Yalnız aktif kullanıcılar için e-posta değişikliği başlatılabilir.")
		}
		if err := repo.InvalidateActiveOneTimeCredentials(ctx, locked.ID, domainauth.PurposeEmailChangeVerification, now); err != nil {
			return err
		}
		if _, err := repo.UpdateEmail(ctx, locked.ID, email, normalized, uuid.New(), now); err != nil {
			return err
		}
		return repo.InsertSecurityEvent(ctx, domainauth.SecurityEvent{
			ID: uuid.New(), SubjectUserID: &locked.ID, ActorUserID: &in.ActorUserID,
			EventType: domainauth.EventEmailChange,
			Metadata:  map[string]any{"reason": "ADMIN_EMAIL_CHANGE_DIRECT", "newEmail": email, "previousEmail": locked.Email},
			CreatedAt: now,
		})
	})
}
