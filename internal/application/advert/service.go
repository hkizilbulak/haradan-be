package advert

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

const (
	minAdvertPageLimit     = 1
	maxAdvertPageLimit     = 100
	defaultAdvertPageLimit = 20

	deletedAdvertMessage = "Silinmiş ilan güncellenemez."
	staleVersionMessage  = "İlan başka bir yerden güncellendi; sayfayı yenileyin."
)

// Service implements the ADVERT-OWNER-01..11 use cases.
type Service struct {
	repo          Repository
	public        PublicRepository
	catalog       CatalogReader
	geo           GeoReader
	horses        HorseReader
	users         UserReader
	clock         Clock
	notifications NotificationEmitter
}

// Config wires advert application dependencies.
type Config struct {
	Repo          Repository
	Public        PublicRepository
	Catalog       CatalogReader
	Geo           GeoReader
	Horses        HorseReader
	Users         UserReader
	Clock         Clock
	Notifications NotificationEmitter
}

// NewService constructs the advert application service.
func NewService(cfg Config) (*Service, error) {
	if cfg.Repo == nil || cfg.Catalog == nil || cfg.Geo == nil || cfg.Horses == nil || cfg.Users == nil {
		return nil, fmt.Errorf("advert service dependencies are required")
	}
	return newService(cfg)
}

// NewAutoArchiveService constructs a minimal service wired only for AutoArchiveSold.
// Caller must not invoke owner-facing or public methods on the returned service.
func NewAutoArchiveService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("advert repository is required")
	}
	return &Service{repo: repo, clock: systemClock{}}, nil
}

func newService(cfg Config) (*Service, error) {
	clock := cfg.Clock
	if clock == nil {
		clock = systemClock{}
	}
	return &Service{
		repo:          cfg.Repo,
		public:        cfg.Public,
		catalog:       cfg.Catalog,
		geo:           cfg.Geo,
		horses:        cfg.Horses,
		users:         cfg.Users,
		clock:         clock,
		notifications: cfg.Notifications,
	}, nil
}

// CreateDraftInput is ADVERT-OWNER-01 input; every field is optional.
type CreateDraftInput struct {
	CategoryID  *uuid.UUID
	DistrictID  *uuid.UUID
	HorseID     *uuid.UUID
	Title       *string
	Description *string
	Address     *string
	Price       *MoneyInput
}

// ListInput is ADVERT-OWNER-02 input.
type ListInput struct {
	Status *string
	Cursor *string
	Limit  *int
}

// ListResult is ADVERT-OWNER-02 output.
type ListResult struct {
	Items      []domainadvert.OwnerView
	NextCursor *string
	HasMore    bool
}

// UpdateDetailsInput is ADVERT-OWNER-04 input.
type UpdateDetailsInput struct {
	ExpectedVersion int

	DistrictIDSet bool
	DistrictID    *uuid.UUID

	HorseIDSet bool
	HorseID    *uuid.UUID

	TitleSet bool
	Title    *string

	DescriptionSet bool
	Description    *string

	AddressSet bool
	Address    *string

	PriceSet bool
	Price    *MoneyInput
}

// ChangeCategoryInput is ADVERT-OWNER-05 input.
type ChangeCategoryInput struct {
	ExpectedVersion int
	CategoryID      uuid.UUID
}

// ReplacePropertiesInput is ADVERT-OWNER-06 input.
type ReplacePropertiesInput struct {
	ExpectedVersion int
	Properties      json.RawMessage
}

