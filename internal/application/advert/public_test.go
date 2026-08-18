package advert

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
)

func TestPublicPageUsesPriorityPublishedAndIDCursor(t *testing.T) {
	now := time.Now().UTC()
	rows := []PublicCard{
		{ID: uuid.New(), SearchPriority: 20, PublishedAt: now},
		{ID: uuid.New(), SearchPriority: 10, PublishedAt: now.Add(-time.Minute)},
	}
	page := publicPage(rows, 1)
	if !page.HasMore || page.NextCursor == nil || len(page.Items) != 1 {
		t.Fatalf("unexpected page: %#v", page)
	}
	cursor, err := decodePublicCursor(*page.NextCursor)
	if err != nil || cursor.Priority != 20 || cursor.ID != rows[0].ID || !cursor.PublishedAt.Equal(now) {
		t.Fatalf("cursor did not preserve ordering key: %#v, %v", cursor, err)
	}
}

func TestHomepagePageIgnoresPackagePriority(t *testing.T) {
	now := time.Now().UTC()
	rows := []PublicCard{
		{ID: uuid.New(), SearchPriority: 0, PublishedAt: now},
		{ID: uuid.New(), SearchPriority: 999, PublishedAt: now.Add(-time.Minute)},
	}
	page := homepagePage(rows, 1)
	cursor, err := decodeHomepageCursor(*page.NextCursor)
	if err != nil || cursor.ID != rows[0].ID {
		t.Fatalf("homepage cursor must use published order only: %#v, %v", cursor, err)
	}
}

func TestShowcaseGeneratesStableSeedWhenProvided(t *testing.T) {
	repo := &fakePublicRepository{}
	svc := &Service{public: repo}
	out, err := svc.ListHomepageShowcase(context.Background(), stringPtr("fixed-seed"), intPtr(1), nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Seed != "fixed-seed" || repo.seed != "fixed-seed" {
		t.Fatalf("seed was not propagated: %#v", out)
	}
}

type fakePublicRepository struct{ seed string }

func (r *fakePublicRepository) SearchPublished(context.Context, domainadvert.PublicSearchQuery) ([]domainadvert.PublicCard, error) {
	return nil, nil
}
func (r *fakePublicRepository) ListHomepageNew(context.Context, domainadvert.HomepageNewQuery) ([]domainadvert.PublicCard, error) {
	return nil, nil
}
func (r *fakePublicRepository) ListHomepageShowcase(_ context.Context, seed string, _ int, _ *uuid.UUID) ([]domainadvert.PublicCard, error) {
	r.seed = seed
	return []domainadvert.PublicCard{}, nil
}
func (r *fakePublicRepository) ListHomepageUrgent(context.Context, int, *uuid.UUID) ([]domainadvert.PublicCard, error) {
	return []domainadvert.PublicCard{}, nil
}
func (r *fakePublicRepository) ListHomepageFeatured(context.Context, int, *uuid.UUID) ([]domainadvert.PublicCard, error) {
	return []domainadvert.PublicCard{}, nil
}
func (r *fakePublicRepository) GetPublishedDetail(context.Context, uuid.UUID, *uuid.UUID) (domainadvert.PublicDetail, error) {
	return domainadvert.PublicDetail{}, nil
}
func (r *fakePublicRepository) RecordView(context.Context, uuid.UUID, string) error {
	return nil
}

func intPtr(v int) *int          { return &v }
func stringPtr(v string) *string { return &v }
