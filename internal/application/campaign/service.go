package campaign

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincampaign "github.com/hkizilbulak/haradan-be/internal/domain/campaign"
	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	"github.com/hkizilbulak/haradan-be/internal/platform/security/richhtml"
)

const (
	minPageLimit     = 1
	maxPageLimit     = 100
	defaultPageLimit = 20

	forbiddenMessage         = "Bu işlem için yetkiniz yok."
	campaignNotFoundMessage  = "Kampanya bulunamadı."
	staleVersionMessage      = "Kampanya başka bir işlem tarafından güncellendi."
	campaignConflictMessage  = "Kampanya kodu zaten kullanılıyor."
	invalidRequestMessage    = "Geçersiz istek."
	invalidCursorMessage     = "Geçersiz sayfalama imleci."
	invalidDateRangeMessage  = "Başlangıç ve bitiş tarihleri geçersiz."
	invalidPriceOrderMessage = "Kampanya fiyatı orijinal fiyattan yüksek olamaz."
	packageNotFoundMessage   = "Paket bulunamadı."
	assetNotFoundMessage     = "Medya varlığı bulunamadı."
)

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// Service implements campaign admin use cases.
type Service struct {
	repo     Repository
	packages PackageLookup
	assets   AssetLookup
	users    UserReader
	clock    Clock
}

// Config wires campaign application dependencies.
type Config struct {
	Repo     Repository
	Packages PackageLookup
	Assets   AssetLookup
	Users    UserReader
	Clock    Clock
}

