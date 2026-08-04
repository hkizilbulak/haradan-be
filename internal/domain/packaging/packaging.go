// Package packaging holds package catalog, advert package assignments, and
// URGENT feature activation aggregates aligned with migration 00009.
package packaging

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// PackageCode is the catalog code CHECK set.
type PackageCode string

const (
	PackageCodeStarter  PackageCode = "STARTER"
	PackageCodeMiddle   PackageCode = "MIDDLE"
	PackageCodeAdvanced PackageCode = "ADVANCED"
)

// Valid reports whether c is a known package code.
func (c PackageCode) Valid() bool {
	switch c {
	case PackageCodeStarter, PackageCodeMiddle, PackageCodeAdvanced:
		return true
	}
	return false
}

// ParsePackageCode converts an external value into a PackageCode.
func ParsePackageCode(v string) (PackageCode, bool) {
	c := PackageCode(strings.TrimSpace(v))
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

// AssignmentSource is the assignment source CHECK set (no PAYMENT).
type AssignmentSource string

const (
	AssignmentSourceAdmin  AssignmentSource = "ADMIN"
	AssignmentSourceSystem AssignmentSource = "SYSTEM"
)

// Valid reports whether s is a known assignment source.
func (s AssignmentSource) Valid() bool {
	switch s {
	case AssignmentSourceAdmin, AssignmentSourceSystem:
		return true
	}
	return false
}

// FeatureCode is the feature activation CHECK set.
type FeatureCode string

const (
	FeatureCodeUrgent FeatureCode = "URGENT"
)

// Valid reports whether c is a known feature code.
func (c FeatureCode) Valid() bool {
	return c == FeatureCodeUrgent
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
	SearchPriority          int
	IsActive                bool
	SortOrder               int
	Version                 int
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// AllowsUrgentFeature reports whether this package may host URGENT.
func (p Package) AllowsUrgentFeature() bool {
	return p.AllowsUrgent && p.Code == PackageCodeAdvanced
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

// AdvertFeatureActivation is an URGENT (or future feature) activation row.
type AdvertFeatureActivation struct {
	ID                  uuid.UUID
	AdvertID            uuid.UUID
	PackageAssignmentID uuid.UUID
	FeatureCode         FeatureCode
	Status              FeatureActivationStatus
	ActivatedByUserID   uuid.UUID
	ActivatedAt         time.Time
	DeactivatedAt       *time.Time
	Reason              *string
	ActivationVersion   int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// IsActive reports status == ACTIVE.
func (a AdvertFeatureActivation) IsActive() bool {
	return a.Status == FeatureActivationStatusActive
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
