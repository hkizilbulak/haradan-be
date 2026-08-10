package favorite_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	appfavorite "github.com/hkizilbulak/haradan-be/internal/application/favorite"
	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type fixture struct {
	svc      *appfavorite.Service
	store    *appfavorite.MemoryStore
	clock    *testClock
	user     uuid.UUID
	stranger uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	clock := &testClock{now: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)}
	store := appfavorite.NewMemoryStore()
	svc, err := appfavorite.NewMemoryService(store, clock)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return &fixture{svc: svc, store: store, clock: clock, user: uuid.New(), stranger: uuid.New()}
}

func requireCode(t *testing.T, err error, want apperr.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("want %s, got nil", want)
	}
	ae, ok := apperr.As(err)
	if !ok || ae.Code != want {
		t.Fatalf("want %s, got %v", want, err)
	}
}

func seedPublished(f *fixture, id uuid.UUID) {
	title := "Yayında ilan"
	now := f.clock.Now()
	cat := uuid.New()
	district := uuid.New()
	province := uuid.New()
	amount := int64(150000)
	currency := "TRY"
	f.store.PutAdvert(appfavorite.AdvertSnapshot{
		ID:               id,
		Status:           string(domainadvert.StatusPublished),
		Title:            &title,
		PublishedAt:      &now,
		CategoryID:       &cat,
		DistrictID:       &district,
		ProvinceID:       &province,
		PriceAmountMinor: &amount,
		PriceCurrency:    &currency,
	})
}

func TestAddFavoriteSuccessAndIdempotent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	advertID := uuid.New()
	seedPublished(f, advertID)

	first, err := f.svc.AddFavorite(ctx, f.user, advertID)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !first.Favorited || first.AdvertID != advertID {
		t.Fatalf("%+v", first)
	}
	second, err := f.svc.AddFavorite(ctx, f.user, advertID)
	if err != nil {
		t.Fatalf("duplicate add: %v", err)
	}
	if !second.Favorited {
		t.Fatal("duplicate must stay favorited")
	}
	if got := len(f.store.Favorites()); got != 1 {
		t.Fatalf("favorites=%d", got)
	}
}

func TestAddFavoriteConcurrentDuplicate(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	advertID := uuid.New()
	seedPublished(f, advertID)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.svc.AddFavorite(ctx, f.user, advertID)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent add: %v", err)
		}
	}
	if got := len(f.store.Favorites()); got != 1 {
		t.Fatalf("favorites=%d", got)
	}
}

func TestAddFavoriteNotFoundHidesExistence(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	missingID := uuid.New()
	_, missingErr := f.svc.AddFavorite(ctx, f.user, missingID)
	requireCode(t, missingErr, apperr.CodeNotFound)

	draftID := uuid.New()
	f.store.PutAdvert(appfavorite.AdvertSnapshot{ID: draftID, Status: string(domainadvert.StatusDraft)})
	_, draftErr := f.svc.AddFavorite(ctx, f.user, draftID)
	requireCode(t, draftErr, apperr.CodeNotFound)

	deletedID := uuid.New()
	now := f.clock.Now()
	title := "Silinmiş"
	cat := uuid.New()
	district := uuid.New()
	province := uuid.New()
	f.store.PutAdvert(appfavorite.AdvertSnapshot{
		ID: deletedID, Status: string(domainadvert.StatusPublished), DeletedAt: &now,
		Title: &title, PublishedAt: &now, CategoryID: &cat, DistrictID: &district, ProvinceID: &province,
	})
	_, deletedErr := f.svc.AddFavorite(ctx, f.user, deletedID)
	requireCode(t, deletedErr, apperr.CodeNotFound)

	// External clients must not tell these cases apart by status or message.
	missingAE, _ := apperr.As(missingErr)
	draftAE, _ := apperr.As(draftErr)
	deletedAE, _ := apperr.As(deletedErr)
	if missingAE.Code != draftAE.Code || missingAE.Code != deletedAE.Code {
		t.Fatalf("codes differ: missing=%s draft=%s deleted=%s", missingAE.Code, draftAE.Code, deletedAE.Code)
	}
	if missingAE.Message != draftAE.Message || missingAE.Message != deletedAE.Message {
		t.Fatalf("messages differ: %q / %q / %q", missingAE.Message, draftAE.Message, deletedAE.Message)
	}
	if len(f.store.Favorites()) != 0 {
		t.Fatal("no favorite rows must be created for hidden adverts")
	}
}

