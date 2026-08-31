// Package paytr holds PayTR iframe checkout charge aggregates.
package paytr

import (
	"time"

	"github.com/google/uuid"

	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
)

// ChargeStatus is the PayTR charge lifecycle.
type ChargeStatus string

const (
	ChargeStatusPending   ChargeStatus = "PENDING"
	ChargeStatusSucceeded ChargeStatus = "SUCCEEDED"
	ChargeStatusFailed    ChargeStatus = "FAILED"
	ChargeStatusCancelled ChargeStatus = "CANCELLED"
)

// Valid reports whether s is a known charge status.
func (s ChargeStatus) Valid() bool {
	switch s {
	case ChargeStatusPending, ChargeStatusSucceeded, ChargeStatusFailed, ChargeStatusCancelled:
		return true
	}
	return false
}

// Charge is a row in hrd_paytr_charges.
type Charge struct {
	ID                uuid.UUID
	MerchantOID       string
	AdvertID          int64
	OwnerUserID       uuid.UUID
	PackageCode       domainpackaging.PackageCode
	AmountMinor       int64
	CurrencyCode      string
	Status            ChargeStatus
	IframeToken       *string
	UserIP            *string
	TokenRequestJSON  *string
	TokenResponseJSON *string
	NotifyPayloadJSON *string
	FailReasonCode    *string
	FailReasonMsg     *string
	PaidAt            *time.Time
	AdvertSubmittedAt *time.Time
	Version           int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// IsTerminal reports whether the charge can no longer change.
func (c Charge) IsTerminal() bool {
	switch c.Status {
	case ChargeStatusSucceeded, ChargeStatusFailed, ChargeStatusCancelled:
		return true
	}
	return false
}
