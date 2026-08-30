package advert

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
)

const (
	defaultPublicLimit   = 20
	maxPublicLimit       = 100
	defaultShowcaseLimit = 20
	maxShowcaseLimit     = 50
)

// PublicSearchInput contains the buyer-facing advert search filters.
type PublicSearchInput struct {
	Cursor          *string
	Limit           *int
	CategoryID      *uuid.UUID
	ProvinceID      *uuid.UUID
	DistrictID      *uuid.UUID
	HorseID         *uuid.UUID
	HasPhoto        *bool
	Sort            *string
	PropertyFilters *string
	ActorUserID     *uuid.UUID
}

type PublicCard = domainadvert.PublicCard
type PublicMedia = domainadvert.PublicMedia
type PublicProperty = domainadvert.PublicProperty
type PublicDetail = domainadvert.PublicDetail
type PublicHorse = domainadvert.PublicHorse
type PublicCursor = domainadvert.PublicCursor
type HomepageCursor = domainadvert.HomepageCursor

type PublicSearchResult struct {
	Items      []PublicCard
	NextCursor *string
	HasMore    bool
}

type HomepageShowcaseResult struct {
	Seed  string
	Items []PublicCard
}

// PublicEnabled reports whether the service has a buyer-facing projection
// repository. Foundation and owner-only test wiring intentionally omit it.
func (s *Service) PublicEnabled() bool { return s.public != nil }

// SearchPublishedAdverts returns published, non-deleted adverts. Property
// filters deliberately fail closed until a validated filter grammar exists.
func (s *Service) SearchPublishedAdverts(ctx context.Context, in PublicSearchInput) (PublicSearchResult, error) {
	if s.public == nil {
		return PublicSearchResult{}, apperr.Internal(fmt.Errorf("public advert repository is not configured"))
	}
	if in.PropertyFilters != nil && strings.TrimSpace(*in.PropertyFilters) != "" {
		return PublicSearchResult{}, apperr.Validation("Desteklenmeyen özellik filtresi.", apperr.FieldError{
			Field: "propertyFilters", Message: "Özellik filtreleri henüz desteklenmiyor.",
		})
	}
	limit, err := resolvePublicLimit(in.Limit, defaultPublicLimit, maxPublicLimit)
	if err != nil {
		return PublicSearchResult{}, err
	}
	var after *PublicCursor
	if in.Cursor != nil && strings.TrimSpace(*in.Cursor) != "" {
		after, err = decodePublicCursor(strings.TrimSpace(*in.Cursor))
		if err != nil {
			return PublicSearchResult{}, apperr.Validation("Geçersiz cursor.", apperr.FieldError{Field: "cursor", Message: "Geçersiz cursor."})
		}
	}
	rows, err := s.public.SearchPublished(ctx, domainadvert.PublicSearchQuery{
		CategoryID: in.CategoryID, ProvinceID: in.ProvinceID, DistrictID: in.DistrictID, HorseID: in.HorseID,
		HasPhoto: in.HasPhoto, ActorUserID: in.ActorUserID, After: after, Limit: limit + 1,
	})
	if err != nil {
		return PublicSearchResult{}, err
	}
	return publicPage(rows, limit), nil
}

func (s *Service) ListHomepageNewAdverts(ctx context.Context, cursor *string, limit *int, actorUserID *uuid.UUID) (PublicSearchResult, error) {
	if s.public == nil {
		return PublicSearchResult{}, apperr.Internal(fmt.Errorf("public advert repository is not configured"))
	}
	resolved, err := resolvePublicLimit(limit, defaultPublicLimit, maxPublicLimit)
	if err != nil {
		return PublicSearchResult{}, err
	}
	var after *HomepageCursor
	if cursor != nil && strings.TrimSpace(*cursor) != "" {
		after, err = decodeHomepageCursor(strings.TrimSpace(*cursor))
		if err != nil {
			return PublicSearchResult{}, apperr.Validation("Geçersiz cursor.", apperr.FieldError{Field: "cursor", Message: "Geçersiz cursor."})
		}
	}
	rows, err := s.public.ListHomepageNew(ctx, domainadvert.HomepageNewQuery{ActorUserID: actorUserID, After: after, Limit: resolved + 1})
	if err != nil {
		return PublicSearchResult{}, err
	}
	return homepagePage(rows, resolved), nil
}

func (s *Service) ListHomepageShowcase(ctx context.Context, seed *string, limit *int, actorUserID *uuid.UUID) (HomepageShowcaseResult, error) {
	if s.public == nil {
		return HomepageShowcaseResult{}, apperr.Internal(fmt.Errorf("public advert repository is not configured"))
	}
	resolved, err := resolvePublicLimit(limit, defaultShowcaseLimit, maxShowcaseLimit)
	if err != nil {
		return HomepageShowcaseResult{}, err
	}
	outSeed := ""
	if seed != nil {
		outSeed = strings.TrimSpace(*seed)
	}
	if outSeed == "" {
		buf := make([]byte, 24)
		if _, err := rand.Read(buf); err != nil {
			return HomepageShowcaseResult{}, apperr.Internal(fmt.Errorf("generate showcase seed: %w", err))
		}
		outSeed = base64.RawURLEncoding.EncodeToString(buf)
	}
	items, err := s.public.ListHomepageShowcase(ctx, outSeed, resolved, actorUserID)
	if err != nil {
		return HomepageShowcaseResult{}, err
	}
	return HomepageShowcaseResult{Seed: outSeed, Items: items}, nil
}