// NewService constructs the campaign application service.
func NewService(cfg Config) (*Service, error) {
	if cfg.Repo == nil || cfg.Packages == nil || cfg.Assets == nil || cfg.Users == nil {
		return nil, fmt.Errorf("campaign service dependencies are required")
	}
	clock := cfg.Clock
	if clock == nil {
		clock = systemClock{}
	}
	return &Service{
		repo:     cfg.Repo,
		packages: cfg.Packages,
		assets:   cfg.Assets,
		users:    cfg.Users,
		clock:    clock,
	}, nil
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// CreateCampaignInput is the admin create request.
type CreateCampaignInput struct {
	ActorUserID                     uuid.UUID
	Code                            string
	Name                            string
	EventType                       domaincampaign.CampaignEventType
	SourcePackageCode               *string
	TargetPackageCode               *string
	Title                           string
	Description                     *string
	EmailSubject                    *string
	EmailHeading                    *string
	EmailBody                       *string
	EmailProviderTemplateID         *string
	CTALabel                        *string
	CTAURL                          *string
	BadgeText                       *string
	ImageAssetID                    *uuid.UUID
	DisplayOriginalPriceAmountMinor *int64
	DisplayCampaignPriceAmountMinor *int64
	CurrencyCode                    string
	StartsAt                        time.Time
	EndsAt                          *time.Time
	IsActive                        bool
}

// UpdateCampaignInput is a partial admin patch.
type UpdateCampaignInput struct {
	ActorUserID     uuid.UUID
	CampaignID      uuid.UUID
	ExpectedVersion int

	Name                            *string
	EventType                       *domaincampaign.CampaignEventType
	SourcePackageCodeSet            bool
	SourcePackageCode               *string
	TargetPackageCodeSet            bool
	TargetPackageCode               *string
	Title                           *string
	DescriptionSet                  bool
	Description                     *string
	EmailSubjectSet                 bool
	EmailSubject                    *string
	EmailHeadingSet                 bool
	EmailHeading                    *string
	EmailBodySet                    bool
	EmailBody                       *string
	EmailProviderTemplateIDSet      bool
	EmailProviderTemplateID         *string
	CTALabelSet                     bool
	CTALabel                        *string
	CTAURLSet                       bool
	CTAURL                          *string
	BadgeTextSet                    bool
	BadgeText                       *string
	ImageAssetIDSet                 bool
	ImageAssetID                    *uuid.UUID
	DisplayOriginalPriceAmountMinor *int64
	ClearOriginalPrice              bool
	DisplayCampaignPriceAmountMinor *int64
	ClearCampaignPrice              bool
	CurrencyCode                    *string
	StartsAt                        *time.Time
	EndsAtSet                       bool
	EndsAt                          *time.Time
	IsActive                        *bool
}

// ListCampaignsInput is the admin list request.
type ListCampaignsInput struct {
	Cursor            *string
	Limit             *int
	EventType         *domaincampaign.CampaignEventType
	IsActive          *bool
	SourcePackageCode *string
	TargetPackageCode *string
}

// ListCampaignsResult is paginated campaign list output.
type ListCampaignsResult struct {
	Items      []domaincampaign.Campaign
	NextCursor *string
	HasMore    bool
}

// CreateCampaign creates a campaign (ACTIVE ADMIN only).
func (s *Service) CreateCampaign(ctx context.Context, in CreateCampaignInput) (domaincampaign.Campaign, error) {
	if err := s.requireAdmin(ctx, in.ActorUserID); err != nil {
		return domaincampaign.Campaign{}, err
	}

	code := strings.TrimSpace(in.Code)
	name := strings.TrimSpace(in.Name)
	title := strings.TrimSpace(in.Title)
	if !domaincampaign.NonBlankName(name) {
		return domaincampaign.Campaign{}, apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field: "name", Message: "Ad zorunludur.",
		})
	}
	if code == "" {
		allocated, err := s.allocateCampaignCode(ctx, domaincampaign.GenerateCampaignCodeBase(name))
		if err != nil {
			return domaincampaign.Campaign{}, err
		}
		code = allocated
	}
	if !in.EventType.Valid() {
		return domaincampaign.Campaign{}, apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field: "eventType", Message: "Olay tipi geçersiz.",
		})
	}
	if !domaincampaign.NonBlankTitle(title) {
		return domaincampaign.Campaign{}, apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field: "title", Message: "Başlık zorunludur.",
		})
	}

	currency := strings.TrimSpace(in.CurrencyCode)
	if currency == "" {
		currency = "TRY"
	}
	if err := validateMoneyFields(in.DisplayOriginalPriceAmountMinor, in.DisplayCampaignPriceAmountMinor, currency); err != nil {
		return domaincampaign.Campaign{}, err
	}
	startsAt := in.StartsAt.UTC()
	var endsAt *time.Time
	if in.EndsAt != nil {
		t := in.EndsAt.UTC()
		endsAt = &t
	}
	if !domaincampaign.ValidTimeRange(startsAt, endsAt) {
		return domaincampaign.Campaign{}, apperr.Validation(invalidDateRangeMessage)
	}
	if !domaincampaign.CampaignPriceLTEOriginal(in.DisplayOriginalPriceAmountMinor, in.DisplayCampaignPriceAmountMinor) {
		return domaincampaign.Campaign{}, apperr.Validation(invalidPriceOrderMessage)
	}

	sourceID, err := s.resolvePackageCode(ctx, in.SourcePackageCode)
	if err != nil {
		return domaincampaign.Campaign{}, err
	}
	targetID, err := s.resolvePackageCode(ctx, in.TargetPackageCode)
	if err != nil {
		return domaincampaign.Campaign{}, err
	}
	if err := s.validateAsset(ctx, in.ImageAssetID); err != nil {
		return domaincampaign.Campaign{}, err
	}

	now := s.clock.Now().UTC()
	created := domaincampaign.Campaign{
		ID:                              uuid.New(),
		Code:                            code,
		Name:                            name,
		EventType:                       in.EventType,
		SourcePackageID:                 sourceID,
		TargetPackageID:                 targetID,
		Title:                           title,
		Description:                     trimOptional(in.Description),
		EmailSubject:                    trimOptional(in.EmailSubject),
		EmailHeading:                    trimOptional(in.EmailHeading),
		EmailBody:                       richhtml.SanitizeOptional(in.EmailBody),
		EmailProviderTemplateID:         trimOptional(in.EmailProviderTemplateID),
		CTALabel:                        trimOptional(in.CTALabel),
		CTAURL:                          trimOptional(in.CTAURL),
		BadgeText:                       trimOptional(in.BadgeText),
		ImageAssetID:                    in.ImageAssetID,
		DisplayOriginalPriceAmountMinor: in.DisplayOriginalPriceAmountMinor,
		DisplayCampaignPriceAmountMinor: in.DisplayCampaignPriceAmountMinor,
		CurrencyCode:                    currency,
		StartsAt:                        startsAt,
		EndsAt:                          endsAt,
		IsActive:                        in.IsActive,
		CreatedByUserID:                 in.ActorUserID,
		Version:                         1,
		CreatedAt:                       now,
		UpdatedAt:                       now,
	}

	if err := s.repo.Create(ctx, created); err != nil {
		return domaincampaign.Campaign{}, err
	}
	return created, nil
}

