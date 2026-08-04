package packaging

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

const (
	minHistoryLimit     = 1
	maxHistoryLimit     = 100
	defaultHistoryLimit = 20

	packageNotFoundMessage      = "Paket bulunamadı."
	advertNotFoundMessage       = "İlan bulunamadı."
	forbiddenMessage            = "Bu işlem için yetkiniz yok."
	packageInactiveMessage      = "Pasif pakete atama yapılamaz."
	invalidDateRangeMessage     = "Başlangıç ve bitiş tarihleri geçersiz."
	advertTerminalMessage       = "Bu ilan durumunda paket ataması yapılamaz."
	urgentAdvertStateMessage    = "Bu ilan durumunda URGENT açılamaz."
	urgentRequiresAdvancedMsg   = "URGENT yalnız ADVANCED paket ile açılabilir."
	packageLossUrgentReason     = "PACKAGE_ASSIGNMENT_CHANGED"
	packageUrgentDisabledReason = "PACKAGE_URGENT_DISABLED"
	stalePackageVersionMessage  = "Paket başka bir işlem tarafından güncellendi."
	invalidCursorMessage        = "Geçersiz sayfalama imleci."
	assignmentNotFoundMessage   = "Aktif paket ataması bulunamadı."
)

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// Service implements package catalog, assignment, and URGENT use cases.
type Service struct {
	packages    PackageRepository
	assignments AssignmentRepository
	features    FeatureRepository
	adverts     AdvertReader
	users       UserReader
	clock       Clock
}

// Config wires packaging application dependencies.
type Config struct {
	Packages    PackageRepository
	Assignments AssignmentRepository
	Features    FeatureRepository
	Adverts     AdvertReader
	Users       UserReader
	Clock       Clock
}