func TestRemoveFavoriteIdempotentAndCrossUser(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	advertID := uuid.New()
	seedPublished(f, advertID)
	if _, err := f.svc.AddFavorite(ctx, f.user, advertID); err != nil {
		t.Fatalf("add: %v", err)
	}

	out, err := f.svc.RemoveFavorite(ctx, f.stranger, advertID)
	if err != nil {
		t.Fatalf("stranger remove: %v", err)
	}
	if out.Favorited {
		t.Fatal("stranger remove must report favorited=false")
	}
	if got := len(f.store.Favorites()); got != 1 {
		t.Fatalf("owner favorite must remain, got %d", got)
	}

	out, err = f.svc.RemoveFavorite(ctx, f.user, advertID)
	if err != nil || out.Favorited {
		t.Fatalf("remove=%v favorited=%v", err, out.Favorited)
	}
	out, err = f.svc.RemoveFavorite(ctx, f.user, advertID)
	if err != nil || out.Favorited {
		t.Fatalf("idempotent remove=%v favorited=%v", err, out.Favorited)
	}
}

func TestListMyFavoritesPlaceholderAndEmpty(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	empty, err := f.svc.ListMyFavorites(ctx, f.user, appfavorite.ListInput{})
	if err != nil {
		t.Fatalf("empty list: %v", err)
	}
	if empty.Items == nil || len(empty.Items) != 0 || empty.HasMore {
		t.Fatalf("%+v", empty)
	}

	publishedID := uuid.New()
	seedPublished(f, publishedID)
	if _, err := f.svc.AddFavorite(ctx, f.user, publishedID); err != nil {
		t.Fatalf("add published: %v", err)
	}

	// Mutate advert to non-public after favorite exists.
	f.store.PutAdvert(appfavorite.AdvertSnapshot{ID: publishedID, Status: string(domainadvert.StatusSuspended)})

	archivedID := uuid.New()
	title := "Eski"
	now := f.clock.Now()
	cat := uuid.New()
	district := uuid.New()
	province := uuid.New()
	// Insert favorite by temporarily publishing, then archive.
	f.store.PutAdvert(appfavorite.AdvertSnapshot{
		ID: archivedID, Status: string(domainadvert.StatusPublished),
		Title: &title, PublishedAt: &now, CategoryID: &cat, DistrictID: &district, ProvinceID: &province,
	})
	f.clock.Advance(time.Second)
	if _, err := f.svc.AddFavorite(ctx, f.user, archivedID); err != nil {
		t.Fatalf("add archived candidate: %v", err)
	}
	f.store.PutAdvert(appfavorite.AdvertSnapshot{ID: archivedID, Status: string(domainadvert.StatusArchived)})

	limit := 10
	list, err := f.svc.ListMyFavorites(ctx, f.user, appfavorite.ListInput{Limit: &limit})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("items=%d", len(list.Items))
	}
	for _, item := range list.Items {
		if item.Available {
			t.Fatalf("expected unavailable %+v", item)
		}
		if item.Card != nil {
			t.Fatal("card must be nil for unavailable")
		}
		if item.UnavailableReason == nil || *item.UnavailableReason == "" {
			t.Fatal("placeholder reason required")
		}
	}

	// Restore one as published and verify card + newest-first order via pagination.
	seedPublished(f, publishedID)
	limit = 1
	page1, err := f.svc.ListMyFavorites(ctx, f.user, appfavorite.ListInput{Limit: &limit})
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if !page1.HasMore || page1.NextCursor == nil || len(page1.Items) != 1 {
		t.Fatalf("%+v", page1)
	}
	page2, err := f.svc.ListMyFavorites(ctx, f.user, appfavorite.ListInput{Limit: &limit, Cursor: page1.NextCursor})
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2.Items) != 1 {
		t.Fatalf("page2 items=%d", len(page2.Items))
	}
	if page1.Items[0].AdvertID == page2.Items[0].AdvertID {
		t.Fatal("pages must not duplicate")
	}

	for _, item := range append(page1.Items, page2.Items...) {
		if item.Available && item.Card != nil {
			if !item.Card.IsFavorite {
				t.Fatal("isFavorite must be true")
			}
		}
	}
}

func TestListIgnoresOtherUsers(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	advertID := uuid.New()
	seedPublished(f, advertID)
	if _, err := f.svc.AddFavorite(ctx, f.stranger, advertID); err != nil {
		t.Fatalf("stranger add: %v", err)
	}
	list, err := f.svc.ListMyFavorites(ctx, f.user, appfavorite.ListInput{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("leaked items=%+v", list.Items)
	}
}

func TestAddFavoriteNilAdvertIDValidation(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.AddFavorite(context.Background(), f.user, uuid.Nil)
	requireCode(t, err, apperr.CodeValidation)
}
