package geo

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaingeo "github.com/hkizilbulak/haradan-be/internal/domain/geo"
	"github.com/hkizilbulak/haradan-be/internal/platform/textnorm"
)

const (
	defaultSearchLimit = 20
	minSearchLimit     = 1
	maxSearchLimit     = 100
)

// Service implements Geo use cases.
type Service struct {
	repo domaingeo.Repository
}

// NewService constructs a Geo application service.
func NewService(repo domaingeo.Repository) *Service {
	return &Service{repo: repo}
}

// ListActiveProvinces returns active provinces in deterministic order.
func (s *Service) ListActiveProvinces(ctx context.Context) ([]domaingeo.Province, error) {
	items, err := s.repo.ListActiveProvinces(ctx)
	if err != nil {
		return nil, apperr.WrapInternal(err)
	}
	if items == nil {
		return []domaingeo.Province{}, nil
	}
	return items, nil
}

// SearchProvinces searches active provinces by Turkish-normalized prefix.
func (s *Service) SearchProvinces(ctx context.Context, q *string, limit *int) ([]domaingeo.Province, error) {
	lim, err := resolveLimit(limit)
	if err != nil {
		return nil, err
	}
	prefix := normalizeQuery(q)
	if prefix == "" {
		return []domaingeo.Province{}, nil
	}
	items, err := s.repo.SearchActiveProvincesByNormalizedPrefix(ctx, prefix, lim)
	if err != nil {
		return nil, apperr.WrapInternal(err)
	}
	if items == nil {
		return []domaingeo.Province{}, nil
	}
	return items, nil
}

// ListDistrictsByProvince lists active districts for an existing active province.
func (s *Service) ListDistrictsByProvince(ctx context.Context, provinceID uuid.UUID) ([]domaingeo.District, error) {
	if _, err := s.repo.GetActiveProvinceID(ctx, provinceID); err != nil {
		return nil, mapRepoErr(err)
	}
	items, err := s.repo.ListActiveDistrictsByProvince(ctx, provinceID)
	if err != nil {
		return nil, apperr.WrapInternal(err)
	}
	if items == nil {
		return []domaingeo.District{}, nil
	}
	return items, nil
}

// SearchDistricts searches active districts by prefix with optional province scope.
func (s *Service) SearchDistricts(ctx context.Context, q *string, provinceID *uuid.UUID, limit *int) ([]domaingeo.District, error) {
	lim, err := resolveLimit(limit)
	if err != nil {
		return nil, err
	}
	if provinceID != nil {
		if _, err := s.repo.GetActiveProvinceID(ctx, *provinceID); err != nil {
			return nil, mapRepoErr(err)
		}
	}
	prefix := normalizeQuery(q)
	if prefix == "" {
		return []domaingeo.District{}, nil
	}
	items, err := s.repo.SearchActiveDistrictsByNormalizedPrefix(ctx, prefix, provinceID, lim)
	if err != nil {
		return nil, apperr.WrapInternal(err)
	}
	if items == nil {
		return []domaingeo.District{}, nil
	}
	return items, nil
}

func normalizeQuery(q *string) string {
	if q == nil {
		return ""
	}
	return textnorm.TurkishFold(strings.TrimSpace(*q))
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