// NewService constructs the packaging application service.
func NewService(cfg Config) (*Service, error) {
	if cfg.Packages == nil || cfg.Assignments == nil || cfg.Features == nil ||
		cfg.Adverts == nil || cfg.Users == nil {
		return nil, fmt.Errorf("packaging service dependencies are required")
	}
	clock := cfg.Clock
	if clock == nil {
		clock = systemClock{}
	}
	return &Service{
		packages:    cfg.Packages,
		assignments: cfg.Assignments,
		features:    cfg.Features,
		adverts:     cfg.Adverts,
		users:       cfg.Users,
		clock:       clock,
	}, nil
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// AssignAdvertPackageInput is the admin package assignment request.
type AssignAdvertPackageInput struct {
	ActorUserID uuid.UUID
	AdvertID    uuid.UUID
	PackageCode domainpackaging.PackageCode
	StartsAt    *time.Time
	EndsAt      *time.Time
	Reason      *string
}

// CancelAdvertPackageInput cancels the active assignment for an advert.
type CancelAdvertPackageInput struct {
	ActorUserID uuid.UUID
	AdvertID    uuid.UUID
	Reason      *string
}

// UpdatePackageInput is the admin package catalog patch.
type UpdatePackageInput struct {
	ActorUserID             uuid.UUID
	Code                    domainpackaging.PackageCode
	ExpectedVersion         int
	DisplayName             *string
	Description             *string
	DescriptionSet          bool
	BadgeText               *string
	BadgeTextSet            bool
	Benefits                *[]string
	DisplayPriceAmountMinor *int64
	DisplayPriceSet         bool
	CurrencyCode            *string
	DefaultDurationDays     *int
	DefaultDurationSet      bool
	AllowsUrgent            *bool
	ShowcaseEligible        *bool
	SearchPriority          *int
	IsActive                *bool
	SortOrder               *int
}

// AssignmentView pairs an assignment with its package snapshot.
type AssignmentView struct {
	Assignment domainpackaging.AdvertPackageAssignment
	Package    domainpackaging.Package
}

// HistoryResult is paginated assignment history.
type HistoryResult struct {
	Items      []AssignmentView
	HasMore    bool
	NextCursor *string
}

// ListPackages returns packages ordered by sort_order ASC, code ASC.
func (s *Service) ListPackages(ctx context.Context, includeInactive bool) ([]domainpackaging.Package, error) {
	return s.packages.List(ctx, includeInactive)
}

// ListAdminPackages returns all packages for an ACTIVE ADMIN.
func (s *Service) ListAdminPackages(ctx context.Context, actorUserID uuid.UUID) ([]domainpackaging.Package, error) {
	if err := s.requireAdmin(ctx, actorUserID); err != nil {
		return nil, err
	}
	return s.packages.List(ctx, true)
}

// GetPackageByCode returns a package by code for an ACTIVE ADMIN.
func (s *Service) GetPackageByCode(
	ctx context.Context,
	actorUserID uuid.UUID,
	code domainpackaging.PackageCode,
) (domainpackaging.Package, error) {
	if err := s.requireAdmin(ctx, actorUserID); err != nil {
		return domainpackaging.Package{}, err
	}
	if !code.Valid() {
		return domainpackaging.Package{}, apperr.Validation("Geçersiz paket kodu.")
	}
	return s.packages.FindByCode(ctx, code)
}

// UpdatePackage applies an optimistic catalog update for an ACTIVE ADMIN.
func (s *Service) UpdatePackage(ctx context.Context, in UpdatePackageInput) (domainpackaging.Package, error) {
	if err := s.requireAdmin(ctx, in.ActorUserID); err != nil {
		return domainpackaging.Package{}, err
	}
	if !in.Code.Valid() {
		return domainpackaging.Package{}, apperr.Validation("Geçersiz paket kodu.")
	}
	if in.ExpectedVersion < 1 {
		return domainpackaging.Package{}, apperr.Validation("Geçersiz sürüm.")
	}

	var out domainpackaging.Package
	err := s.withPackageTx(ctx, func(ctx context.Context, packages PackageRepository, features FeatureRepository) error {
		current, err := packages.LockByCode(ctx, in.Code)
		if err != nil {
			return err
		}
		updated, err := applyPackagePatch(current, in)
		if err != nil {
			return err
		}
		updated.UpdatedAt = s.clock.Now().UTC()
		saved, err := packages.UpdateOptimistic(ctx, updated, in.ExpectedVersion)
		if err != nil {
			return err
		}
		if current.AllowsUrgent && !saved.AllowsUrgent {
			now := s.clock.Now().UTC()
			if _, err := features.DeactivateActiveUrgentForPackage(
				ctx, saved.ID, now, strPtr(packageUrgentDisabledReason), now,
			); err != nil {
				return err
			}
		}
		out = saved
		return nil
	})
	return out, err
}

// AssignAdvertPackage assigns an active package to an advert (ADMIN only).
func (s *Service) AssignAdvertPackage(ctx context.Context, in AssignAdvertPackageInput) (AssignmentView, error) {
	if err := s.requireAdmin(ctx, in.ActorUserID); err != nil {
		return AssignmentView{}, err
	}
	if !in.PackageCode.Valid() {
		return AssignmentView{}, apperr.Validation("Geçersiz paket kodu.")
	}

	pkg, err := s.packages.FindByCode(ctx, in.PackageCode)
	if err != nil {
		return AssignmentView{}, err
	}
	if !pkg.IsActive {
		return AssignmentView{}, apperr.InvalidState(packageInactiveMessage)
	}

	now := s.clock.Now().UTC()
	startsExplicit := in.StartsAt != nil
	endsExplicit := in.EndsAt != nil
	startsAt, endsAt := resolveAssignmentSchedule(in, pkg, now)
	if !domainpackaging.ValidTimeRange(startsAt, endsAt) {
		return AssignmentView{}, apperr.Validation(invalidDateRangeMessage)
	}

	var out AssignmentView
	err = s.withTx(ctx, func(ctx context.Context, assignments AssignmentRepository, features FeatureRepository) error {
		advert, err := s.adverts.FindByIDForUpdate(ctx, in.AdvertID)
		if err != nil {
			return err
		}
		if err := assertAdvertAssignable(advert); err != nil {
			return err
		}

		current, err := assignments.LockActiveByAdvertID(ctx, in.AdvertID)
		hasCurrent := false
		if err == nil {
			hasCurrent = true
		} else if !isNotFound(err) {
			return err
		}
		if hasCurrent && isIdempotentAssignment(current, pkg.ID, startsAt, endsAt, startsExplicit, endsExplicit) {
			out = AssignmentView{Assignment: current, Package: pkg}
			return nil
		}

		if hasCurrent {
			if err := assignments.MarkSuperseded(ctx, current.ID, now, now); err != nil {
				return err
			}
			if err := deactivateUrgentForPackageLoss(ctx, features, in.AdvertID, now); err != nil {
				return err
			}
		}

		created := domainpackaging.AdvertPackageAssignment{
			ID:               uuid.New(),
			AdvertID:         in.AdvertID,
			PackageID:        pkg.ID,
			Status:           domainpackaging.AssignmentStatusActive,
			StartsAt:         startsAt,
			EndsAt:           endsAt,
			AssignedByUserID: in.ActorUserID,
			AssignedAt:       now,
			Reason:           trimReason(in.Reason),
			Source:           domainpackaging.AssignmentSourceAdmin,
			Version:          1,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := assignments.Create(ctx, created); err != nil {
			return err
		}
		// ADVANCED→non-ADVANCED already deactivated above on supersede.
		// ADVANCED→ADVANCED also deactivated on supersede so URGENT cannot
		// remain linked to a superseded assignment row.
		out = AssignmentView{Assignment: created, Package: pkg}
		return nil
	})
	if err != nil {
		return AssignmentView{}, err
	}
	return out, nil
}

// GetAdvertPackage returns the currently effective ACTIVE assignment, if any.
// Missing assignment yields (nil, nil).
func (s *Service) GetAdvertPackage(ctx context.Context, advertID uuid.UUID) (*AssignmentView, error) {
	now := s.clock.Now().UTC()
	a, err := s.assignments.FindEffectiveActiveByAdvertID(ctx, advertID, now)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	pkg, err := s.packages.FindByID(ctx, a.PackageID)
	if err != nil {
		return nil, err
	}
	return &AssignmentView{Assignment: a, Package: pkg}, nil
}

// GetAdminAdvertPackage returns the effective assignment for an ACTIVE ADMIN or NotFound.
func (s *Service) GetAdminAdvertPackage(
	ctx context.Context,
	actorUserID, advertID uuid.UUID,
) (AssignmentView, error) {
	if err := s.requireAdmin(ctx, actorUserID); err != nil {
		return AssignmentView{}, err
	}
	if _, err := s.adverts.FindByID(ctx, advertID); err != nil {
		return AssignmentView{}, err
	}
	view, err := s.GetAdvertPackage(ctx, advertID)
	if err != nil {
		return AssignmentView{}, err
	}
	if view == nil {
		return AssignmentView{}, apperr.NotFound(assignmentNotFoundMessage)
	}
	return *view, nil
}

// CancelAdvertPackage cancels the ACTIVE assignment (idempotent when none).
func (s *Service) CancelAdvertPackage(ctx context.Context, in CancelAdvertPackageInput) error {
	if err := s.requireAdmin(ctx, in.ActorUserID); err != nil {
		return err
	}
	return s.withTx(ctx, func(ctx context.Context, assignments AssignmentRepository, features FeatureRepository) error {
		if _, err := s.adverts.FindByIDForUpdate(ctx, in.AdvertID); err != nil {
			return err
		}
		current, err := assignments.LockActiveByAdvertID(ctx, in.AdvertID)
		if err != nil {
			if isNotFound(err) {
				return nil
			}
			return err
		}
		now := s.clock.Now().UTC()
		if err := assignments.MarkCancelled(ctx, current.ID, now, now, trimReason(in.Reason)); err != nil {
			return err
		}
		return deactivateUrgentForPackageLoss(ctx, features, in.AdvertID, now)
	})
}

// ListAdvertPackageHistoryInput carries admin history pagination.
type ListAdvertPackageHistoryInput struct {
	ActorUserID uuid.UUID
	AdvertID    uuid.UUID
	Limit       *int
	Cursor      *string
}

// ListAdvertPackageHistory returns assignment history for ADMIN actors.
func (s *Service) ListAdvertPackageHistory(ctx context.Context, in ListAdvertPackageHistoryInput) (HistoryResult, error) {
	if err := s.requireAdmin(ctx, in.ActorUserID); err != nil {
		return HistoryResult{}, err
	}
	if _, err := s.adverts.FindByID(ctx, in.AdvertID); err != nil {
		return HistoryResult{}, err
	}
	lim := defaultHistoryLimit
	if in.Limit != nil {
		lim = normalizeLimit(*in.Limit)
	}
	var afterAssignedAt *time.Time
	var afterID *uuid.UUID
	if in.Cursor != nil && strings.TrimSpace(*in.Cursor) != "" {
		t, id, err := decodeHistoryCursor(*in.Cursor)
		if err != nil {
			return HistoryResult{}, err
		}
		afterAssignedAt = &t
		afterID = &id
	}
	rows, err := s.assignments.ListHistoryByAdvertID(ctx, in.AdvertID, afterAssignedAt, afterID, lim+1)
	if err != nil {
		return HistoryResult{}, err
	}
	hasMore := len(rows) > lim
	if hasMore {
		rows = rows[:lim]
	}
	items := make([]AssignmentView, 0, len(rows))
	for _, row := range rows {
		pkg, err := s.packages.FindByID(ctx, row.PackageID)
		if err != nil {
			return HistoryResult{}, err
		}
		items = append(items, AssignmentView{Assignment: row, Package: pkg})
	}
	out := HistoryResult{Items: items, HasMore: hasMore}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1].Assignment
		cursor := encodeHistoryCursor(last.AssignedAt, last.ID)
		out.NextCursor = &cursor
	}
	return out, nil
}

