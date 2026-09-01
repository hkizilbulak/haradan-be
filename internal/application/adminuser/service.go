// Package adminuser implements the ADMIN-USER OpenAPI operations.
package adminuser

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	"github.com/hkizilbulak/haradan-be/internal/platform/security/emailnorm"
	"github.com/hkizilbulak/haradan-be/internal/platform/security/phone"
	"github.com/hkizilbulak/haradan-be/internal/platform/security/token"
)

const defaultLimit = 50
const defaultInvitationTTL = 24 * time.Hour

type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type Config struct {
	Repository      Repository
	Hasher          PasswordHasher
	EmailSender     EmailSender
	EmailConfigured bool
	InvitationTTL   time.Duration
	Clock           Clock
}

type Service struct {
	repo            Repository
	hasher          PasswordHasher
	email           EmailSender
	emailConfigured bool
	invitationTTL   time.Duration
	clock           Clock
}

func NewService(cfg Config) (*Service, error) {
	if cfg.Repository == nil {
		return nil, errors.New("admin user repository is required")
	}
	if cfg.Clock == nil {
		cfg.Clock = systemClock{}
	}
	if cfg.InvitationTTL <= 0 {
		cfg.InvitationTTL = defaultInvitationTTL
	}
	if cfg.EmailSender == nil {
		cfg.EmailSender = noopEmail{}
	}
	return &Service{
		repo:            cfg.Repository,
		hasher:          cfg.Hasher,
		email:           cfg.EmailSender,
		emailConfigured: cfg.EmailConfigured,
		invitationTTL:   cfg.InvitationTTL,
		clock:           cfg.Clock,
	}, nil
}

type noopEmail struct{}

func (noopEmail) SendPasswordReset(context.Context, string, string, string) error {
	return apperr.DependencyUnavailable("E-posta sağlayıcı yapılandırılmamış.")
}

func (noopEmail) SendRegistrationVerification(context.Context, string, string, string) error {
	return apperr.DependencyUnavailable("E-posta sağlayıcı yapılandırılmamış.")
}

type ListInput struct {
	Cursor *string
	Limit  *int
	Status *string
	Role   *string
	Query  *string
}

type EventListInput struct {
	Cursor    *string
	Limit     *int
	EventType *string
}

type ListResult struct {
	Items      []domainuser.User
	NextCursor *string
	HasMore    bool
	TotalCount int
}

type EventListResult struct {
	Items      []domainauth.SecurityEvent
	NextCursor *string
	HasMore    bool
}

type Detail struct {
	User               domainuser.User
	ActiveSessionCount int
}

type CreateInput struct {
	ActorUserID uuid.UUID
	Email       string
	FirstName   string
	LastName    string
	Phone       *string
	Role        domainuser.Role
}

type CreateResult struct {
	Detail              Detail
	InvitationEmailSent bool
}

