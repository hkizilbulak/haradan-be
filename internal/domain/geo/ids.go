package geo

import (
	"fmt"

	"github.com/google/uuid"
)

// CatalogNamespace is a stable UUID v5 namespace for Turkey admin codes.
// IDs must survive catalog refreshes so advert foreign keys stay valid.
var CatalogNamespace = uuid.MustParse("3c9f0a7e-6d21-5b84-9c1a-2e8f47b0d5c3")

// StableProvinceID returns a deterministic UUID for a Turkey plate code (1–81).
func StableProvinceID(plate int) uuid.UUID {
	return uuid.NewSHA1(CatalogNamespace, []byte(fmt.Sprintf("tr:province:%d", plate)))
}

// StableDistrictID returns a deterministic UUID for an official district code.
func StableDistrictID(officialID int) uuid.UUID {
	return uuid.NewSHA1(CatalogNamespace, []byte(fmt.Sprintf("tr:district:%d", officialID)))
}
