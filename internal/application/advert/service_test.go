package advert_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	appadvert "github.com/hkizilbulak/haradan-be/internal/application/advert"
	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
	domaingeo "github.com/hkizilbulak/haradan-be/internal/domain/geo"
	domainhorse "github.com/hkizilbulak/haradan-be/internal/domain/horse"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
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
	svc       *appadvert.Service
	store     *appadvert.MemoryStore
	clock     *testClock
	owner     uuid.UUID
	stranger  uuid.UUID
	category  uuid.UUID
	category2 uuid.UUID
	district  uuid.UUID
	horse     uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	clock := &testClock{now: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)}
	store := appadvert.NewMemoryStore()

	verified := clock.Now().Add(-24 * time.Hour)
	owner := uuid.New()
	stranger := uuid.New()
	store.PutUser(domainuser.User{ID: owner, Status: domainuser.StatusActive, EmailVerifiedAt: &verified})
	store.PutUser(domainuser.User{ID: stranger, Status: domainuser.StatusActive, EmailVerifiedAt: &verified})

	category := uuid.New()
	category2 := uuid.New()
	store.PutCategory(
		domaincatalog.Category{ID: category, Slug: "yarim-kan", Name: "Yarım Kan", IsActive: true},
		0,
		[]domaincatalog.Property{
			{ID: uuid.New(), CategoryID: category, Code: "age", DataType: "INTEGER", IsRequired: true},
			{
				ID:         uuid.New(),
				CategoryID: category,
				Code:       "color",
				DataType:   "SINGLE_SELECT",
				Options:    json.RawMessage(`[{"value":"BAY","label":"Doru"},{"value":"GREY","label":"Kır"}]`),
			},
			{ID: uuid.New(), CategoryID: category, Code: "notes", DataType: "TEXT"},
		},
	)
	store.PutCategory(
		domaincatalog.Category{ID: category2, Slug: "safkan", Name: "Safkan", IsActive: true},
		0,
		[]domaincatalog.Property{
			{ID: uuid.New(), CategoryID: category2, Code: "height", DataType: "DECIMAL"},
		},
	)

	district := uuid.New()
	store.PutDistrict(domaingeo.District{ID: district, ProvinceID: uuid.New(), Name: "Çankaya", IsActive: true})

	horse := uuid.New()
	store.PutHorse(domainhorse.Horse{ID: horse, TJKNumber: "12345", OriginalName: "Rüzgar"})

	svc, err := appadvert.NewMemoryService(store, clock)
	if err != nil {
		t.Fatalf("new memory service: %v", err)
	}
	return &fixture{
		svc:       svc,
		store:     store,
		clock:     clock,
		owner:     owner,
		stranger:  stranger,
		category:  category,
		category2: category2,
		district:  district,
		horse:     horse,
	}
}

