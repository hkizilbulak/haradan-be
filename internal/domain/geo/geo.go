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

// Catalog is a full official province/district snapshot.
type Catalog struct {
	Provinces []Province
	Districts []District
}

// Repository reads province and district reference data.
type Repository interface {
	ListActiveProvinces(ctx context.Context) ([]Province, error)
	SearchActiveProvincesByNormalizedPrefix(ctx context.Context, prefix string, limit int) ([]Province, error)
	GetActiveProvinceID(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	GetActiveDistrict(ctx context.Context, id uuid.UUID) (District, error)
	ListActiveDistrictsByProvince(ctx context.Context, provinceID uuid.UUID) ([]District, error)
	SearchActiveDistrictsByNormalizedPrefix(ctx context.Context, prefix string, provinceID *uuid.UUID, limit int) ([]District, error)
}

// CatalogStore materializes a live official catalog into local reference rows.
// Adverts keep a district foreign key; the picker source is the live catalog.
type CatalogStore interface {
	CountActiveProvinces(ctx context.Context) (int, error)
	ReplaceCatalog(ctx context.Context, provinces []Province, districts []District) error
}