// CreateAdvertDraft implements ADVERT-OWNER-01. Owner comes from the session,
// never from the request body. Partial drafts are allowed.
func (s *Service) CreateAdvertDraft(ctx context.Context, ownerID uuid.UUID, in CreateDraftInput) (domainadvert.OwnerView, error) {
	title, err := normalizeTitle("title", in.Title)
	if err != nil {
		return domainadvert.OwnerView{}, err
	}
	price, err := validateMoney("price", in.Price)
	if err != nil {
		return domainadvert.OwnerView{}, err
	}
	var cat domaincatalog.Category
	if in.CategoryID != nil {
		var err error
		cat, err = s.requireLeafCategoryWithCat(ctx, *in.CategoryID)
		if err != nil {
			return domainadvert.OwnerView{}, err
		}
	}
	if in.DistrictID != nil {
		if err := s.requireActiveDistrict(ctx, *in.DistrictID); err != nil {
			return domainadvert.OwnerView{}, err
		}
	}
	if in.HorseID != nil {
		if in.CategoryID != nil && !cat.AllowTjk {
			return domainadvert.OwnerView{}, apperr.Validation(invalidRequest, apperr.FieldError{
				Field:   "horseId",
				Message: "Bu kategori için TJK atı seçilemez.",
			})
		}
		if err := s.requireHorse(ctx, *in.HorseID); err != nil {
			return domainadvert.OwnerView{}, err
		}
	}

	now := s.clock.Now()
	properties := domainadvert.EmptyProperties()
	if in.HorseID != nil && in.CategoryID != nil && cat.AllowTjk {
		if h, err := s.horses.FindByID(ctx, *in.HorseID); err == nil {
			if enriched, err := EnrichHorseProperties(cat, h, properties, now); err == nil {
				properties = enriched
			}
		}
	}

	draft := domainadvert.Advert{
		OwnerUserID:  ownerID,
		CategoryID:   in.CategoryID,
		DistrictID:   in.DistrictID,
		HorseID:      in.HorseID,
		Title:        title,
		Description:  normalizeDescription(in.Description),
		Address:      normalizeDescription(in.Address),
		Price:        price,
		Status:       domainadvert.StatusDraft,
		Properties:   properties,
		Version:      1,
		MediaVersion: 1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// DRAFT insert and the initial NULL->DRAFT history row share one transaction.
	if err := s.withTx(ctx, func(ctx context.Context, repo Repository, _ pgx.Tx) error {
		if err := repo.Create(ctx, &draft); err != nil {
			return err
		}
		return repo.InsertHistory(ctx, domainadvert.StatusHistory{
			ID:          uuid.New(),
			AdvertID:    draft.ID,
			FromStatus:  nil,
			ToStatus:    domainadvert.StatusDraft,
			ActorUserID: &ownerID,
			IsSystem:    false,
			CreatedAt:   now,
		})
	}); err != nil {
		return domainadvert.OwnerView{}, err
	}
	return draft.ToOwnerView(), nil
}

// ListMyAdverts implements ADVERT-OWNER-02. Soft-deleted adverts are excluded.
func (s *Service) ListMyAdverts(ctx context.Context, ownerID uuid.UUID, in ListInput) (ListResult, error) {
	limit, err := resolveLimit(in.Limit)
	if err != nil {
		return ListResult{}, err
	}
	var status *domainadvert.Status
	if in.Status != nil && strings.TrimSpace(*in.Status) != "" {
		parsed, ok := domainadvert.ParseStatus(strings.TrimSpace(*in.Status))
		if !ok {
			return ListResult{}, apperr.BadRequest(apperr.CodeValidation, "Geçersiz ilan durumu.")
		}
		status = &parsed
	}
	var afterCreated *time.Time
	var afterID *int64
	if in.Cursor != nil && strings.TrimSpace(*in.Cursor) != "" {
		created, id, err := decodeAdvertCursor(strings.TrimSpace(*in.Cursor))
		if err != nil {
			return ListResult{}, apperr.BadRequest(apperr.CodeValidation, "Geçersiz cursor.")
		}
		afterCreated = &created
		afterID = &id
	}

	rows, err := s.repo.ListByOwner(ctx, ownerID, status, afterCreated, afterID, limit+1)
	if err != nil {
		return ListResult{}, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items, err := s.projectOwnerViews(ctx, rows)
	if err != nil {
		return ListResult{}, err
	}
	var next *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		cursor := encodeAdvertCursor(last.CreatedAt, last.ID)
		next = &cursor
	}
	return ListResult{Items: items, NextCursor: next, HasMore: hasMore}, nil
}

// GetMyAdvert implements ADVERT-OWNER-03.
func (s *Service) GetMyAdvert(ctx context.Context, ownerID uuid.UUID, advertID int64) (domainadvert.OwnerView, error) {
	found, err := s.repo.FindByIDForOwner(ctx, ownerID, advertID)
	if err != nil {
		return domainadvert.OwnerView{}, err
	}
	items, err := s.projectOwnerViews(ctx, []domainadvert.Advert{found})
	if err != nil {
		return domainadvert.OwnerView{}, err
	}
	return items[0], nil
}

// UpdateAdvertDraftDetails implements ADVERT-OWNER-04. The category is not
// touched here and no status history is written.
func (s *Service) UpdateAdvertDraftDetails(
	ctx context.Context,
	ownerID uuid.UUID, advertID int64,
	in UpdateDetailsInput,
) (domainadvert.OwnerView, error) {
	if err := requireExpectedVersion(in.ExpectedVersion); err != nil {
		return domainadvert.OwnerView{}, err
	}
	patch, err := s.buildDetailsPatch(ctx, in)
	if err != nil {
		return domainadvert.OwnerView{}, err
	}

	var updated domainadvert.Advert
	err = s.withTx(ctx, func(ctx context.Context, repo Repository, _ pgx.Tx) error {
		current, err := repo.FindByIDForOwnerForUpdate(ctx, ownerID, advertID)
		if err != nil {
			return err
		}
		if err := guardVersionAndStatus(current, in.ExpectedVersion, domainadvert.CanOwnerEditDetails); err != nil {
			return err
		}
		if patch.HorseIDSet && patch.HorseID != nil && current.CategoryID != nil {
			cat, err := s.catalog.GetActiveCategory(ctx, *current.CategoryID)
			if err == nil && !cat.AllowTjk {
				return apperr.Validation(invalidRequest, apperr.FieldError{
					Field:   "horseId",
					Message: "Bu kategori için TJK atı seçilemez.",
				})
			}
			if err == nil && cat.AllowTjk {
				if h, err := s.horses.FindByID(ctx, *patch.HorseID); err == nil {
					if enriched, err := EnrichHorseProperties(cat, h, current.Properties, s.clock.Now()); err == nil {
						patch.PropertiesSet = true
						patch.Properties = enriched
					}
				}
			}
		}
		if patch.IsEmpty() {
			updated = current
			return nil
		}
		now := s.clock.Now()
		updated, err = repo.UpdateDetails(ctx, ownerID, advertID, patch, in.ExpectedVersion, now)
		return err
	})
	if err != nil {
		return domainadvert.OwnerView{}, err
	}
	return updated.ToOwnerView(), nil
}

// ChangeAdvertDraftCategory implements ADVERT-OWNER-05. Only DRAFT adverts may
// change category; a real change clears the dynamic properties.
func (s *Service) ChangeAdvertDraftCategory(
	ctx context.Context,
	ownerID uuid.UUID, advertID int64,
	in ChangeCategoryInput,
) (domainadvert.OwnerView, error) {
	if err := requireExpectedVersion(in.ExpectedVersion); err != nil {
		return domainadvert.OwnerView{}, err
	}
	if in.CategoryID == uuid.Nil {
		return domainadvert.OwnerView{}, apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   "categoryId",
			Message: "Kategori zorunludur.",
		})
	}
	if err := s.requireLeafCategory(ctx, in.CategoryID); err != nil {
		return domainadvert.OwnerView{}, err
	}

	var updated domainadvert.Advert
	cleared := false
	err := s.withTx(ctx, func(ctx context.Context, repo Repository, _ pgx.Tx) error {
		current, err := repo.FindByIDForOwnerForUpdate(ctx, ownerID, advertID)
		if err != nil {
			return err
		}
		if err := guardVersionAndStatus(current, in.ExpectedVersion, domainadvert.CanOwnerChangeCategory); err != nil {
			return err
		}
		// Same category is a no-op: properties survive and the version stays put.
		if current.CategoryID != nil && *current.CategoryID == in.CategoryID {
			updated = current
			return nil
		}
		cleared = hasProperties(current.Properties)
		updated, err = repo.UpdateCategoryClearProperties(ctx, ownerID, advertID, in.CategoryID, in.ExpectedVersion, s.clock.Now())
		return err
	})
	if err != nil {
		return domainadvert.OwnerView{}, err
	}
	view := updated.ToOwnerView()
	view.CategoryClearedWarning = &cleared
	return view, nil
}

