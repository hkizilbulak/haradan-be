package user

import (
	"time"

	"github.com/google/uuid"
)

// Role is the canonical user role.
type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// Status is the canonical account status.
type Status string

const (
	StatusActive   Status = "ACTIVE"
	StatusDisabled Status = "DISABLED"
	StatusClosed   Status = "CLOSED"
)

// User is the IAM user aggregate root.
type User struct {
	ID               uuid.UUID
	Email            string
	EmailNormalized  string
	PasswordHash     string
	Role             Role
	Status           Status
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
	return u.LockedUntil != nil && now.Before(*u.LockedUntil)
}
