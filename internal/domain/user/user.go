package user

import (
	"time"

	"github.com/google/uuid"
)

// Role is the canonical user role.
type Role string

const (
	RoleUser       Role = "user"
	RoleAdmin      Role = "admin"
	RoleCallCenter Role = "CALL_CENTER"
)

// Status is the canonical account status.
type Status string

const (
	StatusActive   Status = "ACTIVE"
	StatusDisabled Status = "DISABLED"
	StatusClosed   Status = "CLOSED"
)

// Channel is the registration or auth method.
type Channel string

const (
	ChannelEmail  Channel = "EMAIL"
	ChannelGoogle Channel = "GOOGLE"
)

// User is the IAM user aggregate root.
type User struct {
	ID               uuid.UUID
	Email            string
	EmailNormalized  string
	PasswordHash     string
	Role             Role
	Status           Status
	Channel          Channel
	EmailVerifiedAt  *time.Time
	FirstName        string
	LastName         string
	Phone            *string
	SecurityStamp    uuid.UUID
	FailedLoginCount int
	LockedUntil      *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// IsActive reports whether the account may open or continue sessions.
func (u User) IsActive() bool {
	return u.Status == StatusActive
}

// IsLocked reports whether a temporary lock is in effect at now.
func (u User) IsLocked(now time.Time) bool {
	return u.LockedUntil != nil && u.LockedUntil.After(now)
}

// UserSetting represents user preferences and settings.
type UserSetting struct {
	UserID        uuid.UUID
	AllowEmail    bool
	AllowSMS      bool
	AllowWhatsapp bool
}

// UserConsentLog represents an audit log entry for user consents.
type UserConsentLog struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	AgreementType string
	Version       string
	IsGranted     bool
	IPAddress     *string
	UserAgent     *string
	Channel       string
	CreatedAt     time.Time
}