func (s *Service) ListHomepageUrgent(ctx context.Context, limit *int, actorUserID *uuid.UUID) (PublicSearchResult, error) {
	if s.public == nil {
		return PublicSearchResult{}, apperr.Internal(fmt.Errorf("public advert repository is not configured"))
	}
	resolved, err := resolvePublicLimit(limit, defaultShowcaseLimit, maxShowcaseLimit)
	if err != nil {
		return PublicSearchResult{}, err
	}
	items, err := s.public.ListHomepageUrgent(ctx, resolved, actorUserID)
	if err != nil {
		return PublicSearchResult{}, err
	}
	return PublicSearchResult{Items: items, HasMore: false}, nil
}

func (s *Service) ListHomepageFeatured(ctx context.Context, limit *int, actorUserID *uuid.UUID) (PublicSearchResult, error) {
	if s.public == nil {
		return PublicSearchResult{}, apperr.Internal(fmt.Errorf("public advert repository is not configured"))
	}
	resolved, err := resolvePublicLimit(limit, defaultShowcaseLimit, maxShowcaseLimit)
	if err != nil {
		return PublicSearchResult{}, err
	}
	items, err := s.public.ListHomepageFeatured(ctx, resolved, actorUserID)
	if err != nil {
		return PublicSearchResult{}, err
	}
	return PublicSearchResult{Items: items, HasMore: false}, nil
}

// HomepageBootstrapResult is the advert-feed portion of GET /v1/homepage.
type HomepageBootstrapResult struct {
	NewAdverts PublicSearchResult
	Urgent     PublicSearchResult
	Featured   PublicSearchResult
	Showcase   HomepageShowcaseResult
}

// GetHomepageBootstrap loads the four homepage advert feeds concurrently.
func (s *Service) GetHomepageBootstrap(ctx context.Context, limit *int, actorUserID *uuid.UUID) (HomepageBootstrapResult, error) {
	if s.public == nil {
		return HomepageBootstrapResult{}, apperr.Internal(fmt.Errorf("public advert repository is not configured"))
	}

	type slot struct {
		kind string
		page PublicSearchResult
		show HomepageShowcaseResult
		err  error
	}
	ch := make(chan slot, 4)

	go func() {
		v, err := s.ListHomepageNewAdverts(ctx, nil, limit, actorUserID)
		ch <- slot{kind: "new", page: v, err: err}
	}()
	go func() {
		v, err := s.ListHomepageUrgent(ctx, limit, actorUserID)
		ch <- slot{kind: "urgent", page: v, err: err}
	}()
	go func() {
		v, err := s.ListHomepageFeatured(ctx, limit, actorUserID)
		ch <- slot{kind: "featured", page: v, err: err}
	}()
	go func() {
		v, err := s.ListHomepageShowcase(ctx, nil, limit, actorUserID)
		ch <- slot{kind: "showcase", show: v, err: err}
	}()

	var out HomepageBootstrapResult
	for i := 0; i < 4; i++ {
		item := <-ch
		if item.err != nil {
			return HomepageBootstrapResult{}, item.err
		}
		switch item.kind {
		case "new":
			out.NewAdverts = item.page
		case "urgent":
			out.Urgent = item.page
		case "featured":
			out.Featured = item.page
		case "showcase":
			out.Showcase = item.show
		}
	}
	return out, nil
}

func (s *Service) GetPublishedAdvertDetail(ctx context.Context, advertID uuid.UUID, actorUserID *uuid.UUID, clientIP string) (PublicDetail, error) {
	if s.public == nil {
		return PublicDetail{}, apperr.Internal(fmt.Errorf("public advert repository is not configured"))
	}
	if clientIP != "" {
		_ = s.public.RecordView(ctx, advertID, clientIP)
	}
	return s.public.GetPublishedDetail(ctx, advertID, actorUserID)
}

func resolvePublicLimit(limit *int, fallback, maximum int) (int, error) {
	if limit == nil {
		return fallback, nil
	}
	if *limit < 1 || *limit > maximum {
		return 0, apperr.Validation(fmt.Sprintf("limit 1 ile %d arasında olmalıdır.", maximum), apperr.FieldError{
			Field: "limit", Message: fmt.Sprintf("limit 1 ile %d arasında olmalıdır.", maximum),
		})
	}
	return *limit, nil
}

func publicPage(rows []PublicCard, limit int) PublicSearchResult {
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := PublicSearchResult{Items: rows, HasMore: hasMore}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		cursor := encodePublicCursor(last.SearchPriority, last.PublishedAt, last.ID)
		out.NextCursor = &cursor
	}
	return out
}

func homepagePage(rows []PublicCard, limit int) PublicSearchResult {
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := PublicSearchResult{Items: rows, HasMore: hasMore}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		cursor := encodeHomepageCursor(last.PublishedAt, last.ID)
		out.NextCursor = &cursor
	}
	return out
}

func encodePublicCursor(priority int, publishedAt time.Time, id uuid.UUID) string {
	raw := fmt.Sprintf("%d|%s|%s", priority, publishedAt.UTC().Format(time.RFC3339Nano), id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodePublicCursor(v string) (*PublicCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid cursor")
	}
	var priority int
	if _, err := fmt.Sscanf(parts[0], "%d", &priority); err != nil {
		return nil, err
	}
	publishedAt, err := time.Parse(time.RFC3339Nano, parts[1])
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(parts[2])
	if err != nil || id == uuid.Nil {
		return nil, fmt.Errorf("invalid cursor id")
	}
	return &PublicCursor{Priority: priority, PublishedAt: publishedAt, ID: id}, nil
}

func encodeHomepageCursor(publishedAt time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(publishedAt.UTC().Format(time.RFC3339Nano) + "|" + id.String()))
}

func decodeHomepageCursor(v string) (*HomepageCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid cursor")
	}
	publishedAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(parts[1])
	if err != nil || id == uuid.Nil {
		return nil, fmt.Errorf("invalid cursor id")
	}
	return &HomepageCursor{PublishedAt: publishedAt, ID: id}, nil
}
