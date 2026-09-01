package advert

import (
	"context"
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
	items := make([]domainadvert.OwnerView, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.ToOwnerView())
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

// ApproveAdvert implements ADVERT-ADMIN-03: PENDING_REVIEW → PUBLISHED.
func (s *Service) ApproveAdvert(
	ctx context.Context,
	actorUserID uuid.UUID, advertID int64,
	expectedVersion int,
) (domainadvert.ModerationDetailView, error) {
	return s.adminTransition(ctx, actorUserID, advertID, expectedVersion, nil,
		domainadvert.StatusPendingReview, domainadvert.StatusPublished, true)
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
	return domainadvert.ModerationDetailView{
		OwnerView:     a.ToOwnerView(),
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