// ReplaceAdvertDynamicProperties implements ADVERT-OWNER-06 with draft-mode
// validation: unknown codes are rejected, required codes are not enforced yet.
func (s *Service) ReplaceAdvertDynamicProperties(
	ctx context.Context,
	ownerID uuid.UUID, advertID int64,
	in ReplacePropertiesInput,
) (domainadvert.OwnerView, error) {
	if err := requireExpectedVersion(in.ExpectedVersion); err != nil {
		return domainadvert.OwnerView{}, err
	}

	var updated domainadvert.Advert
	err := s.withTx(ctx, func(ctx context.Context, repo Repository, _ pgx.Tx) error {
		current, err := repo.FindByIDForOwnerForUpdate(ctx, ownerID, advertID)
		if err != nil {
			return err
		}
		if err := guardVersionAndStatus(current, in.ExpectedVersion, domainadvert.CanOwnerEditDetails); err != nil {
			return err
		}
		if current.CategoryID == nil {
			return apperr.InvalidState("Özellikler için önce kategori seçilmelidir.")
		}
		defs, err := s.catalog.ListFormProperties(ctx, *current.CategoryID)
		if err != nil {
			return err
		}
		normalized, err := validateDynamicProperties(defs, in.Properties, propertyModeDraft)
		if err != nil {
			return err
		}
		updated, err = repo.ReplaceProperties(ctx, ownerID, advertID, normalized, in.ExpectedVersion, s.clock.Now())
		return err
	})
	if err != nil {
		return domainadvert.OwnerView{}, err
	}
	return updated.ToOwnerView(), nil
}

