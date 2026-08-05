// Package campaign holds campaign catalog aggregates aligned with migrations 00009/00011.
package campaign

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// CampaignEventType is the campaign event_type CHECK set.
type CampaignEventType string

const (
	CampaignEventTypePackageExpiry5Days CampaignEventType = "PACKAGE_EXPIRY_5_DAYS"
	CampaignEventTypePackageExpiry1Day  CampaignEventType = "PACKAGE_EXPIRY_1_DAY"
	CampaignEventTypePackageRenewal     CampaignEventType = "PACKAGE_RENEWAL"
	CampaignEventTypePackageUpgrade     CampaignEventType = "PACKAGE_UPGRADE"
)

// Valid reports whether t is a known campaign event type.
func (t CampaignEventType) Valid() bool {
	switch t {
	case CampaignEventTypePackageExpiry5Days,
		CampaignEventTypePackageExpiry1Day,
		CampaignEventTypePackageRenewal,
		CampaignEventTypePackageUpgrade:
		return true
	}
	return false
}

// ParseCampaignEventType converts an external value into a CampaignEventType.
func ParseCampaignEventType(v string) (CampaignEventType, bool) {
	t := CampaignEventType(strings.TrimSpace(v))
	return t, t.Valid()
}

// Campaign is the catalog row from hrd_campaigns.
type Campaign struct {
	ID                              uuid.UUID
	Code                            string
	Name                            string
	EventType                       CampaignEventType
	SourcePackageID                 *uuid.UUID
	TargetPackageID                 *uuid.UUID
	Title                           string
	Description                     *string
	EmailSubject                    *string
	EmailHeading                    *string
	EmailBody                       *string
	CTALabel                        *string
	CTAURL                          *string
	BadgeText                       *string
	ImageAssetID                    *uuid.UUID
	DisplayOriginalPriceAmountMinor *int64
	DisplayCampaignPriceAmountMinor *int64
	CurrencyCode                    string
	StartsAt                        time.Time
	EndsAt                          *time.Time
	IsActive                        bool
	CreatedByUserID                 uuid.UUID
	Version                         int
	CreatedAt                       time.Time
	UpdatedAt                       time.Time
}

// ValidTimeRange reports starts_at <= ends_at when ends_at is set.
func ValidTimeRange(startsAt time.Time, endsAt *time.Time) bool {
	if endsAt == nil {
		return true
	}
	return !endsAt.Before(startsAt)
}

// CampaignPriceLTEOriginal reports campaign price <= original when both are set.
// Nil on either side is allowed (nil-safe).
func CampaignPriceLTEOriginal(original, campaign *int64) bool {
	if original == nil || campaign == nil {
		return true
	}
	return *campaign <= *original
}

// NonBlankName reports whether name is non-blank after trim.
func NonBlankName(name string) bool {
	return strings.TrimSpace(name) != ""
}

// NonBlankTitle reports whether title is non-blank after trim.
func NonBlankTitle(title string) bool {
	return strings.TrimSpace(title) != ""
}
