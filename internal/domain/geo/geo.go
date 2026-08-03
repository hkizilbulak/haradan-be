package geo

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Province is a geo province entity.
type Province struct {
	ID        uuid.UUID
	Name      string
	SortOrder int
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// District is a geo district entity.
type District struct {
	ID         uuid.UUID
	ProvinceID uuid.UUID
	Name       string
	SortOrder  int
	IsActive   bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Repository reads province and district reference data.
type Repository interface {
	ListActiveProvinces(ctx context.Context) ([]Province, error)
	SearchActiveProvincesByNormalizedPrefix(ctx context.Context, prefix string, limit int) ([]Province, error)
	GetActiveProvinceID(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	ListActiveDistrictsByProvince(ctx context.Context, provinceID uuid.UUID) ([]District, error)
	SearchActiveDistrictsByNormalizedPrefix(ctx context.Context, prefix string, provinceID *uuid.UUID, limit int) ([]District, error)
}
