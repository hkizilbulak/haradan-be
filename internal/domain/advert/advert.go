// Package advert holds the advert core aggregate and its lifecycle rules.
package advert

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Status is the single advert lifecycle/moderation state machine value.
type Status string

const (
	StatusDraft            Status = "DRAFT"
	StatusPendingReview    Status = "PENDING_REVIEW"
	StatusChangesRequested Status = "CHANGES_REQUESTED"
	StatusPublished        Status = "PUBLISHED"
	StatusRejected         Status = "REJECTED"
	StatusSuspended        Status = "SUSPENDED"
	StatusSold             Status = "SOLD"
	StatusArchived         Status = "ARCHIVED"
)

// Valid reports whether s is a known phase-one status.
func (s Status) Valid() bool {
	switch s {
	case StatusDraft, StatusPendingReview, StatusChangesRequested, StatusPublished,
		StatusRejected, StatusSuspended, StatusSold, StatusArchived:
		return true
	}
	return false
}

// ParseStatus converts an external value into a Status.
func ParseStatus(v string) (Status, bool) {
	s := Status(v)
	return s, s.Valid()
}

// Money is a price pair stored as minor units; never a float.
type Money struct {
	AmountMinor int64
	Currency    string
}

// Advert is the advert core aggregate mirroring hrd_adverts.
type Advert struct {
	ID           uuid.UUID
	OwnerUserID  uuid.UUID
	CategoryID   *uuid.UUID
	DistrictID   *uuid.UUID
	HorseID      *uuid.UUID
	Title        *string
	Description  *string
	Price        *Money
	Status       Status
	Properties   json.RawMessage
	PublishedAt  *time.Time
	SoldAt       *time.Time
	Version      int
	MediaVersion int
	DeletedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// StatusHistory is an immutable advert status transition record.
type StatusHistory struct {
	ID          uuid.UUID
	AdvertID    uuid.UUID
	FromStatus  *Status
	ToStatus    Status
	ActorUserID *uuid.UUID
	IsSystem    bool
	Reason      *string
	CreatedAt   time.Time
}

// MediaRelation is the owner-visible advert/media link projection. The media
// domain owns the underlying rows; the advert view carries them unresolved.
type MediaRelation struct {
	AssetID         uuid.UUID
	DisplayOrder    int
	IsCover         bool
	LifecycleStatus string
}

// OwnerView is the owner-scoped advert projection.
type OwnerView struct {
	ID                     uuid.UUID
	Status                 Status
	Version                int
	MediaVersion           int
	CategoryID             *uuid.UUID
	DistrictID             *uuid.UUID
	ProvinceID             *uuid.UUID
	HorseID                *uuid.UUID
	Title                  *string
	Description            *string
	Price                  *Money
	Properties             json.RawMessage
	Media                  []MediaRelation
	PublishedAt            *time.Time
	SoldAt                 *time.Time
	DeletedAt              *time.Time
	UpdatedAt              time.Time
	CategoryClearedWarning *bool
}

// DetailsPatch carries owner-editable core fields. The *Set flags separate an
// absent field from an explicit null, which clears the column.
type DetailsPatch struct {
	DistrictIDSet bool
	DistrictID    *uuid.UUID

	HorseIDSet bool
	HorseID    *uuid.UUID

	TitleSet bool
	Title    *string

	DescriptionSet bool
	Description    *string

	PriceSet bool
	Price    *Money
}

// IsEmpty reports whether the patch would change nothing.
func (p DetailsPatch) IsEmpty() bool {
	return !p.DistrictIDSet && !p.HorseIDSet && !p.TitleSet && !p.DescriptionSet && !p.PriceSet
}

// EmptyProperties returns the canonical empty dynamic property object.
func EmptyProperties() json.RawMessage {
	return json.RawMessage(`{}`)
}

// ToOwnerView projects the aggregate for its owner. Media and province are
// filled by the application layer after the core row is loaded.
func (a Advert) ToOwnerView() OwnerView {
	props := a.Properties
	if len(props) == 0 {
		props = EmptyProperties()
	}
	return OwnerView{
		ID:           a.ID,
		Status:       a.Status,
		Version:      a.Version,
		MediaVersion: a.MediaVersion,
		CategoryID:   a.CategoryID,
		DistrictID:   a.DistrictID,
		HorseID:      a.HorseID,
		Title:        a.Title,
		Description:  a.Description,
		Price:        a.Price,
		Properties:   props,
		Media:        []MediaRelation{},
		PublishedAt:  a.PublishedAt,
		SoldAt:       a.SoldAt,
		DeletedAt:    a.DeletedAt,
		UpdatedAt:    a.UpdatedAt,
	}
}

// IsDeleted reports whether the advert is soft-deleted.
func (a Advert) IsDeleted() bool { return a.DeletedAt != nil }

// CanOwnerEditDetails reports whether the owner may edit core content fields.
func CanOwnerEditDetails(s Status) bool {
	return s == StatusDraft || s == StatusChangesRequested
}

// CanOwnerChangeCategory reports whether the owner may change the category.
func CanOwnerChangeCategory(s Status) bool { return s == StatusDraft }

// OwnerTransitionAllowed reports whether the owner may drive from -> to.
func OwnerTransitionAllowed(from, to Status) bool {
	switch from {
	case StatusDraft, StatusChangesRequested:
		return to == StatusPendingReview
	case StatusPublished:
		return to == StatusSold || to == StatusArchived
	}
	return false
}

// AdminTransitionAllowed reports whether an admin moderation action may drive
// from -> to. Resume/unsuspend is intentionally absent (phase-one OpenAPI).
func AdminTransitionAllowed(from, to Status) bool {
	switch from {
	case StatusPendingReview:
		return to == StatusPublished || to == StatusChangesRequested || to == StatusRejected
	case StatusPublished:
		return to == StatusSuspended
	}
	return false
}

// ModerationDetailView is the admin moderation projection: owner-visible
// fields plus owner id and immutable status history.
type ModerationDetailView struct {
	OwnerView
	OwnerUserID   uuid.UUID
	StatusHistory []StatusHistory
}
