// Package comment holds the advert comment entity and domain rules.
package comment

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Domain errors
var (
	ErrCommentNotFound      = errors.New("comment not found")
	ErrEmptyContent         = errors.New("comment content cannot be empty")
	ErrContentTooLong       = errors.New("comment content exceeds maximum allowed length of 1000 characters")
	ErrAdvertNotCommentable = errors.New("comments are only allowed on published adverts")
)

const MaxContentLength = 1000

// Status represents the moderation/publication status of a comment.
type Status string

const (
	StatusPending   Status = "PENDING"
	StatusPublished Status = "PUBLISHED"
	StatusRejected  Status = "REJECTED"
)

// Valid reports whether s is a valid comment status.
func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusPublished, StatusRejected:
		return true
	}
	return false
}

// Comment is the aggregate entity for an advert comment.
type Comment struct {
	ID        uuid.UUID
	AdvertID  uuid.UUID
	UserID    uuid.UUID
	Content   string
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// Validate checks content rules for a comment.
func Validate(content string) (string, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", ErrEmptyContent
	}
	if len([]rune(trimmed)) > MaxContentLength {
		return "", ErrContentTooLong
	}
	return trimmed, nil
}