// SubmitAdvertForReview implements ADVERT-OWNER-07 (DRAFT -> PENDING_REVIEW).
func (s *Service) SubmitAdvertForReview(ctx context.Context, ownerID uuid.UUID, advertID int64, expectedVersion int) (domainadvert.OwnerView, error) {
	return s.submitForReview(ctx, ownerID, advertID, expectedVersion, domainadvert.StatusDraft)
}

// ResubmitAdvertForReview implements ADVERT-OWNER-08 (CHANGES_REQUESTED -> PENDING_REVIEW).
func (s *Service) ResubmitAdvertForReview(ctx context.Context, ownerID uuid.UUID, advertID int64, expectedVersion int) (domainadvert.OwnerView, error) {
	return s.submitForReview(ctx, ownerID, advertID, expectedVersion, domainadvert.StatusChangesRequested)
}

// SoftDeleteAdvertDraft implements ADVERT-OWNER-09. Drafts only; no history row
// because a soft delete is not a status transition.
func (s *Service) SoftDeleteAdvertDraft(ctx context.Context, ownerID uuid.UUID, advertID int64, expectedVersion int) (domainadvert.OwnerView, error) {
	if err := requireExpectedVersion(expectedVersion); err != nil {
		return domainadvert.OwnerView{}, err
	}

	var updated domainadvert.Advert
	err := s.withTx(ctx, func(ctx context.Context, repo Repository, _ pgx.Tx) error {
		current, err := repo.FindByIDForOwnerForUpdate(ctx, ownerID, advertID)
		if err != nil {
			return err
		}
		// Already deleted: idempotent success, no second write.
		if current.IsDeleted() {
			updated = current
			return nil
		}
		if current.Version != expectedVersion {
			return apperr.StaleVersion(staleVersionMessage)
		}
		if current.Status != domainadvert.StatusDraft {
			return apperr.InvalidState("Yalnızca taslak ilanlar silinebilir.")
		}
		updated, err = repo.SoftDeleteDraft(ctx, ownerID, advertID, expectedVersion, s.clock.Now())
		return err
	})
	if err != nil {
		return domainadvert.OwnerView{}, err
	}
	return updated.ToOwnerView(), nil
}

