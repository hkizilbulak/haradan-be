// Package banner holds banner aggregates aligned with migration 00007.
package banner

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Placement string

const (
	PlacementHomepage      Placement = "HOMEPAGE"
	PlacementListingDetail Placement = "LISTING_DETAIL"
	PlacementSearch        Placement = "SEARCH"
)

func (p Placement) Valid() bool {
	return p == PlacementHomepage || p == PlacementListingDetail || p == PlacementSearch
}

type Status string

const (
	StatusActive   Status = "ACTIVE"
	StatusInactive Status = "INACTIVE"
)

func (s Status) Valid() bool { return s == StatusActive || s == StatusInactive }

// Banner mirrors hrd_banners.
type Banner struct {
	ID              uuid.UUID
	Placement       Placement
	Status          Status
	AssetID         uuid.UUID
	Title           *string
	AltText         *string
	TargetURL       *string
	SortOrder       int
	Version         int
	CreatedByUserID *uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func TrimOptional(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}