// ActivateUrgent opens ACTIVE URGENT for an advert (owner or admin).
func (s *Service) ActivateUrgent(ctx context.Context, actorUserID, advertID uuid.UUID) (domainpackaging.AdvertFeatureActivation, error) {
	actor, err := s.users.FindByID(ctx, actorUserID)
	if err != nil {
		return domainpackaging.AdvertFeatureActivation{}, err
	}

	var out domainpackaging.AdvertFeatureActivation
	err = s.withTx(ctx, func(ctx context.Context, assignments AssignmentRepository, features FeatureRepository) error {
		advert, err := s.adverts.FindByIDForUpdate(ctx, advertID)
		if err != nil {
			return err
		}
		if err := assertActorCanManageUrgent(actor, advert); err != nil {
			return err
		}
		if err := assertAdvertAllowsUrgent(advert); err != nil {
			return err
		}

		assignment, err := assignments.LockActiveByAdvertID(ctx, advertID)
		if err != nil {
			if isNotFound(err) {
				return apperr.InvalidState(urgentRequiresAdvancedMsg)
			}
			return err
		}
		if assignment.AdvertID != advertID {
			return apperr.Internal(fmt.Errorf("assignment advert mismatch"))
		}
		now := s.clock.Now().UTC()
		if !assignment.IsEffectiveAt(now) {
			return apperr.InvalidState(urgentRequiresAdvancedMsg)
		}
		pkg, err := s.packages.FindByID(ctx, assignment.PackageID)
		if err != nil {
			return err
		}
		if !pkg.AllowsUrgentFeature() {
			return apperr.InvalidState(urgentRequiresAdvancedMsg)
		}

		// Advert + assignment row locks serialize activation_version generation
		// for the same advert before reading latest / creating ACTIVE.
		existing, err := features.LockActiveByAdvertIDAndCode(ctx, advertID, domainpackaging.FeatureCodeUrgent)
		if err == nil {
			out = existing
			return nil
		}
		if !isNotFound(err) {
			return err
		}

		latest, err := features.FindLatestActivationVersion(ctx, advertID, domainpackaging.FeatureCodeUrgent)
		if err != nil {
			return err
		}
		nextVersion := latest + 1
		if !domainpackaging.ValidActivationVersion(nextVersion) {
			return apperr.Internal(fmt.Errorf("invalid activation version"))
		}

		created := domainpackaging.AdvertFeatureActivation{
			ID:                  uuid.New(),
			AdvertID:            advertID,
			PackageAssignmentID: assignment.ID,
			FeatureCode:         domainpackaging.FeatureCodeUrgent,
			Status:              domainpackaging.FeatureActivationStatusActive,
			ActivatedByUserID:   actorUserID,
			ActivatedAt:         now,
			ActivationVersion:   nextVersion,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		if err := features.Create(ctx, created); err != nil {
			if isConflict(err) {
				existing, findErr := features.FindActiveByAdvertIDAndCode(ctx, advertID, domainpackaging.FeatureCodeUrgent)
				if findErr == nil {
					out = existing
					return nil
				}
			}
			return err
		}
		out = created
		return nil
	})
	return out, err
}

// DeactivateUrgent closes ACTIVE URGENT for an advert (owner or admin).
func (s *Service) DeactivateUrgent(ctx context.Context, actorUserID, advertID uuid.UUID) error {
	actor, err := s.users.FindByID(ctx, actorUserID)
	if err != nil {
		return err
	}

	return s.withTx(ctx, func(ctx context.Context, _ AssignmentRepository, features FeatureRepository) error {
		advert, err := s.adverts.FindByIDForUpdate(ctx, advertID)
		if err != nil {
			return err
		}
		if err := assertActorCanManageUrgent(actor, advert); err != nil {
			return err
		}
		now := s.clock.Now().UTC()
		_, err = features.DeactivateActive(ctx, advertID, domainpackaging.FeatureCodeUrgent, now, nil, now)
		return err
	})
}

// deactivateUrgentForPackageLoss is the internal SYSTEM helper used when an
// assignment is superseded or otherwise loses URGENT eligibility. It must run
// inside an existing packaging transaction via FeatureRepository.WithTx.
func deactivateUrgentForPackageLoss(
	ctx context.Context,
	features FeatureRepository,
	advertID uuid.UUID,
	now time.Time,
) error {
	_, err := features.DeactivateActive(
		ctx, advertID, domainpackaging.FeatureCodeUrgent, now, strPtr(packageLossUrgentReason), now,
	)
	return err
}

func (s *Service) withTx(
	ctx context.Context,
	fn func(context.Context, AssignmentRepository, FeatureRepository) error,
) error {
	tx, err := s.assignments.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	assignments := s.assignments.WithTx(tx)
	features := s.features.WithTx(tx)
	if err := fn(ctx, assignments, features); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return apperr.Internal(fmt.Errorf("commit packaging tx: %w", err))
	}
	return nil
}