// MarkAdvertSold implements ADVERT-OWNER-10 (PUBLISHED -> SOLD).
func (s *Service) MarkAdvertSold(ctx context.Context, ownerID uuid.UUID, advertID int64, expectedVersion int) (domainadvert.OwnerView, error) {
	return s.ownerTransition(ctx, ownerID, advertID, expectedVersion, domainadvert.StatusPublished, domainadvert.StatusSold)
}

// ArchiveAdvert implements ADVERT-OWNER-11 (PUBLISHED -> ARCHIVED).
func (s *Service) ArchiveAdvert(ctx context.Context, ownerID uuid.UUID, advertID int64, expectedVersion int) (domainadvert.OwnerView, error) {
	return s.ownerTransition(ctx, ownerID, advertID, expectedVersion, domainadvert.StatusPublished, domainadvert.StatusArchived)
}

// AutoArchiveSoldResult summarises one auto-archive batch run.
type AutoArchiveSoldResult struct {
	Archived int
	Errors   int
}

// AutoArchiveSold transitions SOLD adverts older than 24 h to ARCHIVED.
// It is invoked by the background worker and is safe to call concurrently;
// each row is updated under an optimistic version guard so a racing call
// simply skips that row.
func (s *Service) AutoArchiveSold(ctx context.Context, batchSize int) AutoArchiveSoldResult {
	if batchSize <= 0 {
		batchSize = 100
	}
	cutoff := s.clock.Now().Add(-24 * time.Hour)
	rows, err := s.repo.ListSoldForAutoArchive(ctx, cutoff, batchSize)
	if err != nil {
		return AutoArchiveSoldResult{Errors: 1}
	}
	var res AutoArchiveSoldResult
	now := s.clock.Now()
	for _, row := range rows {
		_, err := s.repo.SystemTransitionStatus(
			ctx, row.ID, domainadvert.StatusSold, domainadvert.StatusArchived, row.Version, now,
		)
		if err != nil {
			res.Errors++
			continue
		}
		_ = s.repo.InsertHistory(ctx, domainadvert.StatusHistory{
			ID:       uuid.New(),
			AdvertID: row.ID,
			FromStatus: func() *domainadvert.Status {
				s := domainadvert.StatusSold
				return &s
			}(),
			ToStatus:  domainadvert.StatusArchived,
			IsSystem:  true,
			CreatedAt: now,
		})
		res.Archived++
	}
	return res
}

func (s *Service) submitForReview(
	ctx context.Context,
	ownerID uuid.UUID, advertID int64,
	expectedVersion int,
	from domainadvert.Status,
) (domainadvert.OwnerView, error) {
	if err := requireExpectedVersion(expectedVersion); err != nil {
		return domainadvert.OwnerView{}, err
	}
	if err := s.requireActiveOwner(ctx, ownerID); err != nil {
		return domainadvert.OwnerView{}, err
	}

	var updated domainadvert.Advert
	now := s.clock.Now()
	err := s.withTx(ctx, func(ctx context.Context, repo Repository, _ pgx.Tx) error {
		current, err := repo.FindByIDForOwnerForUpdate(ctx, ownerID, advertID)
		if err != nil {
			return err
		}
		if current.IsDeleted() {
			return apperr.InvalidState(deletedAdvertMessage)
		}
		if current.Version != expectedVersion {
			return apperr.StaleVersion(staleVersionMessage)
		}
		if current.Status != from {
			return apperr.InvalidState("İlan bu durumda incelemeye gönderilemez.")
		}
		if err := s.validateForSubmission(ctx, current); err != nil {
			return err
		}
		updated, err = repo.TransitionStatus(
			ctx, ownerID, advertID, from, domainadvert.StatusPendingReview, expectedVersion, nil, now,
		)
		if err != nil {
			return err
		}
		fromStatus := from
		return repo.InsertHistory(ctx, domainadvert.StatusHistory{
			ID:          uuid.New(),
			AdvertID:    advertID,
			FromStatus:  &fromStatus,
			ToStatus:    domainadvert.StatusPendingReview,
			ActorUserID: &ownerID,
			IsSystem:    false,
			CreatedAt:   now,
		})
	})
	if err != nil {
		return domainadvert.OwnerView{}, err
	}
	return updated.ToOwnerView(), nil
}

