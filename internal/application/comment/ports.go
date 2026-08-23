package comment

import (
	"context"
	"time"

	"github.com/google/uuid"

	domaincomment "github.com/hkizilbulak/haradan-be/internal/domain/comment"
)

// CommentRow represents a comment joined with author profile details for API display.
type CommentRow struct {
	Comment    domaincomment.Comment
	AuthorName string
}

// CreateCommentInput is the DTO for creating a comment.
type CreateCommentInput struct {
	UserID   uuid.UUID
	AdvertID uuid.UUID
	Content  string
	Rating   *int
}

// AdvertStatusResult represents advert information required to check if a comment can be posted.
type AdvertStatusResult struct {
	ID        uuid.UUID
	Status    string
	DeletedAt *time.Time
}

// Repository is the comment persistence port (SOLID Dependency Inversion).
type Repository interface {
	// FindAdvertStatus checks whether the advert exists and returns its status.
	FindAdvertStatus(ctx context.Context, advertID uuid.UUID) (AdvertStatusResult, error)

	// InsertComment persists a new comment record.
	InsertComment(ctx context.Context, c domaincomment.Comment) error

	// GetUserAuthorName returns a formatted author display name for a given user ID.
	GetUserAuthorName(ctx context.Context, userID uuid.UUID) (string, error)

	// FindCommentByID retrieves a comment by its ID.
	FindCommentByID(ctx context.Context, commentID uuid.UUID) (domaincomment.Comment, error)

	// DeleteComment soft-deletes a comment.
	DeleteComment(ctx context.Context, commentID uuid.UUID) error

	// ListCommentsByAdvert returns published comments for an advert ordered by created_at DESC.
	ListCommentsByAdvert(ctx context.Context, advertID uuid.UUID, limit, offset int) ([]CommentRow, int, error)
}

// Clock provides time for domain operations.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// IDGenerator creates UUIDs for new entities.
type IDGenerator interface {
	NewID() uuid.UUID
}

type uuidGenerator struct{}

func (uuidGenerator) NewID() uuid.UUID { return uuid.New() }
