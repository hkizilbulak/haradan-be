// Package packaging holds package catalog, advert package assignments, and
// URGENT feature activation aggregates aligned with migrations 00009/00011.
package packaging

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PackageCode is a dynamic catalog code (uppercase-normalized).
// Format: [A-Z0-9][A-Z0-9_-]{1,63}
type PackageCode string

var packageCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{1,63}$`)

// Valid reports whether c matches the controlled package code format.
func (c PackageCode) Valid() bool {
	return packageCodePattern.MatchString(string(c))
}

// NormalizePackageCode trims and uppercases an external code value.
func NormalizePackageCode(v string) PackageCode {
	return PackageCode(strings.ToUpper(strings.TrimSpace(v)))
}

// ParsePackageCode converts an external value into a normalized PackageCode.
func ParsePackageCode(v string) (PackageCode, bool) {
	c := NormalizePackageCode(v)
	return c, c.Valid()
}

// AssignmentStatus is the assignment lifecycle CHECK set.
type AssignmentStatus string

const (
	AssignmentStatusActive     AssignmentStatus = "ACTIVE"
	AssignmentStatusSuperseded AssignmentStatus = "SUPERSEDED"
	AssignmentStatusExpired    AssignmentStatus = "EXPIRED"
	AssignmentStatusCancelled  AssignmentStatus = "CANCELLED"
)

// Valid reports whether s is a known assignment status.
func (s AssignmentStatus) Valid() bool {
	switch s {
	case AssignmentStatusActive, AssignmentStatusSuperseded, AssignmentStatusExpired, AssignmentStatusCancelled:
		return true
	}
	return false
}

// AssignmentSource is the assignment source CHECK set.
type AssignmentSource string

const (
	AssignmentSourceAdmin   AssignmentSource = "ADMIN"
	AssignmentSourceSystem  AssignmentSource = "SYSTEM"
	AssignmentSourcePayment AssignmentSource = "PAYMENT"
)

// Valid reports whether s is a known assignment source.
func (s AssignmentSource) Valid() bool {
	switch s {
	case AssignmentSourceAdmin, AssignmentSourceSystem, AssignmentSourcePayment:
		return true
	}
	return false
}

// FeatureCode is the feature activation CHECK set.
type FeatureCode string

const (
	FeatureCodeUrgent   FeatureCode = "URGENT"
	FeatureCodeFeatured FeatureCode = "FEATURED"
)

// Valid reports whether c is a known feature code.
func (c FeatureCode) Valid() bool {
	switch c {
	case FeatureCodeUrgent, FeatureCodeFeatured:
		return true
	}
	return false
}

// FeatureActivationStatus is the feature activation lifecycle CHECK set.
type FeatureActivationStatus string

const (
	FeatureActivationStatusActive      FeatureActivationStatus = "ACTIVE"
	FeatureActivationStatusDeactivated FeatureActivationStatus = "DEACTIVATED"
)

// Valid reports whether s is a known feature activation status.
func (s FeatureActivationStatus) Valid() bool {
	switch s {
	case FeatureActivationStatusActive, FeatureActivationStatusDeactivated:
		return true
	}
	return false
}

// Package is the catalog row from hrd_packages.
type Package struct {
	ID                      uuid.UUID
	Code                    PackageCode
	DisplayName             string
	Description             *string
	BadgeText               *string
	BenefitsJSON            []byte
	DisplayPriceAmountMinor *int64
	CurrencyCode            string
	DefaultDurationDays     *int
	AllowsUrgent            bool
	ShowcaseEligible        bool
	FeaturedDays            *int
	SearchPriority          int
	BroadcastOnPublish      bool
	IsActive                bool
	SortOrder               int
	Version                 int
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// AllowsUrgentFeature reports whether this package may host URGENT.
func (p Package) AllowsUrgentFeature() bool {
	return p.AllowsUrgent
}

// FeaturedDurationDays returns the timed FEATURED window when the package includes it.
func (p Package) FeaturedDurationDays() (int, bool) {
	if p.FeaturedDays == nil || *p.FeaturedDays <= 0 {
		return 0, false
	}
	return *p.FeaturedDays, true
}

// EmitsPublishBroadcast reports whether assigning/publishing with this package
// should fan out the global PACKAGE_ADVERT_PUBLISHED notification.
func (p Package) EmitsPublishBroadcast() bool {
	return p.BroadcastOnPublish
}

// AdvertPackageAssignment is a package entitlement history row.
type AdvertPackageAssignment struct {
	ID               uuid.UUID
	AdvertID         uuid.UUID
	PackageID        uuid.UUID
	Status           AssignmentStatus
	StartsAt         time.Time
	EndsAt           *time.Time
	AssignedByUserID uuid.UUID
	AssignedAt       time.Time
	SupersededAt     *time.Time
	ExpiredAt        *time.Time
	CancelledAt      *time.Time
	Reason           *string
	Source           AssignmentSource
	Version          int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// IsActiveStatus reports status == ACTIVE (does not evaluate the time window).
func (a AdvertPackageAssignment) IsActiveStatus() bool {
	return a.Status == AssignmentStatusActive
}

// IsEffectiveAt reports whether the ACTIVE assignment covers instant t.
func (a AdvertPackageAssignment) IsEffectiveAt(t time.Time) bool {
	if a.Status != AssignmentStatusActive {
		return false
	}
	if t.Before(a.StartsAt) {
		return false
	}
	if a.EndsAt != nil && !t.Before(*a.EndsAt) {
		return false
	}
	return true
}

// ValidTimeRange reports starts_at <= ends_at when ends_at is set.
func ValidTimeRange(startsAt time.Time, endsAt *time.Time) bool {
	if endsAt == nil {
		return true
	}
	return !endsAt.Before(startsAt)
}

// AdvertFeatureActivation is an URGENT / FEATURED activation row.
type AdvertFeatureActivation struct {
	ID                  uuid.UUID
	AdvertID            uuid.UUID
	PackageAssignmentID uuid.UUID
	FeatureCode         FeatureCode
	Status              FeatureActivationStatus
	ActivatedByUserID   uuid.UUID
	ActivatedAt         time.Time
	EndsAt              *time.Time
	DeactivatedAt       *time.Time
	Reason              *string
	ActivationVersion   int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// IsActive reports status == ACTIVE (does not evaluate ends_at).
func (a AdvertFeatureActivation) IsActive() bool {
	return a.Status == FeatureActivationStatusActive
}

// IsEffectiveAt reports whether an ACTIVE activation covers instant t.
func (a AdvertFeatureActivation) IsEffectiveAt(t time.Time) bool {
	if a.Status != FeatureActivationStatusActive {
		return false
	}
	if a.EndsAt != nil && !t.Before(*a.EndsAt) {
		return false
	}
	return true
}

// ValidActivationVersion reports activation_version >= 1.
func ValidActivationVersion(v int) bool {
	return v >= 1
}

// AdvertStatusAllowsUrgent reports whether an advert lifecycle status may hold
// an URGENT activation. Publish-time notification is a later layer; DRAFT and
// PENDING_REVIEW may activate without emitting events here.
func AdvertStatusAllowsUrgent(status string) bool {
	switch status {
	case "DRAFT", "PENDING_REVIEW", "PUBLISHED":
		return true
	default:
		return false
	}
}