func (s *Service) ownerTransition(
	ctx context.Context,
	ownerID uuid.UUID, advertID int64,
	expectedVersion int,
	from, to domainadvert.Status,
) (domainadvert.OwnerView, error) {
	if err := requireExpectedVersion(expectedVersion); err != nil {
		return domainadvert.OwnerView{}, err
	}
	if !domainadvert.OwnerTransitionAllowed(from, to) {
		return domainadvert.OwnerView{}, apperr.Internal(fmt.Errorf("unsupported owner transition %s->%s", from, to))
	}

	var updated domainadvert.Advert
	now := s.clock.Now()
	err := s.withTx(ctx, func(ctx context.Context, repo Repository, _ pgx.Tx) error {
		current, err := repo.FindByIDForOwnerForUpdate(ctx, ownerID, advertID)
		if err != nil {
			return err
		}
		if current.IsDeleted() {
			return apperr.InvalidState(deletedAdvertMessage)
		}
		if current.Version != expectedVersion {
			return apperr.StaleVersion(staleVersionMessage)
		}
		if current.Status != from {
			return apperr.InvalidState("İlan bu durumda bu işleme uygun değil.")
		}
		updated, err = repo.TransitionStatus(ctx, ownerID, advertID, from, to, expectedVersion, nil, now)
		if err != nil {
			return err
		}
		fromStatus := from
		return repo.InsertHistory(ctx, domainadvert.StatusHistory{
			ID:          uuid.New(),
			AdvertID:    advertID,
			FromStatus:  &fromStatus,
			ToStatus:    to,
			ActorUserID: &ownerID,
			IsSystem:    false,
			CreatedAt:   now,
		})
	})
	if err != nil {
		return domainadvert.OwnerView{}, err
	}
	return updated.ToOwnerView(), nil
}

// validateForSubmission is INTERNAL-04: everything the moderation queue needs
// before a submit is accepted. Description is optional; price and address are required.
func (s *Service) validateForSubmission(ctx context.Context, a domainadvert.Advert) error {
	var fields []apperr.FieldError
	if a.CategoryID == nil {
		fields = append(fields, apperr.FieldError{Field: "categoryId", Message: "Kategori zorunludur."})
	}
	if a.DistrictID == nil {
		fields = append(fields, apperr.FieldError{Field: "districtId", Message: "İlçe zorunludur."})
	}
	if a.Title == nil || strings.TrimSpace(*a.Title) == "" {
		fields = append(fields, apperr.FieldError{Field: "title", Message: "Başlık zorunludur."})
	} else if len([]rune(strings.TrimSpace(*a.Title))) > maxTitleRunes {
		fields = append(fields, apperr.FieldError{
			Field:   "title",
			Message: fmt.Sprintf("Başlık en fazla %d karakter olabilir.", maxTitleRunes),
		})
	}
	if a.Price == nil || a.Price.AmountMinor <= 0 {
		fields = append(fields, apperr.FieldError{Field: "price", Message: "Fiyat zorunludur."})
	}
	mediaByAdvert, err := s.repo.ListMediaRelations(ctx, []int64{a.ID})
	if err != nil {
		return err
	}
	hasUsableMedia := false
	for _, rel := range mediaByAdvert[a.ID] {
		lifecycle := domainmedia.AssetLifecycle(rel.LifecycleStatus)
		if domainmedia.IsAttachableAssetLifecycle(lifecycle) && lifecycle != domainmedia.AssetUploadPending {
			hasUsableMedia = true
			break
		}
	}
	if !hasUsableMedia {
		fields = append(fields, apperr.FieldError{
			Field:   "media",
			Message: "En az bir görsel zorunludur.",
		})
	}
	if len(fields) > 0 {
		return apperr.Validation(invalidRequest, fields...)
	}

	if err := s.requireLeafCategory(ctx, *a.CategoryID); err != nil {
		return err
	}
	if err := s.requireActiveDistrict(ctx, *a.DistrictID); err != nil {
		return err
	}
	if a.HorseID != nil {
		if err := s.requireHorse(ctx, *a.HorseID); err != nil {
			return err
		}
	}

	defs, err := s.catalog.ListFormProperties(ctx, *a.CategoryID)
	if err != nil {
		return err
	}
	if _, err := validateDynamicProperties(defs, a.Properties, propertyModeSubmit); err != nil {
		return err
	}
	return nil
}