// GetCampaign returns a campaign by id (ACTIVE ADMIN only).
func (s *Service) GetCampaign(ctx context.Context, actorUserID, campaignID uuid.UUID) (domaincampaign.Campaign, error) {
	if err := s.requireAdmin(ctx, actorUserID); err != nil {
		return domaincampaign.Campaign{}, err
	}
	return s.repo.GetByID(ctx, campaignID)
}

// ListCampaigns returns paginated campaigns (ACTIVE ADMIN only).
func (s *Service) ListCampaigns(ctx context.Context, actorUserID uuid.UUID, in ListCampaignsInput) (ListCampaignsResult, error) {
	if err := s.requireAdmin(ctx, actorUserID); err != nil {
		return ListCampaignsResult{}, err
	}
	limit, err := resolveLimit(in.Limit)
	if err != nil {
		return ListCampaignsResult{}, err
	}
	if in.EventType != nil && !in.EventType.Valid() {
		return ListCampaignsResult{}, apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field: "eventType", Message: "Olay tipi geçersiz.",
		})
	}

	sourceID, err := s.resolvePackageCode(ctx, in.SourcePackageCode)
	if err != nil {
		return ListCampaignsResult{}, err
	}
	targetID, err := s.resolvePackageCode(ctx, in.TargetPackageCode)
	if err != nil {
		return ListCampaignsResult{}, err
	}

	var afterCreated *time.Time
	var afterID *uuid.UUID
	if in.Cursor != nil && strings.TrimSpace(*in.Cursor) != "" {
		created, id, err := decodeCampaignCursor(strings.TrimSpace(*in.Cursor))
		if err != nil {
			return ListCampaignsResult{}, err
		}
		afterCreated = &created
		afterID = &id
	}

	rows, err := s.repo.List(ctx, ListFilter{
		EventType:       in.EventType,
		IsActive:        in.IsActive,
		SourcePackageID: sourceID,
		TargetPackageID: targetID,
		AfterCreatedAt:  afterCreated,
		AfterID:         afterID,
		Limit:           limit + 1,
	})
	if err != nil {
		return ListCampaignsResult{}, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := ListCampaignsResult{Items: rows, HasMore: hasMore}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		cursor := encodeCampaignCursor(last.CreatedAt, last.ID)
		out.NextCursor = &cursor
	}
	return out, nil
}

// UpdateCampaign applies a partial optimistic patch (ACTIVE ADMIN only).
func (s *Service) UpdateCampaign(ctx context.Context, in UpdateCampaignInput) (domaincampaign.Campaign, error) {
	if err := s.requireAdmin(ctx, in.ActorUserID); err != nil {
		return domaincampaign.Campaign{}, err
	}
	if in.ExpectedVersion < 1 {
		return domaincampaign.Campaign{}, apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field: "expectedVersion", Message: "Sürüm numarası 1 veya daha büyük olmalıdır.",
		})
	}

	var out domaincampaign.Campaign
	err := s.withTx(ctx, func(ctx context.Context, repo Repository) error {
		current, err := repo.LockByID(ctx, in.CampaignID)
		if err != nil {
			return err
		}
		if current.Version != in.ExpectedVersion {
			return apperr.StaleVersion(staleVersionMessage)
		}

		patched, err := s.applyCampaignPatch(ctx, current, in)
		if err != nil {
			return err
		}
		updated, err := repo.UpdateOptimistic(ctx, patched, in.ExpectedVersion)
		if err != nil {
			return err
		}
		out = updated
		return nil
	})
	if err != nil {
		return domaincampaign.Campaign{}, err
	}
	return out, nil
}