func (s *Service) ListUsers(ctx context.Context, in ListInput) (ListResult, error) {
	limit, err := resolveLimit(in.Limit)
	if err != nil {
		return ListResult{}, err
	}
	status, err := parseStatus(in.Status)
	if err != nil {
		return ListResult{}, err
	}
	role, err := parseRole(in.Role)
	if err != nil {
		return ListResult{}, err
	}
	created, id, err := decodeCursor(in.Cursor)
	if err != nil {
		return ListResult{}, err
	}
	rows, totalCount, err := s.repo.ListUsers(ctx, status, role, strings.TrimSpace(deref(in.Query)), created, id, limit+1)
	if err != nil {
		return ListResult{}, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	result := ListResult{Items: rows, HasMore: hasMore, TotalCount: totalCount}
	if hasMore && len(rows) != 0 {
		cursor := encodeCursor(rows[len(rows)-1].CreatedAt, rows[len(rows)-1].ID)
		result.NextCursor = &cursor
	}
	return result, nil
}

func (s *Service) GetDetail(ctx context.Context, userID uuid.UUID) (Detail, error) {
	return s.repo.GetDetail(ctx, userID, s.clock.Now())
}

// CreateUser creates an ACTIVE admin-provisioned user and issues a PASSWORD_RESET
// invitation. The random password is never returned. Email delivery failure does
// not roll back the account; invitationEmailSent=false. Admin can ResendInvitation later.
func (s *Service) CreateUser(ctx context.Context, in CreateInput) (CreateResult, error) {
	if s.hasher == nil {
		return CreateResult{}, apperr.Internal(errors.New("password hasher is required for admin user create"))
	}
	email := strings.TrimSpace(in.Email)
	if !emailnorm.ValidFormat(email) {
		return CreateResult{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "email", Message: "Geçerli bir e-posta girin."})
	}
	firstName := strings.TrimSpace(in.FirstName)
	lastName := strings.TrimSpace(in.LastName)
	if firstName == "" || lastName == "" {
		return CreateResult{}, apperr.Validation("Geçersiz istek.", apperr.FieldError{Field: "name", Message: "Ad ve soyad zorunludur."})
	}
	if !validRole(in.Role) {
		return CreateResult{}, apperr.BadRequest(apperr.CodeValidation, "Geçersiz kullanıcı rolü.")
	}
	normalizedPhone, err := phone.NormalizeOptional(in.Phone)
	if err != nil {
		return CreateResult{}, err
	}

	rawSecret, err := randomPassword()
	if err != nil {
		return CreateResult{}, apperr.Internal(err)
	}
	passwordHash, err := s.hasher.Hash(rawSecret)
	if err != nil {
		return CreateResult{}, apperr.Internal(err)
	}

	now := s.clock.Now()
	user := domainuser.User{
		ID:              uuid.New(),
		Email:           email,
		EmailNormalized: emailnorm.Normalize(email),
		PasswordHash:    passwordHash,
		Role:            in.Role,
		Status:          domainuser.StatusActive,
		// Invitation/password-setup completion proves mailbox possession.
		EmailVerifiedAt: nil,
		FirstName:       firstName,
		LastName:        lastName,
		Phone:           normalizedPhone,
		SecurityStamp:   uuid.New(),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	var detail Detail
	var plain string
	err = s.withTx(ctx, func(repo Repository) error {
		if err := repo.CreateUser(ctx, user); err != nil {
			return err
		}
		tokenPlain, err := s.issueInvitationCredential(ctx, repo, in.ActorUserID, user, now)
		if err != nil {
			return err
		}
		plain = tokenPlain
		detail = Detail{User: user, ActiveSessionCount: 0}
		return nil
	})
	if err != nil {
		return CreateResult{}, err
	}

	sent := s.trySendInvitation(ctx, user, plain)
	return CreateResult{Detail: detail, InvitationEmailSent: sent}, nil
}

// ResendInvitation re-issues a PASSWORD_RESET OTC and attempts invitation email.
// When the email provider is unconfigured, returns dependency-unavailable without
// rotating credentials.
func (s *Service) ResendInvitation(ctx context.Context, actorID, userID uuid.UUID) (CreateResult, error) {
	if !s.emailConfigured {
		return CreateResult{}, apperr.DependencyUnavailable("E-posta servisi henüz yapılandırılmamış. E-posta gerektirmeyen işlemlere devam edebilirsiniz.")
	}

	now := s.clock.Now()
	var detail Detail
	var plain string
	var user domainuser.User
	err := s.withTx(ctx, func(repo Repository) error {
		var err error
		user, err = repo.FindUserForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		if user.Status != domainuser.StatusActive {
			return apperr.Conflict("Yalnız aktif kullanıcılar için davet yeniden gönderilebilir.")
		}
		tokenPlain, err := s.issueInvitationCredential(ctx, repo, actorID, user, now)
		if err != nil {
			return err
		}
		plain = tokenPlain
		sessions, err := repo.ActiveSessionCount(ctx, user.ID, now)
		if err != nil {
			return err
		}
		detail = Detail{User: user, ActiveSessionCount: sessions}
		return nil
	})
	if err != nil {
		return CreateResult{}, err
	}

	sent := s.trySendInvitation(ctx, user, plain)
	if !sent {
		return CreateResult{Detail: detail, InvitationEmailSent: false}, apperr.DependencyUnavailable("E-posta servisine şu anda ulaşılamıyor. Lütfen daha sonra tekrar deneyin.")
	}
	return CreateResult{Detail: detail, InvitationEmailSent: true}, nil
}

func (s *Service) issueInvitationCredential(
	ctx context.Context,
	repo Repository,
	actorID uuid.UUID,
	user domainuser.User,
	now time.Time,
) (plain string, err error) {
	plain, tokenHash, err := token.NewOpaqueToken()
	if err != nil {
		return "", apperr.Internal(err)
	}
	cred := domainauth.OneTimeCredential{
		ID:        uuid.New(),
		UserID:    user.ID,
		Purpose:   domainauth.PurposePasswordReset,
		TokenHash: tokenHash,
		ExpiresAt: now.Add(s.invitationTTL),
		CreatedAt: now,
	}
	if err := repo.InvalidateActiveOneTimeCredentials(ctx, user.ID, domainauth.PurposePasswordReset, now); err != nil {
		return "", err
	}
	if err := repo.CreateOneTimeCredential(ctx, cred); err != nil {
		return "", err
	}
	if err := repo.InsertSecurityEvent(ctx, domainauth.SecurityEvent{
		ID: uuid.New(), SubjectUserID: &user.ID, ActorUserID: &actorID,
		EventType: domainauth.EventPasswordReset,
		Metadata:  map[string]any{"reason": "ADMIN_INVITATION"},
		CreatedAt: now,
	}); err != nil {
		return "", err
	}
	return plain, nil
}

func (s *Service) trySendInvitation(ctx context.Context, user domainuser.User, plain string) bool {
	if !s.emailConfigured || plain == "" {
		return false
	}
	fullName := strings.TrimSpace(user.FirstName + " " + user.LastName)
	if err := s.email.SendPasswordReset(ctx, user.Email, plain, fullName); err != nil {
		return false
	}
	return true
}

func (s *Service) ChangeRole(ctx context.Context, actorID, userID uuid.UUID, expected, next domainuser.Role) (Detail, error) {
	if !validRole(expected) || !validRole(next) {
		return Detail{}, apperr.BadRequest(apperr.CodeValidation, "Geçersiz kullanıcı rolü.")
	}
	now := s.clock.Now()
	var out Detail
	err := s.withTx(ctx, func(repo Repository) error {
		user, err := repo.FindUserForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		if user.Role != expected {
			return apperr.Conflict("Kullanıcı rolü değişti.")
		}
		if err := s.ensureActiveAdminRemains(ctx, repo, user, next, user.Status); err != nil {
			return err
		}
		user, err = repo.UpdateRole(ctx, userID, next, uuid.New(), now)
		if err != nil {
			return err
		}
		if err := repo.RevokeAllSessions(ctx, userID, now, "ROLE_CHANGE"); err != nil {
			return err
		}
		if err := repo.InsertSecurityEvent(ctx, domainauth.SecurityEvent{
			ID: uuid.New(), SubjectUserID: &userID, ActorUserID: &actorID, EventType: domainauth.EventAllSessionsRevoked,
			Metadata: map[string]any{"reason": "ROLE_CHANGE"}, CreatedAt: now,
		}); err != nil {
			return err
		}
		if err := repo.InsertSecurityEvent(ctx, domainauth.SecurityEvent{
			ID: uuid.New(), SubjectUserID: &userID, ActorUserID: &actorID, EventType: domainauth.EventRoleChange,
			Metadata: map[string]any{"previousRole": string(expected), "newRole": string(next)}, CreatedAt: now,
		}); err != nil {
			return err
		}
		count, err := repo.ActiveSessionCount(ctx, userID, now)
		if err != nil {
			return err
		}
		out = Detail{User: user, ActiveSessionCount: count}
		return nil
	})
	return out, err
}

func (s *Service) ChangeStatus(ctx context.Context, actorID, userID uuid.UUID, expected, next domainuser.Status) (Detail, error) {
	if !validStatus(expected) || !validStatus(next) {
		return Detail{}, apperr.BadRequest(apperr.CodeValidation, "Geçersiz kullanıcı durumu.")
	}
	now := s.clock.Now()
	var out Detail
	err := s.withTx(ctx, func(repo Repository) error {
		user, err := repo.FindUserForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		if user.Status != expected {
			return apperr.Conflict("Kullanıcı durumu değişti.")
		}
		if err := s.ensureActiveAdminRemains(ctx, repo, user, user.Role, next); err != nil {
			return err
		}
		user, err = repo.UpdateStatus(ctx, userID, next, uuid.New(), now)
		if err != nil {
			return err
		}
		if next == domainuser.StatusDisabled || next == domainuser.StatusClosed {
			if err := repo.RevokeAllSessions(ctx, userID, now, string(next)); err != nil {
				return err
			}
			if err := repo.InsertSecurityEvent(ctx, domainauth.SecurityEvent{
				ID: uuid.New(), SubjectUserID: &userID, ActorUserID: &actorID, EventType: domainauth.EventAllSessionsRevoked,
				Metadata: map[string]any{"reason": string(next)}, CreatedAt: now,
			}); err != nil {
				return err
			}
		}
		if err := repo.InsertSecurityEvent(ctx, domainauth.SecurityEvent{
			ID: uuid.New(), SubjectUserID: &userID, ActorUserID: &actorID, EventType: domainauth.EventAccountStatusChange,
			Metadata: map[string]any{"previousStatus": string(expected), "newStatus": string(next)}, CreatedAt: now,
		}); err != nil {
			return err
		}
		count, err := repo.ActiveSessionCount(ctx, userID, now)
		if err != nil {
			return err
		}
		out = Detail{User: user, ActiveSessionCount: count}
		return nil
	})
	return out, err
}

func (s *Service) ListSecurityEvents(ctx context.Context, userID uuid.UUID, in EventListInput) (EventListResult, error) {
	limit, err := resolveLimit(in.Limit)
	if err != nil {
		return EventListResult{}, err
	}
	eventType, err := parseEventType(in.EventType)
	if err != nil {
		return EventListResult{}, err
	}
	created, id, err := decodeCursor(in.Cursor)
	if err != nil {
		return EventListResult{}, err
	}
	if _, err := s.repo.FindUser(ctx, userID); err != nil {
		return EventListResult{}, err
	}
	rows, err := s.repo.ListSecurityEvents(ctx, userID, eventType, created, id, limit+1)
	if err != nil {
		return EventListResult{}, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	result := EventListResult{Items: rows, HasMore: hasMore}
	if hasMore && len(rows) != 0 {
		cursor := encodeCursor(rows[len(rows)-1].CreatedAt, rows[len(rows)-1].ID)
		result.NextCursor = &cursor
	}
	return result, nil
}

// ensureActiveAdminRemains rejects transitions that would leave zero ACTIVE admins.
func (s *Service) ensureActiveAdminRemains(ctx context.Context, repo Repository, current domainuser.User, nextRole domainuser.Role, nextStatus domainuser.Status) error {
	wasActiveAdmin := current.Role == domainuser.RoleAdmin && current.Status == domainuser.StatusActive
	willBeActiveAdmin := nextRole == domainuser.RoleAdmin && nextStatus == domainuser.StatusActive
	if !wasActiveAdmin || willBeActiveAdmin {
		return nil
	}
	if err := repo.LockActiveAdminGuard(ctx); err != nil {
		return err
	}
	count, err := repo.CountActiveAdmins(ctx)
	if err != nil {
		return err
	}
	if count <= 1 {
		return apperr.Conflict("Sistemde en az bir aktif admin kalmalıdır.")
	}
	return nil
}

func (s *Service) withTx(ctx context.Context, fn func(Repository) error) error {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(s.repo.WithTx(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return apperr.Internal(fmt.Errorf("commit admin user transaction: %w", err))
	}
	return nil
}

func randomPassword() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func resolveLimit(value *int) (int, error) {
	if value == nil {
		return defaultLimit, nil
	}
	if *value < 1 || *value > 100 {
		return 0, apperr.BadRequest(apperr.CodeValidation, "limit 1 ile 100 arasında olmalıdır.")
	}
	return *value, nil
}

func parseRole(value *string) (*domainuser.Role, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	v := domainuser.Role(*value)
	if !validRole(v) {
		return nil, apperr.BadRequest(apperr.CodeValidation, "Geçersiz kullanıcı rolü.")
	}
	return &v, nil
}

func parseStatus(value *string) (*domainuser.Status, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	v := domainuser.Status(*value)
	if !validStatus(v) {
		return nil, apperr.BadRequest(apperr.CodeValidation, "Geçersiz kullanıcı durumu.")
	}
	return &v, nil
}

func parseEventType(value *string) (*domainauth.SecurityEventType, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	v := domainauth.SecurityEventType(*value)
	switch v {
	case domainauth.EventLoginSuccess, domainauth.EventLoginFailure, domainauth.EventLogout, domainauth.EventSessionRevoked,
		domainauth.EventAllSessionsRevoked, domainauth.EventRefreshReplayDetected, domainauth.EventPasswordChange,
		domainauth.EventPasswordReset, domainauth.EventEmailVerification, domainauth.EventEmailChange, domainauth.EventRoleChange,
		domainauth.EventAccountStatusChange, domainauth.EventBOContextRejected:
		return &v, nil
	default:
		return nil, apperr.BadRequest(apperr.CodeValidation, "Geçersiz güvenlik olayı türü.")
	}
}

func validRole(value domainuser.Role) bool {
	return value == domainuser.RoleUser || value == domainuser.RoleAdmin
}
func validStatus(value domainuser.Status) bool {
	return value == domainuser.StatusActive || value == domainuser.StatusDisabled || value == domainuser.StatusClosed
}
func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

type cursor struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        uuid.UUID `json:"id"`
}

func encodeCursor(createdAt time.Time, id uuid.UUID) string {
	raw, _ := json.Marshal(cursor{CreatedAt: createdAt.UTC(), ID: id})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(value *string) (*time.Time, *uuid.UUID, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(*value)
	if err != nil {
		return nil, nil, apperr.BadRequest(apperr.CodeValidation, "Geçersiz cursor.")
	}
	var decoded cursor
	if err := json.Unmarshal(raw, &decoded); err != nil || decoded.ID == uuid.Nil || decoded.CreatedAt.IsZero() {
		return nil, nil, apperr.BadRequest(apperr.CodeValidation, "Geçersiz cursor.")
	}
	return &decoded.CreatedAt, &decoded.ID, nil
}