func (s *Service) buildDetailsPatch(ctx context.Context, in UpdateDetailsInput) (domainadvert.DetailsPatch, error) {
	patch := domainadvert.DetailsPatch{
		DistrictIDSet:  in.DistrictIDSet,
		DistrictID:     in.DistrictID,
		HorseIDSet:     in.HorseIDSet,
		HorseID:        in.HorseID,
		TitleSet:       in.TitleSet,
		DescriptionSet: in.DescriptionSet,
		AddressSet:     in.AddressSet,
		PriceSet:       in.PriceSet,
	}
	if in.TitleSet {
		title, err := normalizeTitle("title", in.Title)
		if err != nil {
			return domainadvert.DetailsPatch{}, err
		}
		patch.Title = title
	}
	if in.DescriptionSet {
		patch.Description = normalizeDescription(in.Description)
	}
	if in.AddressSet {
		patch.Address = normalizeDescription(in.Address)
	}
	if in.PriceSet {
		price, err := validateMoney("price", in.Price)
		if err != nil {
			return domainadvert.DetailsPatch{}, err
		}
		patch.Price = price
	}
	if in.DistrictIDSet && in.DistrictID != nil {
		if err := s.requireActiveDistrict(ctx, *in.DistrictID); err != nil {
			return domainadvert.DetailsPatch{}, err
		}
	}
	if in.HorseIDSet && in.HorseID != nil {
		if err := s.requireHorse(ctx, *in.HorseID); err != nil {
			return domainadvert.DetailsPatch{}, err
		}
	}
	return patch, nil
}

func (s *Service) requireActiveOwner(ctx context.Context, ownerID uuid.UUID) error {
	owner, err := s.users.FindByID(ctx, ownerID)
	if err != nil {
		if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
			return apperr.Unauthenticated(apperr.CodeUnauthenticated, "Kimlik doğrulama gerekli.")
		}
		return err
	}
	if !owner.IsActive() {
		return apperr.Forbidden(apperr.CodeAccountInactive, "Hesap aktif değil.")
	}
	return nil
}

// requireLeafCategory accepts only active leaf categories; phase one does not
// allow posting directly to a parent category.
func (s *Service) requireLeafCategory(ctx context.Context, categoryID uuid.UUID) error {
	_, err := s.requireLeafCategoryWithCat(ctx, categoryID)
	return err
}

func (s *Service) requireLeafCategoryWithCat(ctx context.Context, categoryID uuid.UUID) (domaincatalog.Category, error) {
	cat, err := s.catalog.GetActiveCategory(ctx, categoryID)
	if err != nil {
		if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
			return domaincatalog.Category{}, apperr.Validation(invalidRequest, apperr.FieldError{
				Field:   "categoryId",
				Message: "Kategori bulunamadı veya aktif değil.",
			})
		}
		return domaincatalog.Category{}, err
	}
	children, err := s.catalog.CountActiveChildren(ctx, cat.ID)
	if err != nil {
		return domaincatalog.Category{}, err
	}
	if children > 0 {
		return domaincatalog.Category{}, apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   "categoryId",
			Message: "Yalnız alt kategorisi olmayan kategoriye ilan verilebilir.",
		})
	}
	return cat, nil
}

