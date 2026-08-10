package notification

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainnotification "github.com/hkizilbulak/haradan-be/internal/domain/notification"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

const (
	forbiddenMessage        = "Bu işlem için yetkiniz yok."
	templateNotFoundMessage = "Bildirim şablonu bulunamadı."
	staleVersionMessage     = "Bildirim şablonu başka bir işlem tarafından güncellendi."
	invalidRequestMessage   = "Geçersiz istek."
)

// Service implements notification template admin use cases.
type Service struct {
	repo  Repository
	users UserReader
	clock Clock
}

// Config wires notification application dependencies.
type Config struct {
	Repo  Repository
	Users UserReader
	Clock Clock
}

// NewService constructs the notification application service.
func NewService(cfg Config) (*Service, error) {
	if cfg.Repo == nil || cfg.Users == nil {
		return nil, fmt.Errorf("notification service dependencies are required")
	}
	clock := cfg.Clock
	if clock == nil {
		clock = systemClock{}
	}
	return &Service{repo: cfg.Repo, users: cfg.Users, clock: clock}, nil
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// UpdateTemplateInput is a partial optimistic patch (event type immutable).
type UpdateTemplateInput struct {
	ActorUserID             uuid.UUID
	EventType               domainnotification.TemplateEventType
	ExpectedVersion         int
	Name                    *string
	InAppTitleTemplate      *string
	InAppBodyTemplate       *string
	ResendTemplateIDSet     bool
	ResendTemplateID        *string
	EmailSubjectFallbackSet bool
	EmailSubjectFallback    *string
	IsActive                *bool
}

// ListTemplates returns all templates ordered by event_type ASC (ACTIVE ADMIN).
func (s *Service) ListTemplates(ctx context.Context, actorUserID uuid.UUID) ([]domainnotification.NotificationTemplate, error) {
	if err := s.requireAdmin(ctx, actorUserID); err != nil {
		return nil, err
	}
	return s.repo.ListTemplates(ctx)
}

// GetTemplateByEventType returns one template (ACTIVE ADMIN).
func (s *Service) GetTemplateByEventType(
	ctx context.Context,
	actorUserID uuid.UUID,
	eventType domainnotification.TemplateEventType,
) (domainnotification.NotificationTemplate, error) {
	if err := s.requireAdmin(ctx, actorUserID); err != nil {
		return domainnotification.NotificationTemplate{}, err
	}
	if !eventType.Valid() {
		return domainnotification.NotificationTemplate{}, apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field: "eventType", Message: "Olay tipi geçersiz.",
		})
	}
	return s.repo.GetByEventType(ctx, eventType)
}

// UpdateTemplate applies an optimistic partial update (ACTIVE ADMIN).
func (s *Service) UpdateTemplate(ctx context.Context, in UpdateTemplateInput) (domainnotification.NotificationTemplate, error) {
	if err := s.requireAdmin(ctx, in.ActorUserID); err != nil {
		return domainnotification.NotificationTemplate{}, err
	}
	if !in.EventType.Valid() {
		return domainnotification.NotificationTemplate{}, apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field: "eventType", Message: "Olay tipi geçersiz.",
		})
	}
	if in.ExpectedVersion < 1 {
		return domainnotification.NotificationTemplate{}, apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field: "expectedVersion", Message: "Sürüm numarası 1 veya daha büyük olmalıdır.",
		})
	}

	var out domainnotification.NotificationTemplate
	err := s.withTx(ctx, func(ctx context.Context, repo Repository) error {
		current, err := repo.LockByEventType(ctx, in.EventType)
		if err != nil {
			return err
		}
		if current.Version != in.ExpectedVersion {
			return apperr.StaleVersion(staleVersionMessage)
		}

		patched := current
		if in.Name != nil {
			name := strings.TrimSpace(*in.Name)
			if !domainnotification.NonBlankName(name) {
				return apperr.Validation(invalidRequestMessage, apperr.FieldError{
					Field: "name", Message: "Ad zorunludur.",
				})
			}
			patched.Name = name
		}
		if in.InAppTitleTemplate != nil {
			title := strings.TrimSpace(*in.InAppTitleTemplate)
			if !domainnotification.NonBlankTitleTemplate(title) {
				return apperr.Validation(invalidRequestMessage, apperr.FieldError{
					Field: "inAppTitleTemplate", Message: "Başlık şablonu zorunludur.",
				})
			}
			patched.InAppTitleTemplate = title
		}
		if in.InAppBodyTemplate != nil {
			body := strings.TrimSpace(*in.InAppBodyTemplate)
			if !domainnotification.NonBlankBodyTemplate(body) {
				return apperr.Validation(invalidRequestMessage, apperr.FieldError{
					Field: "inAppBodyTemplate", Message: "Gövde şablonu zorunludur.",
				})
			}
			patched.InAppBodyTemplate = body
		}
		if in.ResendTemplateIDSet {
			patched.ResendTemplateID = trimOptional(in.ResendTemplateID)
		}
		if in.EmailSubjectFallbackSet {
			patched.EmailSubjectFallback = trimOptional(in.EmailSubjectFallback)
		}
		if in.IsActive != nil {
			patched.IsActive = *in.IsActive
		}

		now := s.clock.Now().UTC()
		actor := in.ActorUserID
		patched.UpdatedByUserID = &actor
		patched.UpdatedAt = now

		updated, err := repo.UpdateOptimistic(ctx, patched, in.ExpectedVersion)
		if err != nil {
			return err
		}
		out = updated
		return nil
	})
	if err != nil {
		return domainnotification.NotificationTemplate{}, err
	}
	return out, nil
}

func (s *Service) withTx(ctx context.Context, fn func(context.Context, Repository) error) error {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(ctx, s.repo.WithTx(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return apperr.Internal(fmt.Errorf("commit notification tx: %w", err))
	}
	return nil
}

func (s *Service) requireAdmin(ctx context.Context, actorUserID uuid.UUID) error {
	actor, err := s.users.FindByID(ctx, actorUserID)
	if err != nil {
		return err
	}
	if actor.Role != domainuser.RoleAdmin || !actor.IsActive() {
		return apperr.Forbidden(apperr.CodeForbidden, forbiddenMessage)
	}
	return nil
}

func trimOptional(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}
