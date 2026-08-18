package comment_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	appcomment "github.com/hkizilbulak/haradan-be/internal/application/comment"
	domaincomment "github.com/hkizilbulak/haradan-be/internal/domain/comment"
)

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time { return c.now }

type fixedIDGen struct {
	id uuid.UUID
}

func (g fixedIDGen) NewID() uuid.UUID { return g.id }

func TestCreateComment_Success(t *testing.T) {
	repo := appcomment.NewMemoryRepository()
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	commentID := uuid.New()
	userID := uuid.New()
	advertID := uuid.New()

	repo.AddAdvert(appcomment.AdvertStatusResult{
		ID:     advertID,
		Status: "PUBLISHED",
	})
	repo.AddUser(userID, "Ahmet K.")

	svc := appcomment.NewMemoryService(
		repo,
		appcomment.WithClock(fixedClock{now: now}),
		appcomment.WithIDGenerator(fixedIDGen{id: commentID}),
	)

	ctx := context.Background()
	res, err := svc.CreateComment(ctx, appcomment.CreateCommentInput{
		UserID:   userID,
		AdvertID: advertID,
		Content:  "  Harika bir at ilanı, detaylar çok temiz!  ",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Comment.ID != commentID {
		t.Errorf("expected comment ID %s, got %s", commentID, res.Comment.ID)
	}
	if res.Comment.Content != "Harika bir at ilanı, detaylar çok temiz!" {
		t.Errorf("expected trimmed content, got %s", res.Comment.Content)
	}
	if res.Comment.Status != domaincomment.StatusPublished {
		t.Errorf("expected PUBLISHED status, got %s", res.Comment.Status)
	}
	if res.AuthorName != "Ahmet K." {
		t.Errorf("expected author name 'Ahmet K.', got '%s'", res.AuthorName)
	}
}

func TestCreateComment_ValidationErrors(t *testing.T) {
	repo := appcomment.NewMemoryRepository()
	svc := appcomment.NewMemoryService(repo)
	ctx := context.Background()

	// Empty content
	_, err := svc.CreateComment(ctx, appcomment.CreateCommentInput{
		UserID:   uuid.New(),
		AdvertID: uuid.New(),
		Content:  "   ",
	})
	if !errors.Is(err, domaincomment.ErrEmptyContent) {
		t.Errorf("expected ErrEmptyContent, got %v", err)
	}

	// Content too long (>1000 chars)
	longText := strings.Repeat("a", 1001)
	_, err = svc.CreateComment(ctx, appcomment.CreateCommentInput{
		UserID:   uuid.New(),
		AdvertID: uuid.New(),
		Content:  longText,
	})
	if !errors.Is(err, domaincomment.ErrContentTooLong) {
		t.Errorf("expected ErrContentTooLong, got %v", err)
	}
}

func TestCreateComment_AdvertNotCommentable(t *testing.T) {
	repo := appcomment.NewMemoryRepository()
	svc := appcomment.NewMemoryService(repo)
	ctx := context.Background()

	draftAdvertID := uuid.New()
	repo.AddAdvert(appcomment.AdvertStatusResult{
		ID:     draftAdvertID,
		Status: "DRAFT",
	})

	// Draft advert cannot receive comments
	_, err := svc.CreateComment(ctx, appcomment.CreateCommentInput{
		UserID:   uuid.New(),
		AdvertID: draftAdvertID,
		Content:  "Güzel ilan",
	})
	if !errors.Is(err, domaincomment.ErrAdvertNotCommentable) {
		t.Errorf("expected ErrAdvertNotCommentable for DRAFT advert, got %v", err)
	}

	// Non-existent advert
	_, err = svc.CreateComment(ctx, appcomment.CreateCommentInput{
		UserID:   uuid.New(),
		AdvertID: uuid.New(),
		Content:  "Güzel ilan",
	})
	if err == nil {
		t.Error("expected error for non-existent advert, got nil")
	}
}

func TestListComments_Pagination(t *testing.T) {
	repo := appcomment.NewMemoryRepository()
	advertID := uuid.New()
	userID := uuid.New()

	repo.AddAdvert(appcomment.AdvertStatusResult{
		ID:     advertID,
		Status: "PUBLISHED",
	})
	repo.AddUser(userID, "Mehmet Y.")

	svc := appcomment.NewMemoryService(repo)
	ctx := context.Background()

	// Post 3 comments
	_, _ = svc.CreateComment(ctx, appcomment.CreateCommentInput{UserID: userID, AdvertID: advertID, Content: "İlk yorum"})
	_, _ = svc.CreateComment(ctx, appcomment.CreateCommentInput{UserID: userID, AdvertID: advertID, Content: "İkinci yorum"})
	_, _ = svc.CreateComment(ctx, appcomment.CreateCommentInput{UserID: userID, AdvertID: advertID, Content: "Üçüncü yorum"})

	res, err := svc.ListComments(ctx, advertID, 2, 0)
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if res.TotalCount != 3 {
		t.Errorf("expected total count 3, got %d", res.TotalCount)
	}
	if len(res.Items) != 2 {
		t.Errorf("expected 2 items for limit=2, got %d", len(res.Items))
	}

	// Offset test
	resPage2, err := svc.ListComments(ctx, advertID, 2, 2)
	if err != nil {
		t.Fatalf("unexpected list page 2 error: %v", err)
	}
	if len(resPage2.Items) != 1 {
		t.Errorf("expected 1 item for offset=2 limit=2, got %d", len(resPage2.Items))
	}
}