func (s *Service) applyCampaignPatch(
	ctx context.Context,
	current domaincampaign.Campaign,
	in UpdateCampaignInput,
) (domaincampaign.Campaign, error) {
	c := current

	// Campaign code is immutable after create (not exposed on OpenAPI update).
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if !domaincampaign.NonBlankName(name) {
			return domaincampaign.Campaign{}, apperr.Validation(invalidRequestMessage, apperr.FieldError{
				Field: "name", Message: "Ad zorunludur.",
			})
		}
		c.Name = name
	}
	if in.EventType != nil {
		if !in.EventType.Valid() {
			return domaincampaign.Campaign{}, apperr.Validation(invalidRequestMessage, apperr.FieldError{
				Field: "eventType", Message: "Olay tipi geçersiz.",
			})
		}
		c.EventType = *in.EventType
	}
	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if !domaincampaign.NonBlankTitle(title) {
			return domaincampaign.Campaign{}, apperr.Validation(invalidRequestMessage, apperr.FieldError{
				Field: "title", Message: "Başlık zorunludur.",
			})
		}
		c.Title = title
	}

	if in.SourcePackageCodeSet {
		id, err := s.resolvePackageCode(ctx, in.SourcePackageCode)
		if err != nil {
			return domaincampaign.Campaign{}, err
		}
		c.SourcePackageID = id
	}
	if in.TargetPackageCodeSet {
		id, err := s.resolvePackageCode(ctx, in.TargetPackageCode)
		if err != nil {
			return domaincampaign.Campaign{}, err
		}
		c.TargetPackageID = id
	}

	if in.DescriptionSet {
		c.Description = trimOptional(in.Description)
	}
	if in.EmailSubjectSet {
		c.EmailSubject = trimOptional(in.EmailSubject)
	}
	if in.EmailHeadingSet {
		c.EmailHeading = trimOptional(in.EmailHeading)
	}
	if in.EmailBodySet {
		c.EmailBody = richhtml.SanitizeOptional(in.EmailBody)
	}
	if in.EmailProviderTemplateIDSet {
		c.EmailProviderTemplateID = trimOptional(in.EmailProviderTemplateID)
	}
	if in.CTALabelSet {
		c.CTALabel = trimOptional(in.CTALabel)
	}
	if in.CTAURLSet {
		c.CTAURL = trimOptional(in.CTAURL)
	}
	if in.BadgeTextSet {
		c.BadgeText = trimOptional(in.BadgeText)
	}
	if in.ImageAssetIDSet {
		if err := s.validateAsset(ctx, in.ImageAssetID); err != nil {
			return domaincampaign.Campaign{}, err
		}
		c.ImageAssetID = in.ImageAssetID
	}

	if in.ClearOriginalPrice {
		c.DisplayOriginalPriceAmountMinor = nil
	} else if in.DisplayOriginalPriceAmountMinor != nil {
		c.DisplayOriginalPriceAmountMinor = in.DisplayOriginalPriceAmountMinor
	}
	if in.ClearCampaignPrice {
		c.DisplayCampaignPriceAmountMinor = nil
	} else if in.DisplayCampaignPriceAmountMinor != nil {
		c.DisplayCampaignPriceAmountMinor = in.DisplayCampaignPriceAmountMinor
	}
	if in.CurrencyCode != nil {
		currency := strings.TrimSpace(*in.CurrencyCode)
		if currency == "" {
			return domaincampaign.Campaign{}, apperr.Validation(invalidRequestMessage, apperr.FieldError{
				Field: "currencyCode", Message: "Para birimi geçersiz.",
			})
		}
		c.CurrencyCode = currency
	}
	if in.StartsAt != nil {
		c.StartsAt = in.StartsAt.UTC()
	}
	if in.EndsAtSet {
		if in.EndsAt == nil {
			c.EndsAt = nil
		} else {
			t := in.EndsAt.UTC()
			c.EndsAt = &t
		}
	}
	if in.IsActive != nil {
		c.IsActive = *in.IsActive
	}

	if err := validateMoneyFields(c.DisplayOriginalPriceAmountMinor, c.DisplayCampaignPriceAmountMinor, c.CurrencyCode); err != nil {
		return domaincampaign.Campaign{}, err
	}
	if !domaincampaign.ValidTimeRange(c.StartsAt, c.EndsAt) {
		return domaincampaign.Campaign{}, apperr.Validation(invalidDateRangeMessage)
	}
	if !domaincampaign.CampaignPriceLTEOriginal(c.DisplayOriginalPriceAmountMinor, c.DisplayCampaignPriceAmountMinor) {
		return domaincampaign.Campaign{}, apperr.Validation(invalidPriceOrderMessage)
	}

	c.UpdatedAt = s.clock.Now().UTC()
	return c, nil
}