// seed inserts an advert directly so lifecycle states can be tested without
// walking the whole flow.
func (f *fixture) seed(t *testing.T, ownerID uuid.UUID, status domainadvert.Status, mutate func(*domainadvert.Advert)) domainadvert.Advert {
	t.Helper()
	title := "Satılık at"
	description := "Sağlıklı, eğitimli."
	category := f.category
	district := f.district
	now := f.clock.Now()
	a := domainadvert.Advert{
		ID:           uuid.New(),
		OwnerUserID:  ownerID,
		CategoryID:   &category,
		DistrictID:   &district,
		Title:        &title,
		Description:  &description,
		Status:       status,
		Properties:   json.RawMessage(`{"age":5}`),
		Version:      1,
		MediaVersion: 1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if status == domainadvert.StatusPublished {
		published := now.Add(-time.Hour)
		a.PublishedAt = &published
	}
	if mutate != nil {
		mutate(&a)
	}
	f.store.PutAdvert(a)

	// Submission/approval requires a READY cover attachment. Seed a MASTER_READY
	// cover so submit-path and moderation approval tests can focus on core/
	// property logic.
	if status == domainadvert.StatusDraft ||
		status == domainadvert.StatusChangesRequested ||
		status == domainadvert.StatusPendingReview {
		assetID := uuid.New()
		f.store.PutMediaRelations(a.ID, []domainadvert.MediaRelation{{
			AssetID:         assetID,
			DisplayOrder:    0,
			IsCover:         true,
			LifecycleStatus: string(domainmedia.AssetMasterReady),
		}})
	}
	return a
}

func requireCode(t *testing.T, err error, want apperr.Code) *apperr.Error {
	t.Helper()
	ae, ok := apperr.As(err)
	if !ok {
		t.Fatalf("want %s, got non-domain error: %v", want, err)
	}
	if ae.Code != want {
		t.Fatalf("want %s, got %s (%v)", want, ae.Code, err)
	}
	return ae
}

func ptr[T any](v T) *T { return &v }

func TestCreateAdvertDraftSuccess(t *testing.T) {
	f := newFixture(t)
	view, err := f.svc.CreateAdvertDraft(context.Background(), f.owner, appadvert.CreateDraftInput{
		CategoryID:  &f.category,
		DistrictID:  &f.district,
		HorseID:     &f.horse,
		Title:       ptr("  Satılık safkan  "),
		Description: ptr("Açıklama"),
		Price:       &appadvert.MoneyInput{AmountMinor: ptr(int64(150000)), Currency: ptr("TRY")},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if view.Status != domainadvert.StatusDraft {
		t.Fatalf("status=%s", view.Status)
	}
	if view.Version != 1 || view.MediaVersion != 1 {
		t.Fatalf("version=%d mediaVersion=%d", view.Version, view.MediaVersion)
	}
	if view.Title == nil || *view.Title != "Satılık safkan" {
		t.Fatalf("title=%v", view.Title)
	}
	if view.Price == nil || view.Price.AmountMinor != 150000 || view.Price.Currency != "TRY" {
		t.Fatalf("price=%+v", view.Price)
	}
	if string(view.Properties) != "{}" {
		t.Fatalf("properties=%s", view.Properties)
	}
	if view.Media == nil || len(view.Media) != 0 {
		t.Fatalf("media=%+v", view.Media)
	}

	history := f.store.History()
	if len(history) != 1 {
		t.Fatalf("history=%+v", history)
	}
	if history[0].FromStatus != nil || history[0].ToStatus != domainadvert.StatusDraft {
		t.Fatalf("history[0]=%+v", history[0])
	}
	if history[0].ActorUserID == nil || *history[0].ActorUserID != f.owner || history[0].IsSystem {
		t.Fatalf("history actor=%+v", history[0])
	}
}

func TestCreateAdvertDraftEmptyIsAllowed(t *testing.T) {
	f := newFixture(t)
	view, err := f.svc.CreateAdvertDraft(context.Background(), f.owner, appadvert.CreateDraftInput{})
	if err != nil {
		t.Fatalf("create empty draft: %v", err)
	}
	if view.CategoryID != nil || view.Title != nil || view.Price != nil {
		t.Fatalf("expected empty draft, got %+v", view)
	}
}

func TestCreateAdvertDraftValidation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	t.Run("title too long", func(t *testing.T) {
		_, err := f.svc.CreateAdvertDraft(ctx, f.owner, appadvert.CreateDraftInput{
			Title: ptr(strings.Repeat("a", 201)),
		})
		ae := requireCode(t, err, apperr.CodeValidation)
		if len(ae.FieldErrors) == 0 || ae.FieldErrors[0].Field != "title" {
			t.Fatalf("fields=%+v", ae.FieldErrors)
		}
	})

	t.Run("money half pair", func(t *testing.T) {
		_, err := f.svc.CreateAdvertDraft(ctx, f.owner, appadvert.CreateDraftInput{
			Price: &appadvert.MoneyInput{AmountMinor: ptr(int64(1000))},
		})
		ae := requireCode(t, err, apperr.CodeValidation)
		if len(ae.FieldErrors) == 0 || ae.FieldErrors[0].Field != "price" {
			t.Fatalf("fields=%+v", ae.FieldErrors)
		}

		_, err = f.svc.CreateAdvertDraft(ctx, f.owner, appadvert.CreateDraftInput{
			Price: &appadvert.MoneyInput{Currency: ptr("TRY")},
		})
		requireCode(t, err, apperr.CodeValidation)
	})

	t.Run("bad currency and negative amount", func(t *testing.T) {
		_, err := f.svc.CreateAdvertDraft(ctx, f.owner, appadvert.CreateDraftInput{
			Price: &appadvert.MoneyInput{AmountMinor: ptr(int64(100)), Currency: ptr("try")},
		})
		requireCode(t, err, apperr.CodeValidation)

		_, err = f.svc.CreateAdvertDraft(ctx, f.owner, appadvert.CreateDraftInput{
			Price: &appadvert.MoneyInput{AmountMinor: ptr(int64(-1)), Currency: ptr("TRY")},
		})
		requireCode(t, err, apperr.CodeValidation)
	})

	t.Run("unknown references", func(t *testing.T) {
		unknown := uuid.New()
		_, err := f.svc.CreateAdvertDraft(ctx, f.owner, appadvert.CreateDraftInput{CategoryID: &unknown})
		requireCode(t, err, apperr.CodeValidation)

		_, err = f.svc.CreateAdvertDraft(ctx, f.owner, appadvert.CreateDraftInput{DistrictID: &unknown})
		requireCode(t, err, apperr.CodeValidation)

		_, err = f.svc.CreateAdvertDraft(ctx, f.owner, appadvert.CreateDraftInput{HorseID: &unknown})
		requireCode(t, err, apperr.CodeValidation)
	})

	t.Run("non leaf category", func(t *testing.T) {
		parent := uuid.New()
		f.store.PutCategory(domaincatalog.Category{ID: parent, Slug: "atlar", Name: "Atlar", IsActive: true}, 2, nil)
		_, err := f.svc.CreateAdvertDraft(ctx, f.owner, appadvert.CreateDraftInput{CategoryID: &parent})
		requireCode(t, err, apperr.CodeValidation)
	})
}

func TestListMyAdvertsEmpty(t *testing.T) {
	f := newFixture(t)
	got, err := f.svc.ListMyAdverts(context.Background(), f.owner, appadvert.ListInput{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got.Items == nil || len(got.Items) != 0 {
		t.Fatalf("items=%+v", got.Items)
	}
	if got.HasMore || got.NextCursor != nil {
		t.Fatalf("hasMore=%v cursor=%v", got.HasMore, got.NextCursor)
	}
}

func TestListMyAdvertsExcludesDeletedAndForeignAndPages(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	oldest := f.seed(t, f.owner, domainadvert.StatusDraft, nil)
	f.clock.Advance(time.Minute)
	middle := f.seed(t, f.owner, domainadvert.StatusPublished, nil)
	f.clock.Advance(time.Minute)
	newest := f.seed(t, f.owner, domainadvert.StatusDraft, nil)

	f.seed(t, f.stranger, domainadvert.StatusDraft, nil)
	f.seed(t, f.owner, domainadvert.StatusDraft, func(a *domainadvert.Advert) {
		deleted := f.clock.Now()
		a.DeletedAt = &deleted
	})

	first, err := f.svc.ListMyAdverts(ctx, f.owner, appadvert.ListInput{Limit: ptr(2)})
	if err != nil {
		t.Fatalf("list page 1: %v", err)
	}
	if len(first.Items) != 2 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("page1=%+v hasMore=%v cursor=%v", first.Items, first.HasMore, first.NextCursor)
	}
	if first.Items[0].ID != newest.ID || first.Items[1].ID != middle.ID {
		t.Fatalf("unexpected order: %s %s", first.Items[0].ID, first.Items[1].ID)
	}

	second, err := f.svc.ListMyAdverts(ctx, f.owner, appadvert.ListInput{Limit: ptr(2), Cursor: first.NextCursor})
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}
	if len(second.Items) != 1 || second.Items[0].ID != oldest.ID || second.HasMore {
		t.Fatalf("page2=%+v hasMore=%v", second.Items, second.HasMore)
	}

	filtered, err := f.svc.ListMyAdverts(ctx, f.owner, appadvert.ListInput{Status: ptr("PUBLISHED")})
	if err != nil {
		t.Fatalf("list filtered: %v", err)
	}
	if len(filtered.Items) != 1 || filtered.Items[0].ID != middle.ID {
		t.Fatalf("filtered=%+v", filtered.Items)
	}
}

func TestListMyAdvertsBadInputs(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	_, err := f.svc.ListMyAdverts(ctx, f.owner, appadvert.ListInput{Status: ptr("NOPE")})
	requireCode(t, err, apperr.CodeValidation)

	_, err = f.svc.ListMyAdverts(ctx, f.owner, appadvert.ListInput{Cursor: ptr("not-a-cursor")})
	requireCode(t, err, apperr.CodeValidation)

	_, err = f.svc.ListMyAdverts(ctx, f.owner, appadvert.ListInput{Limit: ptr(0)})
	requireCode(t, err, apperr.CodeValidation)

	_, err = f.svc.ListMyAdverts(ctx, f.owner, appadvert.ListInput{Limit: ptr(101)})
	requireCode(t, err, apperr.CodeValidation)
}

func TestGetMyAdvertCrossUserIsNotFound(t *testing.T) {
	f := newFixture(t)
	foreign := f.seed(t, f.stranger, domainadvert.StatusPublished, nil)

	_, err := f.svc.GetMyAdvert(context.Background(), f.owner, foreign.ID)
	requireCode(t, err, apperr.CodeNotFound)

	_, err = f.svc.GetMyAdvert(context.Background(), f.owner, uuid.New())
	requireCode(t, err, apperr.CodeNotFound)

	own, err := f.svc.GetMyAdvert(context.Background(), f.stranger, foreign.ID)
	if err != nil || own.ID != foreign.ID {
		t.Fatalf("own=%+v err=%v", own, err)
	}
}

func TestUpdateAdvertDraftDetailsSuccess(t *testing.T) {
	f := newFixture(t)
	draft := f.seed(t, f.owner, domainadvert.StatusDraft, nil)
	f.clock.Advance(time.Minute)

	view, err := f.svc.UpdateAdvertDraftDetails(context.Background(), f.owner, draft.ID, appadvert.UpdateDetailsInput{
		ExpectedVersion: 1,
		TitleSet:        true,
		Title:           ptr("Yeni başlık"),
		HorseIDSet:      true,
		HorseID:         &f.horse,
		PriceSet:        true,
		Price:           &appadvert.MoneyInput{AmountMinor: ptr(int64(2500)), Currency: ptr("EUR")},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if view.Version != 2 {
		t.Fatalf("version=%d", view.Version)
	}
	if view.Title == nil || *view.Title != "Yeni başlık" {
		t.Fatalf("title=%v", view.Title)
	}
	if view.HorseID == nil || *view.HorseID != f.horse {
		t.Fatalf("horseId=%v", view.HorseID)
	}
	if view.Price == nil || view.Price.Currency != "EUR" {
		t.Fatalf("price=%+v", view.Price)
	}
	// Details updates never write status history.
	if len(f.store.History()) != 0 {
		t.Fatalf("history=%+v", f.store.History())
	}
}

func TestUpdateAdvertDraftDetailsClearsWithExplicitNull(t *testing.T) {
	f := newFixture(t)
	draft := f.seed(t, f.owner, domainadvert.StatusDraft, func(a *domainadvert.Advert) {
		a.Price = &domainadvert.Money{AmountMinor: 100, Currency: "TRY"}
		a.HorseID = &f.horse
	})

	view, err := f.svc.UpdateAdvertDraftDetails(context.Background(), f.owner, draft.ID, appadvert.UpdateDetailsInput{
		ExpectedVersion: 1,
		PriceSet:        true,
		HorseIDSet:      true,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if view.Price != nil || view.HorseID != nil {
		t.Fatalf("expected cleared price/horse, got %+v", view)
	}
}

func TestUpdateAdvertDraftDetailsStaleVersion(t *testing.T) {
	f := newFixture(t)
	draft := f.seed(t, f.owner, domainadvert.StatusDraft, nil)

	_, err := f.svc.UpdateAdvertDraftDetails(context.Background(), f.owner, draft.ID, appadvert.UpdateDetailsInput{
		ExpectedVersion: 7,
		TitleSet:        true,
		Title:           ptr("Yeni"),
	})
	requireCode(t, err, apperr.CodeStaleVersion)
}

func TestUpdateAdvertDraftDetailsInvalidState(t *testing.T) {
	f := newFixture(t)
	published := f.seed(t, f.owner, domainadvert.StatusPublished, nil)

	_, err := f.svc.UpdateAdvertDraftDetails(context.Background(), f.owner, published.ID, appadvert.UpdateDetailsInput{
		ExpectedVersion: 1,
		TitleSet:        true,
		Title:           ptr("Yeni"),
	})
	requireCode(t, err, apperr.CodeInvalidState)

	changes := f.seed(t, f.owner, domainadvert.StatusChangesRequested, nil)
	if _, err := f.svc.UpdateAdvertDraftDetails(context.Background(), f.owner, changes.ID, appadvert.UpdateDetailsInput{
		ExpectedVersion: 1,
		TitleSet:        true,
		Title:           ptr("Düzeltildi"),
	}); err != nil {
		t.Fatalf("CHANGES_REQUESTED must stay editable: %v", err)
	}
}

func TestUpdateAdvertDraftDetailsCrossUserIsNotFound(t *testing.T) {
	f := newFixture(t)
	foreign := f.seed(t, f.stranger, domainadvert.StatusDraft, nil)

	_, err := f.svc.UpdateAdvertDraftDetails(context.Background(), f.owner, foreign.ID, appadvert.UpdateDetailsInput{
		ExpectedVersion: 1,
		TitleSet:        true,
		Title:           ptr("Ele geçirme"),
	})
	requireCode(t, err, apperr.CodeNotFound)
}

func TestChangeAdvertDraftCategoryClearsProperties(t *testing.T) {
	f := newFixture(t)
	draft := f.seed(t, f.owner, domainadvert.StatusDraft, nil)

	view, err := f.svc.ChangeAdvertDraftCategory(context.Background(), f.owner, draft.ID, appadvert.ChangeCategoryInput{
		ExpectedVersion: 1,
		CategoryID:      f.category2,
	})
	if err != nil {
		t.Fatalf("change category: %v", err)
	}
	if view.CategoryID == nil || *view.CategoryID != f.category2 {
		t.Fatalf("categoryId=%v", view.CategoryID)
	}
	if string(view.Properties) != "{}" {
		t.Fatalf("properties=%s", view.Properties)
	}
	if view.Version != 2 {
		t.Fatalf("version=%d", view.Version)
	}
	if view.CategoryClearedWarning == nil || !*view.CategoryClearedWarning {
		t.Fatalf("warning=%v", view.CategoryClearedWarning)
	}
}

func TestChangeAdvertDraftCategorySameCategoryIsNoOp(t *testing.T) {
	f := newFixture(t)
	draft := f.seed(t, f.owner, domainadvert.StatusDraft, nil)

	view, err := f.svc.ChangeAdvertDraftCategory(context.Background(), f.owner, draft.ID, appadvert.ChangeCategoryInput{
		ExpectedVersion: 1,
		CategoryID:      f.category,
	})
	if err != nil {
		t.Fatalf("change category: %v", err)
	}
	if view.Version != 1 {
		t.Fatalf("version=%d, same category must not bump", view.Version)
	}
	if string(view.Properties) != `{"age":5}` {
		t.Fatalf("properties=%s, same category must not clear", view.Properties)
	}
	if view.CategoryClearedWarning == nil || *view.CategoryClearedWarning {
		t.Fatalf("warning=%v", view.CategoryClearedWarning)
	}
}

func TestChangeAdvertDraftCategoryRejectedOutsideDraft(t *testing.T) {
	f := newFixture(t)
	changes := f.seed(t, f.owner, domainadvert.StatusChangesRequested, nil)

	_, err := f.svc.ChangeAdvertDraftCategory(context.Background(), f.owner, changes.ID, appadvert.ChangeCategoryInput{
		ExpectedVersion: 1,
		CategoryID:      f.category2,
	})
	requireCode(t, err, apperr.CodeInvalidState)
}

func TestReplaceAdvertDynamicProperties(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	t.Run("unknown key rejected", func(t *testing.T) {
		draft := f.seed(t, f.owner, domainadvert.StatusDraft, nil)
		_, err := f.svc.ReplaceAdvertDynamicProperties(ctx, f.owner, draft.ID, appadvert.ReplacePropertiesInput{
			ExpectedVersion: 1,
			Properties:      json.RawMessage(`{"age":4,"sneaky":"value"}`),
		})
		ae := requireCode(t, err, apperr.CodeValidation)
		if len(ae.FieldErrors) != 1 || ae.FieldErrors[0].Field != "properties.sneaky" {
			t.Fatalf("fields=%+v", ae.FieldErrors)
		}
	})

	t.Run("draft mode ignores required", func(t *testing.T) {
		draft := f.seed(t, f.owner, domainadvert.StatusDraft, nil)
		view, err := f.svc.ReplaceAdvertDynamicProperties(ctx, f.owner, draft.ID, appadvert.ReplacePropertiesInput{
			ExpectedVersion: 1,
			Properties:      json.RawMessage(`{"color":"BAY","notes":"  temiz  "}`),
		})
		if err != nil {
			t.Fatalf("replace: %v", err)
		}
		if view.Version != 2 {
			t.Fatalf("version=%d", view.Version)
		}
		var got map[string]any
		if err := json.Unmarshal(view.Properties, &got); err != nil {
			t.Fatalf("decode properties: %v", err)
		}
		if got["color"] != "BAY" || got["notes"] != "temiz" {
			t.Fatalf("properties=%s", view.Properties)
		}
		if _, ok := got["age"]; ok {
			t.Fatalf("replace must drop absent keys: %s", view.Properties)
		}
	})

	t.Run("blank and null values are dropped", func(t *testing.T) {
		draft := f.seed(t, f.owner, domainadvert.StatusDraft, nil)
		view, err := f.svc.ReplaceAdvertDynamicProperties(ctx, f.owner, draft.ID, appadvert.ReplacePropertiesInput{
			ExpectedVersion: 1,
			Properties:      json.RawMessage(`{"age":6,"notes":"   ","color":null}`),
		})
		if err != nil {
			t.Fatalf("replace: %v", err)
		}
		if string(view.Properties) != `{"age":6}` {
			t.Fatalf("properties=%s", view.Properties)
		}
	})

	t.Run("blank required value fails submit", func(t *testing.T) {
		draft := f.seed(t, f.owner, domainadvert.StatusDraft, func(a *domainadvert.Advert) {
			a.Properties = json.RawMessage(`{"age":null,"notes":"  "}`)
		})
		_, err := f.svc.SubmitAdvertForReview(ctx, f.owner, draft.ID, 1)
		ae := requireCode(t, err, apperr.CodeValidation)
		if len(ae.FieldErrors) != 1 || ae.FieldErrors[0].Field != "properties.age" {
			t.Fatalf("fields=%+v", ae.FieldErrors)
		}
	})

	t.Run("type and option checks", func(t *testing.T) {
		draft := f.seed(t, f.owner, domainadvert.StatusDraft, nil)
		_, err := f.svc.ReplaceAdvertDynamicProperties(ctx, f.owner, draft.ID, appadvert.ReplacePropertiesInput{
			ExpectedVersion: 1,
			Properties:      json.RawMessage(`{"age":"beş"}`),
		})
		requireCode(t, err, apperr.CodeValidation)

		_, err = f.svc.ReplaceAdvertDynamicProperties(ctx, f.owner, draft.ID, appadvert.ReplacePropertiesInput{
			ExpectedVersion: 1,
			Properties:      json.RawMessage(`{"age":4.5}`),
		})
		requireCode(t, err, apperr.CodeValidation)

		_, err = f.svc.ReplaceAdvertDynamicProperties(ctx, f.owner, draft.ID, appadvert.ReplacePropertiesInput{
			ExpectedVersion: 1,
			Properties:      json.RawMessage(`{"color":"PINK"}`),
		})
		requireCode(t, err, apperr.CodeValidation)

		_, err = f.svc.ReplaceAdvertDynamicProperties(ctx, f.owner, draft.ID, appadvert.ReplacePropertiesInput{
			ExpectedVersion: 1,
			Properties:      json.RawMessage(`[]`),
		})
		requireCode(t, err, apperr.CodeValidation)
	})

	t.Run("requires category", func(t *testing.T) {
		draft := f.seed(t, f.owner, domainadvert.StatusDraft, func(a *domainadvert.Advert) {
			a.CategoryID = nil
		})
		_, err := f.svc.ReplaceAdvertDynamicProperties(ctx, f.owner, draft.ID, appadvert.ReplacePropertiesInput{
			ExpectedVersion: 1,
			Properties:      json.RawMessage(`{"age":4}`),
		})
		requireCode(t, err, apperr.CodeInvalidState)
	})

	t.Run("stale version", func(t *testing.T) {
		draft := f.seed(t, f.owner, domainadvert.StatusDraft, nil)
		_, err := f.svc.ReplaceAdvertDynamicProperties(ctx, f.owner, draft.ID, appadvert.ReplacePropertiesInput{
			ExpectedVersion: 9,
			Properties:      json.RawMessage(`{"age":4}`),
		})
		requireCode(t, err, apperr.CodeStaleVersion)
	})
}

func TestSoftDeleteAdvertDraft(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	draft := f.seed(t, f.owner, domainadvert.StatusDraft, nil)

	view, err := f.svc.SoftDeleteAdvertDraft(ctx, f.owner, draft.ID, 1)
	if err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if view.DeletedAt == nil || view.Version != 2 {
		t.Fatalf("view=%+v", view)
	}
	if len(f.store.History()) != 0 {
		t.Fatalf("soft delete must not write history: %+v", f.store.History())
	}

	// Already deleted stays a success and writes nothing further.
	again, err := f.svc.SoftDeleteAdvertDraft(ctx, f.owner, draft.ID, 2)
	if err != nil {
		t.Fatalf("idempotent soft delete: %v", err)
	}
	if again.Version != 2 || again.DeletedAt == nil {
		t.Fatalf("again=%+v", again)
	}

	listed, err := f.svc.ListMyAdverts(ctx, f.owner, appadvert.ListInput{})
	if err != nil || len(listed.Items) != 0 {
		t.Fatalf("deleted draft must not be listed: %+v err=%v", listed.Items, err)
	}
}

func TestSoftDeleteAdvertDraftAllowsPublished(t *testing.T) {
	f := newFixture(t)
	published := f.seed(t, f.owner, domainadvert.StatusPublished, nil)

	view, err := f.svc.SoftDeleteAdvertDraft(context.Background(), f.owner, published.ID, 1)
	if err != nil {
		t.Fatalf("soft delete published: %v", err)
	}
	if view.DeletedAt == nil {
		t.Fatalf("published advert must be soft deleted: %+v", view)
	}
}

func TestSubmitAdvertForReviewUnverifiedEmailAllowed(t *testing.T) {
	f := newFixture(t)
	unverified := uuid.New()
	f.store.PutUser(domainuser.User{ID: unverified, Status: domainuser.StatusActive})
	draft := f.seed(t, unverified, domainadvert.StatusDraft, nil)

	view, err := f.svc.SubmitAdvertForReview(context.Background(), unverified, draft.ID, 1)
	if err != nil {
		t.Fatalf("unverified owner must be able to submit: %v", err)
	}
	if view.Status != domainadvert.StatusPendingReview {
		t.Fatalf("status=%s", view.Status)
	}
	if len(f.store.History()) != 1 {
		t.Fatalf("history=%+v", f.store.History())
	}
}

func TestSubmitAdvertForReviewInactiveAccount(t *testing.T) {
	f := newFixture(t)
	verified := f.clock.Now()
	disabled := uuid.New()
	f.store.PutUser(domainuser.User{ID: disabled, Status: domainuser.StatusDisabled, EmailVerifiedAt: &verified})
	draft := f.seed(t, disabled, domainadvert.StatusDraft, nil)

	_, err := f.svc.SubmitAdvertForReview(context.Background(), disabled, draft.ID, 1)
	requireCode(t, err, apperr.CodeAccountInactive)
}

func TestSubmitAdvertForReviewSuccess(t *testing.T) {
	f := newFixture(t)
	draft := f.seed(t, f.owner, domainadvert.StatusDraft, nil)

	view, err := f.svc.SubmitAdvertForReview(context.Background(), f.owner, draft.ID, 1)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if view.Status != domainadvert.StatusPendingReview || view.Version != 2 {
		t.Fatalf("view status=%s version=%d", view.Status, view.Version)
	}
	if view.PublishedAt != nil {
		t.Fatalf("submit must not publish: %v", view.PublishedAt)
	}

	history := f.store.History()
	if len(history) != 1 {
		t.Fatalf("history=%+v", history)
	}
	if history[0].FromStatus == nil || *history[0].FromStatus != domainadvert.StatusDraft {
		t.Fatalf("from=%v", history[0].FromStatus)
	}
	if history[0].ToStatus != domainadvert.StatusPendingReview {
		t.Fatalf("to=%s", history[0].ToStatus)
	}
}

func TestSubmitAdvertForReviewFullValidation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	t.Run("missing core fields", func(t *testing.T) {
		draft := f.seed(t, f.owner, domainadvert.StatusDraft, func(a *domainadvert.Advert) {
			a.CategoryID = nil
			a.DistrictID = nil
			a.Title = nil
			a.Description = ptr("   ")
		})
		_, err := f.svc.SubmitAdvertForReview(ctx, f.owner, draft.ID, 1)
		ae := requireCode(t, err, apperr.CodeValidation)
		if len(ae.FieldErrors) != 4 {
			t.Fatalf("fields=%+v", ae.FieldErrors)
		}
	})

	t.Run("required property enforced on submit", func(t *testing.T) {
		draft := f.seed(t, f.owner, domainadvert.StatusDraft, func(a *domainadvert.Advert) {
			a.Properties = json.RawMessage(`{"color":"BAY"}`)
		})
		_, err := f.svc.SubmitAdvertForReview(ctx, f.owner, draft.ID, 1)
		ae := requireCode(t, err, apperr.CodeValidation)
		if len(ae.FieldErrors) != 1 || ae.FieldErrors[0].Field != "properties.age" {
			t.Fatalf("fields=%+v", ae.FieldErrors)
		}
	})

	t.Run("missing cover image", func(t *testing.T) {
		draft := f.seed(t, f.owner, domainadvert.StatusDraft, nil)
		f.store.PutMediaRelations(draft.ID, nil)
		_, err := f.svc.SubmitAdvertForReview(ctx, f.owner, draft.ID, 1)
		ae := requireCode(t, err, apperr.CodeValidation)
		if len(ae.FieldErrors) != 1 || ae.FieldErrors[0].Field != "media" {
			t.Fatalf("fields=%+v", ae.FieldErrors)
		}
	})

	t.Run("uploaded or validating cover image succeeds", func(t *testing.T) {
		draft := f.seed(t, f.owner, domainadvert.StatusDraft, nil)
		assetID := uuid.New()
		f.store.PutMediaRelations(draft.ID, []domainadvert.MediaRelation{{
			AssetID:         assetID,
			DisplayOrder:    0,
			IsCover:         true,
			LifecycleStatus: string(domainmedia.AssetUploaded),
		}})
		view, err := f.svc.SubmitAdvertForReview(ctx, f.owner, draft.ID, 1)
		if err != nil {
			t.Fatalf("submit with uploaded cover: %v", err)
		}
		if view.Status != domainadvert.StatusPendingReview {
			t.Fatalf("status=%s", view.Status)
		}
	})

	t.Run("wrong status and stale version", func(t *testing.T) {
		pending := f.seed(t, f.owner, domainadvert.StatusPendingReview, nil)
		_, err := f.svc.SubmitAdvertForReview(ctx, f.owner, pending.ID, 1)
		requireCode(t, err, apperr.CodeInvalidState)

		draft := f.seed(t, f.owner, domainadvert.StatusDraft, nil)
		_, err = f.svc.SubmitAdvertForReview(ctx, f.owner, draft.ID, 4)
		requireCode(t, err, apperr.CodeStaleVersion)
	})
}

func TestResubmitAdvertForReview(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	changes := f.seed(t, f.owner, domainadvert.StatusChangesRequested, nil)
	view, err := f.svc.ResubmitAdvertForReview(ctx, f.owner, changes.ID, 1)
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if view.Status != domainadvert.StatusPendingReview || view.Version != 2 {
		t.Fatalf("view=%+v", view)
	}

	draft := f.seed(t, f.owner, domainadvert.StatusDraft, nil)
	_, err = f.svc.ResubmitAdvertForReview(ctx, f.owner, draft.ID, 1)
	requireCode(t, err, apperr.CodeInvalidState)
}

func TestMarkAdvertSoldAndArchive(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	published := f.seed(t, f.owner, domainadvert.StatusPublished, nil)
	sold, err := f.svc.MarkAdvertSold(ctx, f.owner, published.ID, 1)
	if err != nil {
		t.Fatalf("sold: %v", err)
	}
	if sold.Status != domainadvert.StatusSold || sold.Version != 2 {
		t.Fatalf("sold=%+v", sold)
	}
	if sold.PublishedAt == nil {
		t.Fatalf("published_at must survive: %+v", sold)
	}

	other := f.seed(t, f.owner, domainadvert.StatusPublished, nil)
	archived, err := f.svc.ArchiveAdvert(ctx, f.owner, other.ID, 1)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if archived.Status != domainadvert.StatusArchived || archived.Version != 2 {
		t.Fatalf("archived=%+v", archived)
	}

	history := f.store.History()
	if len(history) != 2 {
		t.Fatalf("history=%+v", history)
	}
	if history[0].ToStatus != domainadvert.StatusSold || history[1].ToStatus != domainadvert.StatusArchived {
		t.Fatalf("history=%+v", history)
	}

	// Only PUBLISHED adverts may be sold or archived.
	draft := f.seed(t, f.owner, domainadvert.StatusDraft, nil)
	_, err = f.svc.MarkAdvertSold(ctx, f.owner, draft.ID, 1)
	requireCode(t, err, apperr.CodeInvalidState)
	_, err = f.svc.ArchiveAdvert(ctx, f.owner, draft.ID, 1)
	requireCode(t, err, apperr.CodeInvalidState)

	_, err = f.svc.MarkAdvertSold(ctx, f.owner, uuid.New(), 1)
	requireCode(t, err, apperr.CodeNotFound)
}

func TestExpectedVersionMustBePositive(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	draft := f.seed(t, f.owner, domainadvert.StatusDraft, nil)

	_, err := f.svc.SubmitAdvertForReview(ctx, f.owner, draft.ID, 0)
	requireCode(t, err, apperr.CodeValidation)

	_, err = f.svc.SoftDeleteAdvertDraft(ctx, f.owner, draft.ID, -1)
	requireCode(t, err, apperr.CodeValidation)

	_, err = f.svc.ChangeAdvertDraftCategory(ctx, f.owner, draft.ID, appadvert.ChangeCategoryInput{
		ExpectedVersion: 0,
		CategoryID:      f.category2,
	})
	requireCode(t, err, apperr.CodeValidation)
}