func (s *Service) withPackageTx(
	ctx context.Context,
	fn func(context.Context, PackageRepository, FeatureRepository) error,
) error {
	tx, err := s.packages.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	packages := s.packages.WithTx(tx)
	features := s.features.WithTx(tx)
	if err := fn(ctx, packages, features); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return apperr.Internal(fmt.Errorf("commit packaging package tx: %w", err))
	}
	return nil
}

func applyPackagePatch(current domainpackaging.Package, in UpdatePackageInput) (domainpackaging.Package, error) {
	out := current

	if in.DisplayName != nil {
		name := strings.TrimSpace(*in.DisplayName)
		if name == "" {
			return domainpackaging.Package{}, apperr.Validation("Görünen ad boş olamaz.")
		}
		out.DisplayName = name
	}
	if in.DescriptionSet {
		out.Description = trimOptional(in.Description)
	}
	if in.BadgeTextSet {
		out.BadgeText = trimOptional(in.BadgeText)
	}
	if in.Benefits != nil {
		raw, err := json.Marshal(*in.Benefits)
		if err != nil {
			return domainpackaging.Package{}, apperr.Internal(fmt.Errorf("marshal benefits: %w", err))
		}
		out.BenefitsJSON = raw
	}
	if in.DisplayPriceSet {
		out.DisplayPriceAmountMinor = in.DisplayPriceAmountMinor
		if in.DisplayPriceAmountMinor != nil && *in.DisplayPriceAmountMinor < 0 {
			return domainpackaging.Package{}, apperr.Validation("Fiyat negatif olamaz.")
		}
	}
	if in.CurrencyCode != nil {
		code := strings.TrimSpace(*in.CurrencyCode)
		if !currencyPattern.MatchString(code) {
			return domainpackaging.Package{}, apperr.Validation("Geçersiz para birimi.")
		}
		out.CurrencyCode = code
	}
	if in.DefaultDurationSet {
		if in.DefaultDurationDays != nil && *in.DefaultDurationDays <= 0 {
			return domainpackaging.Package{}, apperr.Validation("Varsayılan süre pozitif olmalıdır.")
		}
		out.DefaultDurationDays = in.DefaultDurationDays
	}
	if in.AllowsUrgent != nil {
		out.AllowsUrgent = *in.AllowsUrgent
	}
	if in.ShowcaseEligible != nil {
		out.ShowcaseEligible = *in.ShowcaseEligible
	}
	if in.SearchPriority != nil {
		if *in.SearchPriority < 0 {
			return domainpackaging.Package{}, apperr.Validation("Arama önceliği negatif olamaz.")
		}
		out.SearchPriority = *in.SearchPriority
	}
	if in.IsActive != nil {
		out.IsActive = *in.IsActive
	}
	if in.SortOrder != nil {
		if *in.SortOrder < 0 {
			return domainpackaging.Package{}, apperr.Validation("Sıra negatif olamaz.")
		}
		out.SortOrder = *in.SortOrder
	}
	return out, nil
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

func encodeHistoryCursor(assignedAt time.Time, id uuid.UUID) string {
	raw := assignedAt.UTC().Format(time.RFC3339Nano) + "|" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeHistoryCursor(cursor string) (time.Time, uuid.UUID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, apperr.Validation(invalidCursorMessage, apperr.FieldError{
			Field: "cursor", Message: invalidCursorMessage,
		})
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, apperr.Validation(invalidCursorMessage, apperr.FieldError{
			Field: "cursor", Message: invalidCursorMessage,
		})
	}
	assignedAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, apperr.Validation(invalidCursorMessage, apperr.FieldError{
			Field: "cursor", Message: invalidCursorMessage,
		})
	}
	id, err := uuid.Parse(parts[1])
	if err != nil || id == uuid.Nil {
		return time.Time{}, uuid.Nil, apperr.Validation(invalidCursorMessage, apperr.FieldError{
			Field: "cursor", Message: invalidCursorMessage,
		})
	}
	return assignedAt, id, nil
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

func assertActorCanManageUrgent(actor domainuser.User, advert domainadvert.Advert) error {
	if !actor.IsActive() {
		return apperr.Forbidden(apperr.CodeForbidden, forbiddenMessage)
	}
	if actor.Role == domainuser.RoleAdmin {
		return nil
	}
	if advert.OwnerUserID == actor.ID {
		return nil
	}
	return apperr.Forbidden(apperr.CodeForbidden, forbiddenMessage)
}

func assertAdvertAssignable(a domainadvert.Advert) error {
	if a.IsDeleted() {
		return apperr.InvalidState(advertTerminalMessage)
	}
	switch a.Status {
	case domainadvert.StatusSold, domainadvert.StatusArchived, domainadvert.StatusSuspended:
		return apperr.InvalidState(advertTerminalMessage)
	default:
		return nil
	}
}

func assertAdvertAllowsUrgent(a domainadvert.Advert) error {
	if a.IsDeleted() {
		return apperr.InvalidState(urgentAdvertStateMessage)
	}
	if !domainpackaging.AdvertStatusAllowsUrgent(string(a.Status)) {
		return apperr.InvalidState(urgentAdvertStateMessage)
	}
	return nil
}

// resolveAssignmentSchedule computes starts/ends for a new assignment.
// Explicit nil fields use now / package default duration; they are not fuzzy.
func resolveAssignmentSchedule(
	in AssignAdvertPackageInput,
	pkg domainpackaging.Package,
	now time.Time,
) (time.Time, *time.Time) {
	startsAt := now
	if in.StartsAt != nil {
		startsAt = in.StartsAt.UTC()
	}
	var endsAt *time.Time
	if in.EndsAt != nil {
		t := in.EndsAt.UTC()
		endsAt = &t
	} else if pkg.DefaultDurationDays != nil {
		t := startsAt.AddDate(0, 0, *pkg.DefaultDurationDays)
		endsAt = &t
	}
	return startsAt, endsAt
}

// isIdempotentAssignment decides whether an assign request should reuse the
// current ACTIVE row without creating history.
//
// Rules (no fuzzy clock comparison):
//   - different package → not idempotent
//   - StartsAt and EndsAt both omitted → same package is idempotent
//   - otherwise compare exact computed starts/ends to the current row
func isIdempotentAssignment(
	current domainpackaging.AdvertPackageAssignment,
	packageID uuid.UUID,
	startsAt time.Time,
	endsAt *time.Time,
	startsExplicit, endsExplicit bool,
) bool {
	if current.PackageID != packageID {
		return false
	}
	if !startsExplicit && !endsExplicit {
		return true
	}
	return sameAssignmentTimes(current, startsAt, endsAt)
}

func sameAssignmentTimes(
	current domainpackaging.AdvertPackageAssignment,
	startsAt time.Time,
	endsAt *time.Time,
) bool {
	if !current.StartsAt.Equal(startsAt) {
		return false
	}
	switch {
	case current.EndsAt == nil && endsAt == nil:
		return true
	case current.EndsAt != nil && endsAt != nil:
		return current.EndsAt.Equal(*endsAt)
	default:
		return false
	}
}

func normalizeLimit(limit int) int {
	if limit < minHistoryLimit {
		return defaultHistoryLimit
	}
	if limit > maxHistoryLimit {
		return maxHistoryLimit
	}
	return limit
}

func trimReason(reason *string) *string {
	if reason == nil {
		return nil
	}
	r := *reason
	if r == "" {
		return nil
	}
	return &r
}

func strPtr(s string) *string { return &s }

func isNotFound(err error) bool {
	var ae *apperr.Error
	return errors.As(err, &ae) && ae.Kind == apperr.KindNotFound
}

func isConflict(err error) bool {
	var ae *apperr.Error
	return errors.As(err, &ae) && ae.Kind == apperr.KindConflict
}
