// Package adminuser implements the ADMIN-USER OpenAPI operations.
package adminuser

import (
	"context"
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
)

const defaultLimit = 50

type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type Config struct {
	Repository Repository
	Clock      Clock
}

type Service struct {
	repo  Repository
	clock Clock
}

func NewService(cfg Config) (*Service, error) {
	if cfg.Repository == nil {
		return nil, errors.New("admin user repository is required")
	}
	if cfg.Clock == nil {
		cfg.Clock = systemClock{}
	}
	return &Service{repo: cfg.Repository, clock: cfg.Clock}, nil
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
	rows, err := s.repo.ListUsers(ctx, status, role, strings.TrimSpace(deref(in.Query)), created, id, limit+1)
	if err != nil {
		return ListResult{}, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	result := ListResult{Items: rows, HasMore: hasMore}
	if hasMore && len(rows) != 0 {
		cursor := encodeCursor(rows[len(rows)-1].CreatedAt, rows[len(rows)-1].ID)
		result.NextCursor = &cursor
	}
	return result, nil
}

func (s *Service) GetDetail(ctx context.Context, userID uuid.UUID) (Detail, error) {
	return s.repo.GetDetail(ctx, userID, s.clock.Now())
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
		user, err = repo.UpdateRole(ctx, userID, next, uuid.New(), now)
		if err != nil {
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
	// The target lookup deliberately precedes event lookup to keep nonexistent
	// users indistinguishable from users with no events.
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
