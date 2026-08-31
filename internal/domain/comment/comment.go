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
	ErrCommentNotFound           = errors.New("comment not found")
	ErrEmptyContent              = errors.New("lütfen bir yorum yazınız veya puan veriniz")
	ErrContentTooLong            = errors.New("comment content exceeds maximum allowed length of 1000 characters")
	ErrInvalidRating             = errors.New("rating must be between 1 and 5")
	ErrAdvertNotCommentable      = errors.New("comments are only allowed on published adverts")
	ErrUnauthorizedCommentAction = errors.New("yalnızca kendi yorumunuzu silebilirsiniz")
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
	AdvertID  int64
	UserID    uuid.UUID
	Content   string
	Rating    *int
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// Validate checks content and rating rules for a comment.
func Validate(content string, rating *int) (string, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" && rating == nil {
		return "", ErrEmptyContent
	}
	if len([]rune(trimmed)) > MaxContentLength {
		return "", ErrContentTooLong
	}
	if rating != nil && (*rating < 1 || *rating > 5) {
		return "", ErrInvalidRating
	}
	return trimmed, nil
}
