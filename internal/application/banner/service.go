package banner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainbanner "github.com/hkizilbulak/haradan-be/internal/domain/banner"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

const invalidRequest = "Geçersiz istek."

type Service struct {
	repo  Repository
	media MediaReader
	users UserReader
	clock Clock
}
type Config struct {
	Repo  Repository
	Media MediaReader
	Users UserReader
	Clock Clock
}

func NewService(c Config) (*Service, error) {
	if c.Repo == nil || c.Media == nil || c.Users == nil {
		return nil, errors.New("banner service dependencies are required")
	}
	if c.Clock == nil {
		c.Clock = systemClock{}
	}
	return &Service{c.Repo, c.Media, c.Users, c.Clock}, nil
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type CreateInput struct {
	ActorUserID               uuid.UUID
	Placement                 domainbanner.Placement
	AssetID                   uuid.UUID
	Title, AltText, TargetURL *string
	SortOrder                 *int
}
type UpdateInput struct {
	ActorUserID, BannerID     uuid.UUID
	ExpectedVersion           int
	AssetID                   *uuid.UUID
	Title, AltText, TargetURL *string
	SortOrder                 *int
}
type SetStatusInput struct {
	ActorUserID, BannerID uuid.UUID
	ExpectedVersion       int
	Status                domainbanner.Status
}
type ReorderItem struct {
	ID                         uuid.UUID
	ExpectedVersion, SortOrder int
}

func (s *Service) CreateBanner(ctx context.Context, in CreateInput) (domainbanner.Banner, error) {
	if err := s.requireAdmin(ctx, in.ActorUserID); err != nil {
		return domainbanner.Banner{}, err
	}
	if !in.Placement.Valid() || in.AssetID == uuid.Nil {
		return domainbanner.Banner{}, validation("placement", "Yerleşim ve görsel zorunludur.")
	}
	if in.SortOrder != nil && *in.SortOrder < 0 {
		return domainbanner.Banner{}, validation("sortOrder", "Sıra negatif olamaz.")
	}
	if err := s.validateAsset(ctx, in.AssetID); err != nil {
		return domainbanner.Banner{}, err
	}
	order := 0
	if in.SortOrder != nil {
		order = *in.SortOrder
	}
	now := s.clock.Now().UTC()
	actor := in.ActorUserID
	b := domainbanner.Banner{ID: uuid.New(), Placement: in.Placement, Status: domainbanner.StatusInactive, AssetID: in.AssetID, Title: domainbanner.TrimOptional(in.Title), AltText: domainbanner.TrimOptional(in.AltText), TargetURL: domainbanner.TrimOptional(in.TargetURL), SortOrder: order, Version: 1, CreatedByUserID: &actor, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.Create(ctx, b); err != nil {
		return domainbanner.Banner{}, err
	}
	return b, nil
}
func (s *Service) GetBannerAdminDetail(ctx context.Context, actor, id uuid.UUID) (domainbanner.Banner, error) {
	if err := s.requireAdmin(ctx, actor); err != nil {
		return domainbanner.Banner{}, err
	}
	return s.repo.GetByID(ctx, id)
}
func (s *Service) ListBannersAdmin(ctx context.Context, actor uuid.UUID, f ListFilter) ([]domainbanner.Banner, error) {
	if err := s.requireAdmin(ctx, actor); err != nil {
		return nil, err
	}
	if f.Placement != nil && !f.Placement.Valid() {
		return nil, validation("placement", "Yerleşim geçersiz.")
	}
	if f.Status != nil && !f.Status.Valid() {
		return nil, validation("status", "Durum geçersiz.")
	}
	return s.repo.List(ctx, f)
}
func (s *Service) UpdateBanner(ctx context.Context, in UpdateInput) (domainbanner.Banner, error) {
	if err := s.requireAdmin(ctx, in.ActorUserID); err != nil {
		return domainbanner.Banner{}, err
	}
	if in.ExpectedVersion < 1 {
		return domainbanner.Banner{}, validation("expectedVersion", "Sürüm geçersiz.")
	}
	var out domainbanner.Banner
	err := s.withTx(ctx, func(r Repository) error {
		b, err := r.LockByID(ctx, in.BannerID)
		if err != nil {
			return err
		}
		if b.Version != in.ExpectedVersion {
			return apperr.StaleVersion("Banner başka bir işlem tarafından güncellendi.")
		}
		if in.AssetID != nil {
			if err := s.validateAsset(ctx, *in.AssetID); err != nil {
				return err
			}
			b.AssetID = *in.AssetID
		}
		if in.SortOrder != nil {
			if *in.SortOrder < 0 {
				return validation("sortOrder", "Sıra negatif olamaz.")
			}
			b.SortOrder = *in.SortOrder
		}
		if in.Title != nil {
			b.Title = domainbanner.TrimOptional(in.Title)
		}
		if in.AltText != nil {
			b.AltText = domainbanner.TrimOptional(in.AltText)
		}
		if in.TargetURL != nil {
			b.TargetURL = domainbanner.TrimOptional(in.TargetURL)
		}
		b.UpdatedAt = s.clock.Now().UTC()
		out, err = r.UpdateOptimistic(ctx, b, in.ExpectedVersion)
		return err
	})
	return out, err
}
func (s *Service) SetBannerStatus(ctx context.Context, in SetStatusInput) (domainbanner.Banner, error) {
	if err := s.requireAdmin(ctx, in.ActorUserID); err != nil {
		return domainbanner.Banner{}, err
	}
	if in.ExpectedVersion < 1 || !in.Status.Valid() {
		return domainbanner.Banner{}, validation("status", "Durum veya sürüm geçersiz.")
	}
	var out domainbanner.Banner
	err := s.withTx(ctx, func(r Repository) error {
		b, err := r.LockByID(ctx, in.BannerID)
		if err != nil {
			return err
		}
		if b.Version != in.ExpectedVersion {
			return apperr.StaleVersion("Banner başka bir işlem tarafından güncellendi.")
		}
		if in.Status == domainbanner.StatusActive {
			if err := s.requireReadyVariant(ctx, b.AssetID, b.Placement); err != nil {
				return err
			}
		}
		b.Status, b.UpdatedAt = in.Status, s.clock.Now().UTC()
		out, err = r.UpdateOptimistic(ctx, b, in.ExpectedVersion)
		return err
	})
	return out, err
}
func (s *Service) ReorderBanners(ctx context.Context, actor uuid.UUID, placement domainbanner.Placement, items []ReorderItem) error {
	if err := s.requireAdmin(ctx, actor); err != nil {
		return err
	}
	if !placement.Valid() || len(items) == 0 {
		return validation("items", "Yerleşim ve öğeler zorunludur.")
	}
	seen := map[uuid.UUID]bool{}
	return s.withTx(ctx, func(r Repository) error {
		for _, item := range items {
			if item.ID == uuid.Nil || item.ExpectedVersion < 1 || item.SortOrder < 0 || seen[item.ID] {
				return validation("items", "Sıralama öğeleri geçersiz.")
			}
			seen[item.ID] = true
			b, err := r.LockByID(ctx, item.ID)
			if err != nil {
				return err
			}
			if b.Placement != placement {
				return validation("items", "Banner yerleşimi eşleşmiyor.")
			}
			if b.Version != item.ExpectedVersion {
				return apperr.StaleVersion("Banner başka bir işlem tarafından güncellendi.")
			}
			b.SortOrder, b.UpdatedAt = item.SortOrder, s.clock.Now().UTC()
			if _, err = r.UpdateOptimistic(ctx, b, item.ExpectedVersion); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *Service) ListActiveBannersByPlacement(ctx context.Context, placement domainbanner.Placement) ([]domainbanner.Banner, error) {
	if !placement.Valid() {
		return nil, validation("placement", "Yerleşim geçersiz.")
	}
	return s.repo.ListActive(ctx, placement)
}
func (s *Service) validateAsset(ctx context.Context, id uuid.UUID) error {
	_, err := s.media.FindAssetByID(ctx, id)
	if isNotFound(err) {
		return apperr.NotFound("Medya varlığı bulunamadı.")
	}
	return err
}
func (s *Service) requireReadyVariant(ctx context.Context, asset uuid.UUID, placement domainbanner.Placement) error {
	variants, err := s.media.ListVariantsByAsset(ctx, asset)
	if err != nil {
		return err
	}
	profile := map[domainbanner.Placement]string{domainbanner.PlacementHomepage: domainmedia.ProfileHomepage, domainbanner.PlacementListingDetail: domainmedia.ProfileDetail, domainbanner.PlacementSearch: domainmedia.ProfileSearch}[placement]
	for _, v := range variants {
		if v.TransformProfile == profile && v.LifecycleStatus == domainmedia.VariantReady && v.ObjectKey != nil && strings.TrimSpace(*v.ObjectKey) != "" {
			return nil
		}
	}
	return apperr.InvalidState("Banner görseli yayınlanmaya hazır değil.")
}
func (s *Service) requireAdmin(ctx context.Context, id uuid.UUID) error {
	u, err := s.users.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if u.Role != domainuser.RoleAdmin || !u.IsActive() {
		return apperr.Forbidden(apperr.CodeForbidden, "Bu işlem için yetkiniz yok.")
	}
	return nil
}
func (s *Service) withTx(ctx context.Context, fn func(Repository) error) error {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = fn(s.repo.WithTx(tx)); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return apperr.Internal(fmt.Errorf("commit banner tx: %w", err))
	}
	return nil
}
func validation(field, message string) error {
	return apperr.Validation(invalidRequest, apperr.FieldError{Field: field, Message: message})
}
func isNotFound(err error) bool {
	var ae *apperr.Error
	return errors.As(err, &ae) && ae.Kind == apperr.KindNotFound
}
