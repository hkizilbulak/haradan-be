package advert

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
)

const (
	adminInvalidStateMessage = "İlan bu durumda bu işleme uygun değil."
	reasonRequiredMessage    = "Moderasyon gerekçesi zorunludur."
)

// ModerationListInput is ADVERT-ADMIN-01 input.
type ModerationListInput struct {
	Status *string
	Cursor *string
	Limit  *int
}

// ModerationReasonInput carries expectedVersion + required reason.
type ModerationReasonInput struct {
	ExpectedVersion int
	Reason          string
}

// ListAdvertModerationQueue implements ADVERT-ADMIN-01.
// When status is omitted, all non-deleted adverts are returned.
func (s *Service) ListAdvertModerationQueue(ctx context.Context, in ModerationListInput) (ListResult, error) {
	limit, err := resolveLimit(in.Limit)
	if err != nil {
		return ListResult{}, err
	}
	var status *domainadvert.Status
	if in.Status != nil && strings.TrimSpace(*in.Status) != "" && !strings.EqualFold(strings.TrimSpace(*in.Status), "ALL") {
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

	rows, totalCount, err := s.repo.ListForModeration(ctx, status, afterCreated, afterID, limit+1)
	if err != nil {
		return ListResult{}, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items, err := s.projectOwnerViews(ctx, rows)
	if err != nil {
		items = make([]domainadvert.OwnerView, 0, len(rows))
		for _, row := range rows {
			items = append(items, row.ToOwnerView())
		}
	}
	var next *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		cursor := encodeAdvertCursor(last.CreatedAt, last.ID)
		next = &cursor
	}
	return ListResult{Items: items, NextCursor: next, HasMore: hasMore, TotalCount: totalCount}, nil
}

// GetAdvertModerationDetail implements ADVERT-ADMIN-02.
func (s *Service) GetAdvertModerationDetail(ctx context.Context, advertID int64) (domainadvert.ModerationDetailView, error) {
	found, err := s.repo.FindByID(ctx, advertID)
	if err != nil {
		return domainadvert.ModerationDetailView{}, err
	}
	return s.moderationDetail(ctx, found)
}

type AdminUpdateAdvertInput struct {
	ExpectedVersion *int
	Title           *string
	Description     *string
	Price           *MoneyInput
	DistrictID      *uuid.UUID
	HorseID         *uuid.UUID
	Properties      map[string]interface{}
	Media           []AdminMediaInput
}

type AdminMediaInput struct {
	AssetID      uuid.UUID
	DisplayOrder int
	IsCover      bool
}

// UpdateAdvertAdmin implements admin editing of advert details, properties, and media.
func (s *Service) UpdateAdvertAdmin(
	ctx context.Context,
	actorUserID uuid.UUID,
	advertID int64,
	in AdminUpdateAdvertInput,
) (domainadvert.ModerationDetailView, error) {
	var updated domainadvert.Advert
	now := s.clock.Now()

	err := s.withTx(ctx, func(ctx context.Context, repo Repository, tx pgx.Tx) error {
		current, err := repo.FindByIDForUpdate(ctx, advertID)
		if err != nil {
			return err
		}
		if current.IsDeleted() {
			return apperr.InvalidState(deletedAdvertMessage)
		}
		expectedVer := current.Version

		patch := domainadvert.DetailsPatch{}
		if in.Title != nil {
			title := strings.TrimSpace(*in.Title)
			if title != "" {
				patch.TitleSet = true
				patch.Title = &title
			}
		}
		if in.Description != nil {
			patch.DescriptionSet = true
			patch.Description = in.Description
		}
		if in.Price != nil {
			if in.Price.AmountMinor != nil && *in.Price.AmountMinor > 0 {
				curr := "TRY"
				if in.Price.Currency != nil && *in.Price.Currency != "" {
					curr = *in.Price.Currency
				}
				patch.PriceSet = true
				patch.Price = &domainadvert.Money{
					AmountMinor: *in.Price.AmountMinor,
					Currency:    curr,
				}
			}
		}
		if in.DistrictID != nil {
			if err := s.requireActiveDistrict(ctx, *in.DistrictID); err == nil {
				patch.DistrictIDSet = true
				patch.DistrictID = in.DistrictID
			}
		}
		if in.HorseID != nil {
			patch.HorseIDSet = true
			patch.HorseID = in.HorseID
		}
		if in.Properties != nil {
			// Merge existing properties with provided properties
			existingProps := map[string]interface{}{}
			if len(current.Properties) > 0 {
				_ = json.Unmarshal(current.Properties, &existingProps)
			}
			for k, v := range in.Properties {
				existingProps[k] = v
			}
			// Synchronize canonical category property codes across aliases
			syncCanonicalProperties(existingProps)

			marshaled, mErr := json.Marshal(existingProps)
			if mErr == nil {
				patch.PropertiesSet = true
				patch.Properties = marshaled
			}
		}

		updated, err = repo.UpdateDetailsAdmin(ctx, advertID, patch, expectedVer, now)
		if err != nil {
			return err
		}

		// Handle Media if provided
		if in.Media != nil {
			var mediaRels []domainadvert.MediaRelation
			hasCover := false
			for i, m := range in.Media {
				if m.IsCover {
					hasCover = true
				}
				mediaRels = append(mediaRels, domainadvert.MediaRelation{
					AssetID:      m.AssetID,
					DisplayOrder: i,
					IsCover:      m.IsCover,
				})
			}
			if !hasCover && len(mediaRels) > 0 {
				mediaRels[0].IsCover = true
			}
			if err := repo.ReplaceAdvertMediaAdmin(ctx, advertID, mediaRels, now); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return domainadvert.ModerationDetailView{}, err
	}

	return s.moderationDetail(ctx, updated)
}

// DeleteAdvert implements ADVERT-ADMIN-07: Permanently deletes an advert from DB.
// Allowed only if advert status is SUSPENDED, ARCHIVED, or REJECTED.
func (s *Service) DeleteAdvert(ctx context.Context, actorUserID uuid.UUID, advertID int64) error {
	return s.withTx(ctx, func(ctx context.Context, repo Repository, tx pgx.Tx) error {
		current, err := repo.FindByIDForUpdate(ctx, advertID)
		if err != nil {
			return err
		}
		if current.Status != domainadvert.StatusSuspended && current.Status != domainadvert.StatusArchived && current.Status != domainadvert.StatusRejected {
			return apperr.InvalidState("Sadece yayından kaldırılan veya reddedilen ilanlar silinebilir.")
		}
		return repo.HardDelete(ctx, advertID)
	})
}

// ApproveAdvert implements ADVERT-ADMIN-03: PENDING_REVIEW / SUSPENDED → PUBLISHED.
func (s *Service) ApproveAdvert(
	ctx context.Context,
	actorUserID uuid.UUID, advertID int64,
	expectedVersion int,
) (domainadvert.ModerationDetailView, error) {
	if err := requireExpectedVersion(expectedVersion); err != nil {
		return domainadvert.ModerationDetailView{}, err
	}

	var updated domainadvert.Advert
	now := s.clock.Now()
	err := s.withTx(ctx, func(ctx context.Context, repo Repository, tx pgx.Tx) error {
		current, err := repo.FindByIDForUpdate(ctx, advertID)
		if err != nil {
			return err
		}
		if current.Version != expectedVersion {
			return apperr.StaleVersion(staleVersionMessage)
		}
		if current.Status != domainadvert.StatusPendingReview && current.Status != domainadvert.StatusSuspended && current.Status != domainadvert.StatusArchived {
			return apperr.InvalidState(adminInvalidStateMessage)
		}
		if !domainadvert.AdminTransitionAllowed(current.Status, domainadvert.StatusPublished) {
			return apperr.Internal(
				fmt.Errorf("unsupported admin transition %s->%s", current.Status, domainadvert.StatusPublished),
			)
		}
		if err := s.validateForSubmission(ctx, current); err != nil {
			return err
		}
		var publishedAt *time.Time
		if current.PublishedAt != nil {
			publishedAt = current.PublishedAt
		} else {
			publishedAt = &now
		}
		fromStatus := current.Status
		updated, err = repo.TransitionStatus(
			ctx, current.OwnerUserID, advertID, fromStatus, domainadvert.StatusPublished, expectedVersion, publishedAt, now,
		)
		if err != nil {
			return err
		}
		return repo.InsertHistory(ctx, domainadvert.StatusHistory{
			ID:          uuid.New(),
			AdvertID:    advertID,
			FromStatus:  &fromStatus,
			ToStatus:    domainadvert.StatusPublished,
			ActorUserID: &actorUserID,
			IsSystem:    false,
			Reason:      nil,
			CreatedAt:   now,
		})
	})
	if err != nil {
		return domainadvert.ModerationDetailView{}, err
	}

	return s.moderationDetail(ctx, updated)
}

// RequestAdvertChanges implements ADVERT-ADMIN-04: PENDING_REVIEW → CHANGES_REQUESTED.
func (s *Service) RequestAdvertChanges(
	ctx context.Context,
	actorUserID uuid.UUID, advertID int64,
	in ModerationReasonInput,
) (domainadvert.ModerationDetailView, error) {
	reason, err := requireModerationReason(in.Reason)
	if err != nil {
		return domainadvert.ModerationDetailView{}, err
	}
	return s.adminTransition(ctx, actorUserID, advertID, in.ExpectedVersion, &reason,
		domainadvert.StatusPendingReview, domainadvert.StatusChangesRequested, false)
}

// RejectAdvert implements ADVERT-ADMIN-05: PENDING_REVIEW → REJECTED.
func (s *Service) RejectAdvert(
	ctx context.Context,
	actorUserID uuid.UUID, advertID int64,
	in ModerationReasonInput,
) (domainadvert.ModerationDetailView, error) {
	reason, err := requireModerationReason(in.Reason)
	if err != nil {
		return domainadvert.ModerationDetailView{}, err
	}
	return s.adminTransition(ctx, actorUserID, advertID, in.ExpectedVersion, &reason,
		domainadvert.StatusPendingReview, domainadvert.StatusRejected, false)
}

// SuspendAdvert implements ADVERT-ADMIN-06: PUBLISHED → SUSPENDED.
func (s *Service) SuspendAdvert(
	ctx context.Context,
	actorUserID uuid.UUID, advertID int64,
	in ModerationReasonInput,
) (domainadvert.ModerationDetailView, error) {
	reason, err := requireModerationReason(in.Reason)
	if err != nil {
		return domainadvert.ModerationDetailView{}, err
	}
	return s.adminTransition(ctx, actorUserID, advertID, in.ExpectedVersion, &reason,
		domainadvert.StatusPublished, domainadvert.StatusSuspended, false)
}

func (s *Service) adminTransition(
	ctx context.Context,
	actorUserID uuid.UUID, advertID int64,
	expectedVersion int,
	reason *string,
	from, to domainadvert.Status,
	setPublishedAt bool,
) (domainadvert.ModerationDetailView, error) {
	if err := requireExpectedVersion(expectedVersion); err != nil {
		return domainadvert.ModerationDetailView{}, err
	}
	if !domainadvert.AdminTransitionAllowed(from, to) {
		return domainadvert.ModerationDetailView{}, apperr.Internal(
			fmt.Errorf("unsupported admin transition %s->%s", from, to),
		)
	}

	var updated domainadvert.Advert
	now := s.clock.Now()
	err := s.withTx(ctx, func(ctx context.Context, repo Repository, tx pgx.Tx) error {
		current, err := repo.FindByIDForUpdate(ctx, advertID)
		if err != nil {
			return err
		}
		if current.Version != expectedVersion {
			return apperr.StaleVersion(staleVersionMessage)
		}
		if current.Status != from {
			return apperr.InvalidState(adminInvalidStateMessage)
		}
		if to == domainadvert.StatusPublished {
			if err := s.validateForSubmission(ctx, current); err != nil {
				return err
			}
		}
		var publishedAt *time.Time
		if setPublishedAt {
			publishedAt = &now
		}
		updated, err = repo.TransitionStatus(
			ctx, current.OwnerUserID, advertID, from, to, expectedVersion, publishedAt, now,
		)
		if err != nil {
			return err
		}
		fromStatus := from
		if err := repo.InsertHistory(ctx, domainadvert.StatusHistory{
			ID:          uuid.New(),
			AdvertID:    advertID,
			FromStatus:  &fromStatus,
			ToStatus:    to,
			ActorUserID: &actorUserID,
			IsSystem:    false,
			Reason:      reason,
			CreatedAt:   now,
		}); err != nil {
			return err
		}
		if s.notifications != nil && to == domainadvert.StatusPublished {
			return s.notifications.OnAdvertPublished(ctx, tx, advertID)
		}
		return nil
	})
	if err != nil {
		return domainadvert.ModerationDetailView{}, err
	}
	return s.moderationDetail(ctx, updated)
}

func (s *Service) moderationDetail(ctx context.Context, a domainadvert.Advert) (domainadvert.ModerationDetailView, error) {
	history, err := s.repo.ListStatusHistory(ctx, a.ID)
	if err != nil {
		return domainadvert.ModerationDetailView{}, err
	}
	if history == nil {
		history = []domainadvert.StatusHistory{}
	}
	ownerView := a.ToOwnerView()
	views, err := s.projectOwnerViews(ctx, []domainadvert.Advert{a})
	if err == nil && len(views) > 0 {
		ownerView = views[0]
	}
	return domainadvert.ModerationDetailView{
		OwnerView:     ownerView,
		OwnerUserID:   a.OwnerUserID,
		StatusHistory: history,
	}, nil
}

func requireModerationReason(raw string) (string, error) {
	reason := strings.TrimSpace(raw)
	if reason == "" {
		return "", apperr.Validation(invalidRequest, apperr.FieldError{
			Field:   "reason",
			Message: reasonRequiredMessage,
		})
	}
	return reason, nil
}

func syncCanonicalProperties(p map[string]interface{}) {
	getString := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := p[k]; ok && v != nil {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" && s != "-" {
					return strings.TrimSpace(s)
				}
			}
		}
		return ""
	}

	getBool := func(keys ...string) *bool {
		for _, k := range keys {
			if v, ok := p[k]; ok && v != nil {
				if b, ok := v.(bool); ok {
					return &b
				}
				if s, ok := v.(string); ok {
					lower := strings.ToLower(strings.TrimSpace(s))
					if lower == "true" || lower == "evet" || lower == "1" {
						b := true
						return &b
					}
					if lower == "false" || lower == "hayır" || lower == "hayir" || lower == "0" {
						b := false
						return &b
					}
				}
			}
		}
		return nil
	}

	// 1. Race Horse Flags
	if b := getBool("IS_FOR_RENT", "isForRent", "kiralikMi", "kiralik", "is_for_rent"); b != nil {
		p["IS_FOR_RENT"] = *b
		p["isForRent"] = *b
		if *b {
			p["kiralikMi"] = "Evet"
			p["kiralik"] = "Evet"
		} else {
			p["kiralikMi"] = "Hayır"
			p["kiralik"] = "Hayır"
		}
	}
	if b := getBool("IN_TRAINING", "inTraining", "idmandaMi", "idmanda", "in_training"); b != nil {
		p["IN_TRAINING"] = *b
		p["inTraining"] = *b
		if *b {
			p["idmandaMi"] = "Evet"
			p["idmanda"] = "Evet"
		} else {
			p["idmandaMi"] = "Hayır"
			p["idmanda"] = "Hayır"
		}
	}
	if b := getBool("IS_RACE_READY", "isRaceReady", "kosarDurumdaMi", "kosar", "is_race_ready"); b != nil {
		p["IS_RACE_READY"] = *b
		p["isRaceReady"] = *b
		if *b {
			p["kosarDurumdaMi"] = "Evet"
			p["kosar"] = "Evet"
		} else {
			p["kosarDurumdaMi"] = "Hayır"
			p["kosar"] = "Hayır"
		}
	}

	// 2. Horse specs & pedigree
	if s := getString("REGISTERED_NAME", "atAdi", "horseName", "studHorseName", "registered_name"); s != "" {
		p["REGISTERED_NAME"] = s
		p["atAdi"] = s
		p["horseName"] = s
	}
	if s := getString("HORSE_BREED", "atIrki", "breed", "STALLION_BREED", "studBreed", "horseBreed"); s != "" {
		p["HORSE_BREED"] = s
		p["atIrki"] = s
		p["breed"] = s
		p["STALLION_BREED"] = s
	}
	if s := getString("HORSE_AGE", "yas", "age", "STALLION_AGE", "studAge", "horseAge"); s != "" {
		p["HORSE_AGE"] = s
		p["yas"] = s
		p["age"] = s
		p["STALLION_AGE"] = s
	}
	if s := getString("HORSE_GENDER", "cinsiyet", "gender", "horseGender"); s != "" {
		p["HORSE_GENDER"] = s
		p["cinsiyet"] = s
		p["gender"] = s
	}
	if s := getString("COAT_COLOR", "donu", "don", "coatColor", "studCoatColor", "coat_color"); s != "" {
		p["COAT_COLOR"] = s
		p["donu"] = s
		p["don"] = s
		p["coatColor"] = s
	}
	if s := getString("SIRE", "baba", "sire", "studSire"); s != "" {
		p["SIRE"] = s
		p["baba"] = s
		p["sire"] = s
	}
	if s := getString("DAM", "anne", "dam", "studDam"); s != "" {
		p["DAM"] = s
		p["anne"] = s
		p["dam"] = s
	}
	if s := getString("DAMSIRE", "anneBabasi", "damsire", "studDamsire", "kisrakBabasi"); s != "" {
		p["DAMSIRE"] = s
		p["anneBabasi"] = s
		p["damsire"] = s
	}
	if s := getString("TJK_NUMBER", "tjkNumber", "tjkNo", "tjk_number"); s != "" {
		p["TJK_NUMBER"] = s
		p["tjkNumber"] = s
		p["tjkNo"] = s
	}
	if s := getString("HEIGHT_CM", "heightCm", "cidago", "height_cm"); s != "" {
		p["HEIGHT_CM"] = s
		p["heightCm"] = s
		p["cidago"] = s
	}

	// 3. Mare
	if b := getBool("IS_PREGNANT", "isPregnant", "gebeMi", "gebe"); b != nil {
		p["IS_PREGNANT"] = *b
		p["isPregnant"] = *b
	}
	if s := getString("COVERING_STALLION", "coveringStallion", "gebeOlduguAygir"); s != "" {
		p["COVERING_STALLION"] = s
		p["coveringStallion"] = s
	}
	if s := getString("PREGNANCY_STAGE", "pregnancyStage", "gebelikDurumu"); s != "" {
		p["PREGNANCY_STAGE"] = s
		p["pregnancyStage"] = s
	}
	if s := getString("LAST_COVERING_DATE", "lastCoveringDate", "sonAsimTarihi"); s != "" {
		p["LAST_COVERING_DATE"] = s
		p["lastCoveringDate"] = s
	}

	// 4. Farrier
	if b := getBool("SICAK_UYGULAMA", "sicakUygulama", "sicak_uygulama", "sicak"); b != nil {
		p["SICAK_UYGULAMA"] = *b
		p["sicakUygulama"] = *b
		p["sicak_uygulama"] = *b
	}

	// 5. Facility / Pansiyon
	if b := getBool("grassPaddock", "facilityGrassPaddock", "cimPadok"); b != nil {
		p["grassPaddock"] = *b
		p["facilityGrassPaddock"] = *b
	}
	if b := getBool("sandPaddock", "facilitySandPaddock", "kumPadok"); b != nil {
		p["sandPaddock"] = *b
		p["facilitySandPaddock"] = *b
	}
	if b := getBool("stallionPaddock", "facilityStallionPaddock", "aygirPadogu"); b != nil {
		p["stallionPaddock"] = *b
		p["facilityStallionPaddock"] = *b
	}
	if b := getBool("veterinarian", "facilityVeterinarian", "vet", "veteriner"); b != nil {
		p["veterinarian"] = *b
		p["facilityVeterinarian"] = *b
		p["vet"] = *b
	}
	if b := getBool("farrier", "facilityFarrier", "nalbant"); b != nil {
		p["farrier"] = *b
		p["facilityFarrier"] = *b
		p["nalbant"] = *b
	}
	if b := getBool("maternity", "facilityFoalingBarn", "foalingBarn", "dogumhane"); b != nil {
		p["maternity"] = *b
		p["facilityFoalingBarn"] = *b
		p["foalingBarn"] = *b
	}
}