func (s *Service) requireActiveDistrict(ctx context.Context, districtID uuid.UUID) error {
	if _, err := s.geo.GetActiveDistrict(ctx, districtID); err != nil {
		if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
			return apperr.Validation(invalidRequest, apperr.FieldError{
				Field:   "districtId",
				Message: "İlçe bulunamadı veya aktif değil.",
			})
		}
		return err
	}
	return nil
}

func (s *Service) requireHorse(ctx context.Context, horseID uuid.UUID) error {
	if _, err := s.horses.FindByID(ctx, horseID); err != nil {
		if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
			return apperr.Validation(invalidRequest, apperr.FieldError{
				Field:   "horseId",
				Message: "At kaydı bulunamadı.",
			})
		}
		return err
	}
	return nil
}

func (s *Service) withTx(ctx context.Context, fn func(context.Context, Repository, pgx.Tx) error) error {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(ctx, s.repo.WithTx(tx), tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		if errors.Is(err, pgx.ErrTxClosed) {
			return apperr.Internal(err)
		}
		return apperr.Internal(fmt.Errorf("commit advert tx: %w", err))
	}
	return nil
}

// projectOwnerViews attaches media relations and province ids for owner reads.
func (s *Service) projectOwnerViews(ctx context.Context, rows []domainadvert.Advert) ([]domainadvert.OwnerView, error) {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	mediaByAdvert, err := s.repo.ListMediaRelations(ctx, ids)
	if err != nil {
		return nil, err
	}
	items := make([]domainadvert.OwnerView, 0, len(rows))
	for _, row := range rows {
		view := row.ToOwnerView()
		if media := mediaByAdvert[row.ID]; len(media) > 0 {
			view.Media = media
		}
		if row.DistrictID != nil {
			district, geoErr := s.geo.GetActiveDistrict(ctx, *row.DistrictID)
			if geoErr == nil {
				pid := district.ProvinceID
				view.ProvinceID = &pid
			}
		}
		items = append(items, view)
	}
	return items, nil
}

// guardVersionAndStatus rejects a stale snapshot before judging the status: when
// the version no longer matches, the caller's whole view is outdated.
func guardVersionAndStatus(a domainadvert.Advert, expectedVersion int, allowed func(domainadvert.Status) bool) error {
	if a.IsDeleted() {
		return apperr.InvalidState(deletedAdvertMessage)
	}
	if a.Version != expectedVersion {
		return apperr.StaleVersion(staleVersionMessage)
	}
	if !allowed(a.Status) {
		return apperr.InvalidState("İlan bu durumda düzenlenemez.")
	}
	return nil
}

func requireExpectedVersion(v int) error {
	if v < 1 {
		return apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   "expectedVersion",
			Message: "Sürüm numarası 1 veya daha büyük olmalıdır.",
		})
	}
	return nil
}

func resolveLimit(limit *int) (int, error) {
	if limit == nil {
		return defaultAdvertPageLimit, nil
	}
	if *limit < minAdvertPageLimit || *limit > maxAdvertPageLimit {
		return 0, apperr.BadRequest(apperr.CodeValidation, "limit 1 ile 100 arasında olmalıdır")
	}
	return *limit, nil
}

func hasProperties(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("{}")) {
		return false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return false
	}
	return len(obj) > 0
}

func encodeAdvertCursor(createdAt time.Time, id int64) string {
	raw := createdAt.UTC().Format(time.RFC3339Nano) + "|" + strconv.FormatInt(id, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeAdvertCursor(cursor string) (time.Time, int64, error) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, 0, err
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, 0, fmt.Errorf("bad cursor")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, 0, err
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return time.Time{}, 0, err
	}
	return createdAt, id, nil
}
