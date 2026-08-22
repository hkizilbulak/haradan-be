package advert

import (
	"time"

	"github.com/google/uuid"
)

// Public projection types cross the application/infrastructure boundary.
type PublicCard struct {
	ID                 uuid.UUID
	CategoryID         uuid.UUID
	DistrictID         uuid.UUID
	ProvinceID         uuid.UUID
	HorseID            *uuid.UUID
	Title              string
	Price              *Money
	PublishedAt        time.Time
	Cover              *PublicMedia
	PackageCode        *string
	PackageDisplayName *string
	PackageBadgeText   *string
	IsUrgent           bool
	UrgentActivatedAt  *time.Time
	IsFeatured         bool
	FeaturedUntil      *time.Time
	IsFavorite         *bool
	SearchPriority     int
	ViewCount          int
}
type PublicMedia struct {
	AssetID      uuid.UUID
	DisplayOrder int
	IsCover      bool
	ObjectKey    string
	Usage        *string
}
type PublicProperty struct {
	Code         string
	Title        string
	Value        any
	DisplayValue *string
}
type PublicHorse struct {
	ID        uuid.UUID
	Name      string
	TJKNumber *string
}
type PublicDetail struct {
	PublicCard
	Description  string
	Address      *string
	CategoryName string
	CategorySlug string
	DistrictName string
	ProvinceName string
	Horse        *PublicHorse
	Properties   []PublicProperty
	Media        []PublicMedia
	SellerPhone  *string
	SellerID     *uuid.UUID
}
type PublicCursor struct {
	Priority    int
	PublishedAt time.Time
	ID          uuid.UUID
}
type HomepageCursor struct {
	PublishedAt time.Time
	ID          uuid.UUID
}
type PublicSearchQuery struct {
	CategoryID  *uuid.UUID
	ProvinceID  *uuid.UUID
	DistrictID  *uuid.UUID
	HorseID     *uuid.UUID
	HasPhoto    *bool
	ActorUserID *uuid.UUID
	After       *PublicCursor
	Limit       int
}
type HomepageNewQuery struct {
	ActorUserID *uuid.UUID
	After       *HomepageCursor
	Limit       int
}
