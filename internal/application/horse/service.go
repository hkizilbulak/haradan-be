package horse

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainhorse "github.com/hkizilbulak/haradan-be/internal/domain/horse"
	"github.com/hkizilbulak/haradan-be/internal/platform/textnorm"
)

const (
	defaultSearchLimit = 20
	minSearchLimit     = 1
	maxSearchLimit     = 100
	maxTJKNumberLen    = 64
)

// Service implements Horse use cases (DB-backed; no live TJK).
type Service struct {
	repo domainhorse.Repository
}

// NewService constructs a Horse application service.
func NewService(repo domainhorse.Repository) *Service {
	return &Service{repo: repo}
}

// SearchForSelection implements HORSE-01 against local hrd_horses only.
func (s *Service) SearchForSelection(ctx context.Context, q *string, tjkNumber *string, limit *int) ([]domainhorse.SelectionProjection, error) {
	lim, err := resolveLimit(limit)
	if err != nil {
		return nil, err
	}
	prefix := normalizeQuery(q)
	tjk, err := normalizeTJKNumber(tjkNumber)
	if err != nil {
		return nil, err
	}
	if prefix == "" && tjk == "" {
		return []domainhorse.SelectionProjection{}, nil
	}

	var horses []domainhorse.Horse
	switch {
	case tjk != "" && prefix == "":
		h, err := s.repo.FindByTJKNumber(ctx, tjk)
		if err != nil {
			if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
				return []domainhorse.SelectionProjection{}, nil
			}
			return nil, mapRepoErr(err)
		}
		horses = []domainhorse.Horse{h}
	case tjk != "" && prefix != "":
		h, err := s.repo.FindByTJKNumber(ctx, tjk)
		if err != nil {
			if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindNotFound {
				return []domainhorse.SelectionProjection{}, nil
			}
			return nil, mapRepoErr(err)
		}
		if !strings.HasPrefix(h.NameNormalized, prefix) {
			return []domainhorse.SelectionProjection{}, nil
		}
		horses = []domainhorse.Horse{h}
	default:
		items, err := s.repo.SearchByNormalizedPrefix(ctx, prefix, lim)
		if err != nil {
			return nil, mapRepoErr(err)
		}
		horses = items
	}

	out := make([]domainhorse.SelectionProjection, 0, len(horses))
	for _, h := range horses {
		out = append(out, toSelection(h))
	}
	return out, nil
}

// GetPublicDetail implements HORSE-02.
func (s *Service) GetPublicDetail(ctx context.Context, id uuid.UUID) (domainhorse.PublicDetail, error) {
	h, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return domainhorse.PublicDetail{}, mapRepoErr(err)
	}
	return toPublicDetail(h), nil
}

func toSelection(h domainhorse.Horse) domainhorse.SelectionProjection {
	return domainhorse.SelectionProjection{
		ID:           h.ID,
		OriginalName: h.OriginalName,
		TJKNumber:    h.TJKNumber,
		BirthYear:    h.BirthYear,
		SireName:     h.SireName,
		DamName:      h.DamName,
	}
}

func toPublicDetail(h domainhorse.Horse) domainhorse.PublicDetail {
	detail := h.Detail
	if len(detail) == 0 {
		detail = json.RawMessage(`{}`)
	}
	return domainhorse.PublicDetail{
		ID:           h.ID,
		OriginalName: h.OriginalName,
		TJKNumber:    h.TJKNumber,
		BirthYear:    h.BirthYear,
		SireName:     h.SireName,
		DamName:      h.DamName,
		Breed:        h.Breed,
		Gender:       h.Gender,
		Coat:         h.Coat,
		Detail:       detail,
	}
}

func normalizeQuery(q *string) string {
	if q == nil {
		return ""
	}
	return textnorm.TurkishFold(strings.TrimSpace(*q))
}

func normalizeTJKNumber(tjkNumber *string) (string, error) {
	if tjkNumber == nil {
		return "", nil
	}
	v := strings.TrimSpace(*tjkNumber)
	if v == "" {
		return "", nil
	}
	if len(v) > maxTJKNumberLen {
		return "", apperr.Validation("Geçersiz istek.", apperr.FieldError{
			Field:   "tjkNumber",
			Message: "TJK numarası çok uzun.",
		})
	}
	return v, nil
}

func resolveLimit(limit *int) (int, error) {
	if limit == nil {
		return defaultSearchLimit, nil
	}
	if *limit < minSearchLimit || *limit > maxSearchLimit {
		return 0, apperr.Validation("Geçersiz limit değeri.", apperr.FieldError{
			Field:   "limit",
			Message: "limit 1 ile 100 arasında olmalıdır",
		})
	}
	return *limit, nil
}

func mapRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if e, ok := apperr.As(err); ok {
		return e
	}
	return apperr.WrapInternal(err)
}
