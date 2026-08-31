package comment

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincomment "github.com/hkizilbulak/haradan-be/internal/domain/comment"
)

// Service defines the use cases for advert comments.
type Service struct {
	repo  Repository
	clock Clock
	idGen IDGenerator
}

// Option modifies Service configuration.
type Option func(*Service)

// WithClock sets a custom clock for testing.
func WithClock(c Clock) Option {
	return func(s *Service) {
		s.clock = c
	}
}

// WithIDGenerator sets a custom ID generator for testing.
func WithIDGenerator(g IDGenerator) Option {
	return func(s *Service) {
		s.idGen = g
	}
}

// NewService constructs a new Comment application service.
func NewService(repo Repository, opts ...Option) *Service {
	s := &Service{
		repo:  repo,
		clock: systemClock{},
		idGen: uuidGenerator{},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// CreateComment validates and posts a new comment on a published advert.
func (s *Service) CreateComment(ctx context.Context, input CreateCommentInput) (CommentRow, error) {
	// 1. Validate content and rating
	sanitizedContent, err := domaincomment.Validate(input.Content, input.Rating)
	if err != nil {
		return CommentRow{}, err
	}

	// 2. Check advert existence & status
	adv, err := s.repo.FindAdvertStatus(ctx, input.AdvertID)
	if err != nil {
		return CommentRow{}, fmt.Errorf("failed to lookup advert status: %w", err)
	}
	if adv.DeletedAt != nil || adv.Status != "PUBLISHED" {
		return CommentRow{}, domaincomment.ErrAdvertNotCommentable
	}

	// 3. Get user display name
	authorName, err := s.repo.GetUserAuthorName(ctx, input.UserID)
	if err != nil {
		authorName = "Kullanıcı"
	}

	// 4. Build comment entity
	now := s.clock.Now()
	cmt := domaincomment.Comment{
		ID:        s.idGen.NewID(),
		AdvertID:  input.AdvertID,
		UserID:    input.UserID,
		Content:   sanitizedContent,
		Rating:    input.Rating,
		Status:    domaincomment.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 5. Persist comment
	if err := s.repo.InsertComment(ctx, cmt); err != nil {
		return CommentRow{}, fmt.Errorf("failed to insert comment: %w", err)
	}

	return CommentRow{
		Comment:    cmt,
		AuthorName: authorName,
	}, nil
}

// ListCommentsResult represents paginated comments list response.
type ListCommentsResult struct {
	Items      []CommentRow
	TotalCount int
}

// ListComments retrieves published comments for an advert with pagination.
func (s *Service) ListComments(ctx context.Context, advertID int64, limit, offset int) (ListCommentsResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	// Check advert existence
	adv, err := s.repo.FindAdvertStatus(ctx, advertID)
	if err != nil {
		return ListCommentsResult{}, fmt.Errorf("failed to lookup advert: %w", err)
	}
	if adv.DeletedAt != nil {
		return ListCommentsResult{}, domaincomment.ErrCommentNotFound
	}

	rows, total, err := s.repo.ListCommentsByAdvert(ctx, advertID, limit, offset)
	if err != nil {
		return ListCommentsResult{}, fmt.Errorf("failed to list comments: %w", err)
	}

	if rows == nil {
		rows = []CommentRow{}
	}

	return ListCommentsResult{
		Items:      rows,
		TotalCount: total,
	}, nil
}

// DeleteComment validates ownership and deletes the comment.
func (s *Service) DeleteComment(ctx context.Context, advertID int64, commentID, userID uuid.UUID) error {
	cmt, err := s.repo.FindCommentByID(ctx, commentID)
	if err != nil {
		if apErr, ok := apperr.As(err); ok && apErr.Kind == apperr.KindNotFound {
			return domaincomment.ErrCommentNotFound
		}
		return err
	}
	if cmt.DeletedAt != nil {
		return domaincomment.ErrCommentNotFound
	}
	if cmt.AdvertID != advertID {
		return domaincomment.ErrCommentNotFound
	}
	if cmt.UserID != userID {
		return domaincomment.ErrUnauthorizedCommentAction
	}
	return s.repo.DeleteComment(ctx, commentID)
}

// AdminListComments retrieves all comments based on status with pagination for admin.
func (s *Service) AdminListComments(ctx context.Context, status *domaincomment.Status, limit, offset int) (ListCommentsResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	rows, total, err := s.repo.AdminListComments(ctx, status, limit, offset)
	if err != nil {
		return ListCommentsResult{}, fmt.Errorf("failed to list admin comments: %w", err)
	}

	if rows == nil {
		rows = []CommentRow{}
	}

	return ListCommentsResult{
		Items:      rows,
		TotalCount: total,
	}, nil
}

// ApproveComment updates the comment status to PUBLISHED.
func (s *Service) ApproveComment(ctx context.Context, commentID uuid.UUID) error {
	cmt, err := s.repo.FindCommentByID(ctx, commentID)
	if err != nil {
		if apErr, ok := apperr.As(err); ok && apErr.Kind == apperr.KindNotFound {
			return domaincomment.ErrCommentNotFound
		}
		return err
	}
	if cmt.DeletedAt != nil {
		return domaincomment.ErrCommentNotFound
	}
	return s.repo.UpdateCommentStatus(ctx, commentID, domaincomment.StatusPublished)
}

// RejectComment updates the comment status to REJECTED.
func (s *Service) RejectComment(ctx context.Context, commentID uuid.UUID) error {
	cmt, err := s.repo.FindCommentByID(ctx, commentID)
	if err != nil {
		if apErr, ok := apperr.As(err); ok && apErr.Kind == apperr.KindNotFound {
			return domaincomment.ErrCommentNotFound
		}
		return err
	}
	if cmt.DeletedAt != nil {
		return domaincomment.ErrCommentNotFound
	}
	return s.repo.UpdateCommentStatus(ctx, commentID, domaincomment.StatusRejected)
}

// AdminDeleteComment soft-deletes a comment as an admin.
func (s *Service) AdminDeleteComment(ctx context.Context, commentID uuid.UUID) error {
	cmt, err := s.repo.FindCommentByID(ctx, commentID)
	if err != nil {
		if apErr, ok := apperr.As(err); ok && apErr.Kind == apperr.KindNotFound {
			return domaincomment.ErrCommentNotFound
		}
		return err
	}
	if cmt.DeletedAt != nil {
		return domaincomment.ErrCommentNotFound
	}
	return s.repo.DeleteComment(ctx, commentID)
}
