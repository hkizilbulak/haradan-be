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

func TestGetPublishedAdvertDetailResolvesProperties(t *testing.T) {
	dispVal := "Evet"
	dispOption := "Safkan İngiliz"
	advID := uuid.New()
	detail := domainadvert.PublicDetail{
		PublicCard: domainadvert.PublicCard{
			ID:    advID,
			Title: "Test Advert",
		},
		Properties: []domainadvert.PublicProperty{
			{Code: "grassPaddock", Title: "Çim Padok", Value: true, DisplayValue: &dispVal},
			{Code: "studBreed", Title: "Aygır Irkı", Value: "THOROUGHBRED", DisplayValue: &dispOption},
		},
	}
	repo := &fakePublicRepository{detail: detail}
	svc := &Service{public: repo}

	out, err := svc.GetPublishedAdvertDetail(context.Background(), advID, nil, "127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ID != advID {
		t.Fatalf("expected advert id %v, got %v", advID, out.ID)
	}
	if len(out.Properties) != 2 {
		t.Fatalf("expected 2 properties, got %d", len(out.Properties))
	}
	if out.Properties[0].Title != "Çim Padok" || *out.Properties[0].DisplayValue != "Evet" {
		t.Fatalf("unexpected property 0: %#v", out.Properties[0])
	}
	if out.Properties[1].Title != "Aygır Irkı" || *out.Properties[1].DisplayValue != "Safkan İngiliz" {
		t.Fatalf("unexpected property 1: %#v", out.Properties[1])
	}
}

type fakePublicRepository struct {
	seed   string
	detail domainadvert.PublicDetail
}

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
	return r.detail, nil
}
func (r *fakePublicRepository) RecordView(context.Context, uuid.UUID, string) error {
	return nil
}

func intPtr(v int) *int          { return &v }
func stringPtr(v string) *string { return &v }