func (s *Service) resolvePackageCode(ctx context.Context, code *string) (*uuid.UUID, error) {
	if code == nil {
		return nil, nil
	}
	raw := strings.TrimSpace(*code)
	if raw == "" {
		return nil, nil
	}
	pkgCode, ok := domainpackaging.ParsePackageCode(raw)
	if !ok {
		return nil, apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field: "packageCode", Message: "Paket kodu geçersiz.",
		})
	}
	pkg, err := s.packages.FindByCode(ctx, pkgCode)
	if err != nil {
		if isNotFound(err) {
			return nil, apperr.NotFound(packageNotFoundMessage)
		}
		return nil, err
	}
	id := pkg.ID
	return &id, nil
}

func (s *Service) allocateCampaignCode(ctx context.Context, base string) (string, error) {
	existing, err := s.repo.List(ctx, ListFilter{Limit: maxPageLimit})
	if err != nil {
		return "", err
	}
	used := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		used[item.Code] = struct{}{}
	}
	candidate := strings.TrimSpace(base)
	if candidate == "" {
		candidate = "CAMPAIGN"
	}
	for i := 0; i < 1000; i++ {
		try := candidate
		if i > 0 {
			suffix := fmt.Sprintf("_%d", i+1)
			stem := candidate
			if len(stem)+len(suffix) > 64 {
				stem = stem[:64-len(suffix)]
				stem = strings.TrimRight(stem, "_")
			}
			try = stem + suffix
		}
		if _, ok := used[try]; !ok {
			return try, nil
		}
	}
	return "", apperr.Conflict("Kampanya kodu üretilemedi.")
}

func (s *Service) validateAsset(ctx context.Context, assetID *uuid.UUID) error {
	if assetID == nil {
		return nil
	}
	if *assetID == uuid.Nil {
		return apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field: "imageAssetId", Message: "Medya varlığı geçersiz.",
		})
	}
	if _, err := s.assets.FindAssetByID(ctx, *assetID); err != nil {
		if isNotFound(err) {
			return apperr.NotFound(assetNotFoundMessage)
		}
		return err
	}
	return nil
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
		return apperr.Internal(fmt.Errorf("commit campaign tx: %w", err))
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

func validateMoneyFields(original, campaign *int64, currency string) error {
	if original != nil && *original < 0 {
		return apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field: "displayOriginalPriceAmountMinor", Message: "Tutar negatif olamaz.",
		})
	}
	if campaign != nil && *campaign < 0 {
		return apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field: "displayCampaignPriceAmountMinor", Message: "Tutar negatif olamaz.",
		})
	}
	if !currencyPattern.MatchString(currency) {
		return apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field: "currencyCode", Message: "Para birimi üç büyük harf olmalıdır.",
		})
	}
	return nil
}

func resolveLimit(limit *int) (int, error) {
	if limit == nil {
		return defaultPageLimit, nil
	}
	if *limit < minPageLimit || *limit > maxPageLimit {
		return 0, apperr.Validation(invalidRequestMessage, apperr.FieldError{
			Field: "limit", Message: "Sayfa boyutu 1 ile 100 arasında olmalıdır.",
		})
	}
	return *limit, nil
}

func encodeCampaignCursor(createdAt time.Time, id uuid.UUID) string {
	raw := createdAt.UTC().Format(time.RFC3339Nano) + "|" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCampaignCursor(cursor string) (time.Time, uuid.UUID, error) {
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
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
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
	return createdAt, id, nil
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

func isNotFound(err error) bool {
	var ae *apperr.Error
	return errors.As(err, &ae) && ae.Kind == apperr.KindNotFound
}
