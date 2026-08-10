package advert_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domainadvert "github.com/hkizilbulak/haradan-be/internal/domain/advert"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	pgadvert "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/advert"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/testutil"
	pguser "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/user"
)

type refs struct {
	owner    uuid.UUID
	stranger uuid.UUID
	category uuid.UUID
	other    uuid.UUID
	district uuid.UUID
}

func seedRefs(t *testing.T, ctx context.Context, tx pgx.Tx, now time.Time) refs {
	t.Helper()
	users := pguser.NewRepository(tx)

	newUser := func(email string) uuid.UUID {
		u := domainuser.User{
			ID: uuid.New(), Email: email, EmailNormalized: email,
			PasswordHash: "hash", Role: domainuser.RoleUser, Status: domainuser.StatusActive,
			FirstName: "A", LastName: "B", SecurityStamp: uuid.New(), CreatedAt: now, UpdatedAt: now,
		}
		if err := users.Create(ctx, u); err != nil {
			t.Fatalf("create user: %v", err)
		}
		return u.ID
	}

	out := refs{
		owner:    newUser("advert-owner-" + uuid.NewString() + "@example.com"),
		stranger: newUser("advert-stranger-" + uuid.NewString() + "@example.com"),
		category: uuid.New(),
		other:    uuid.New(),
		district: uuid.New(),
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO hrd_categories (id, parent_id, slug, name, description, is_active, sort_order, version, created_at, updated_at)
VALUES ($1, NULL, $3, 'Leaf A', NULL, true, 1, 1, $5, $5),
       ($2, NULL, $4, 'Leaf B', NULL, true, 2, 1, $5, $5)`,
		out.category, out.other,
		"leaf-a-"+out.category.String()[:8], "leaf-b-"+out.other.String()[:8], now,
	); err != nil {
		t.Fatalf("insert categories: %v", err)
	}

	provinceID := uuid.New()
	if _, err := tx.Exec(ctx, `
INSERT INTO hrd_provinces (id, name, name_normalized, is_active, sort_order, created_at, updated_at)
VALUES ($1, $2, $3, true, 1, $4, $4)`,
		provinceID, "Prov "+provinceID.String()[:8], "prov-"+provinceID.String()[:8], now,
	); err != nil {
		t.Fatalf("insert province: %v", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO hrd_districts (id, province_id, name, name_normalized, is_active, sort_order, created_at, updated_at)
VALUES ($1, $2, $3, $4, true, 1, $5, $5)`,
		out.district, provinceID, "Dist "+out.district.String()[:8], "dist-"+out.district.String()[:8], now,
	); err != nil {
		t.Fatalf("insert district: %v", err)
	}
	return out
}

func newDraft(ownerID uuid.UUID, now time.Time) domainadvert.Advert {
	return domainadvert.Advert{
		ID:           uuid.New(),
		OwnerUserID:  ownerID,
		Status:       domainadvert.StatusDraft,
		Properties:   domainadvert.EmptyProperties(),
		Version:      1,
		MediaVersion: 1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func TestRepositoryAdvertLifecycleIntegration(t *testing.T) {
	ctx, tx, cleanup := testutil.OpenTestTx(t)
	defer cleanup()

	now := time.Now().UTC().Truncate(time.Microsecond)
	ref := seedRefs(t, ctx, tx, now)
	repo := pgadvert.NewRepository(nil).WithTx(tx)

	draft := newDraft(ref.owner, now)
	if err := repo.Create(ctx, draft); err != nil {
		t.Fatalf("create advert: %v", err)
	}
	if err := repo.InsertHistory(ctx, domainadvert.StatusHistory{
		ID:          uuid.New(),
		AdvertID:    draft.ID,
		ToStatus:    domainadvert.StatusDraft,
		ActorUserID: &ref.owner,
		CreatedAt:   now,
	}); err != nil {
		t.Fatalf("insert initial history: %v", err)
	}

	found, err := repo.FindByIDForOwner(ctx, ref.owner, draft.ID)
	if err != nil {
		t.Fatalf("find for owner: %v", err)
	}
	if found.Status != domainadvert.StatusDraft || found.Version != 1 || string(found.Properties) != "{}" {
		t.Fatalf("found=%+v", found)
	}

	// A foreign owner must not be able to tell the advert exists.
	if _, err := repo.FindByIDForOwner(ctx, ref.stranger, draft.ID); err == nil {
		t.Fatal("cross-user read must fail")
	} else if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("want NOT_FOUND, got %v", err)
	}

	title := "Satılık at"
	description := "Açıklama"
	updated, err := repo.UpdateDetails(ctx, ref.owner, draft.ID, domainadvert.DetailsPatch{
		DistrictIDSet:  true,
		DistrictID:     &ref.district,
		TitleSet:       true,
		Title:          &title,
		DescriptionSet: true,
		Description:    &description,
		PriceSet:       true,
		Price:          &domainadvert.Money{AmountMinor: 250000, Currency: "TRY"},
	}, 1, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("update details: %v", err)
	}
	if updated.Version != 2 || updated.Title == nil || *updated.Title != title {
		t.Fatalf("updated=%+v", updated)
	}
	if updated.Price == nil || updated.Price.AmountMinor != 250000 || updated.Price.Currency != "TRY" {
		t.Fatalf("price=%+v", updated.Price)
	}
	if updated.DistrictID == nil || *updated.DistrictID != ref.district {
		t.Fatalf("districtId=%v", updated.DistrictID)
	}

	// Replaying the same expected version must be rejected.
	if _, err := repo.UpdateDetails(ctx, ref.owner, draft.ID, domainadvert.DetailsPatch{
		TitleSet: true,
		Title:    &title,
	}, 1, now.Add(2*time.Minute)); err == nil {
		t.Fatal("stale update must fail")
	} else if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeStaleVersion {
		t.Fatalf("want STALE_VERSION, got %v", err)
	}

	withProps, err := repo.ReplaceProperties(ctx, ref.owner, draft.ID, json.RawMessage(`{"age":5}`), 2, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("replace properties: %v", err)
	}
	if withProps.Version != 3 {
		t.Fatalf("withProps=%+v", withProps)
	}
	var storedProps map[string]int
	if err := json.Unmarshal(withProps.Properties, &storedProps); err != nil {
		t.Fatalf("decode properties %s: %v", withProps.Properties, err)
	}
	if storedProps["age"] != 5 {
		t.Fatalf("properties=%s", withProps.Properties)
	}

	categorized, err := repo.UpdateCategoryClearProperties(ctx, ref.owner, draft.ID, ref.category, 3, now.Add(4*time.Minute))
	if err != nil {
		t.Fatalf("change category: %v", err)
	}
	if categorized.Version != 4 || categorized.CategoryID == nil || *categorized.CategoryID != ref.category {
		t.Fatalf("categorized=%+v", categorized)
	}
	if string(categorized.Properties) != "{}" {
		t.Fatalf("properties must be cleared, got %s", categorized.Properties)
	}

	pending, err := repo.TransitionStatus(
		ctx, ref.owner, draft.ID,
		domainadvert.StatusDraft, domainadvert.StatusPendingReview,
		4, nil, now.Add(5*time.Minute),
	)
	if err != nil {
		t.Fatalf("transition to pending review: %v", err)
	}
	if pending.Status != domainadvert.StatusPendingReview || pending.Version != 5 || pending.PublishedAt != nil {
		t.Fatalf("pending=%+v", pending)
	}
	if err := repo.InsertHistory(ctx, domainadvert.StatusHistory{
		ID:          uuid.New(),
		AdvertID:    draft.ID,
		FromStatus:  statusPtr(domainadvert.StatusDraft),
		ToStatus:    domainadvert.StatusPendingReview,
		ActorUserID: &ref.owner,
		CreatedAt:   now.Add(5 * time.Minute),
	}); err != nil {
		t.Fatalf("insert transition history: %v", err)
	}

	// Wrong from status is rejected without touching the row.
	if _, err := repo.TransitionStatus(
		ctx, ref.owner, draft.ID,
		domainadvert.StatusDraft, domainadvert.StatusPendingReview,
		5, nil, now.Add(6*time.Minute),
	); err == nil {
		t.Fatal("transition from wrong status must fail")
	} else if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeStaleVersion {
		t.Fatalf("want STALE_VERSION, got %v", err)
	}

	var historyCount int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrd_advert_status_history WHERE advert_id = $1`, draft.ID,
	).Scan(&historyCount); err != nil {
		t.Fatalf("count history: %v", err)
	}
	if historyCount != 2 {
		t.Fatalf("historyCount=%d", historyCount)
	}
}

func TestRepositoryListByOwnerOrderAndSoftDeleteIntegration(t *testing.T) {
	ctx, tx, cleanup := testutil.OpenTestTx(t)
	defer cleanup()

	now := time.Now().UTC().Truncate(time.Microsecond)
	ref := seedRefs(t, ctx, tx, now)
	repo := pgadvert.NewRepository(nil).WithTx(tx)

	oldest := newDraft(ref.owner, now.Add(-2*time.Hour))
	middle := newDraft(ref.owner, now.Add(-time.Hour))
	newest := newDraft(ref.owner, now)
	foreign := newDraft(ref.stranger, now)
	for _, a := range []domainadvert.Advert{oldest, middle, newest, foreign} {
		if err := repo.Create(ctx, a); err != nil {
			t.Fatalf("create advert: %v", err)
		}
	}

	page1, err := repo.ListByOwner(ctx, ref.owner, nil, nil, nil, 2)
	if err != nil {
		t.Fatalf("list page 1: %v", err)
	}
	if len(page1) != 2 || page1[0].ID != newest.ID || page1[1].ID != middle.ID {
		t.Fatalf("page1=%+v", page1)
	}

	cursorCreated := page1[1].CreatedAt
	page2, err := repo.ListByOwner(ctx, ref.owner, nil, &cursorCreated, &page1[1].ID, 2)
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != oldest.ID {
		t.Fatalf("page2=%+v", page2)
	}

	deleted, err := repo.SoftDeleteDraft(ctx, ref.owner, newest.ID, 1, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if deleted.DeletedAt == nil || deleted.Version != 2 {
		t.Fatalf("deleted=%+v", deleted)
	}

	remaining, err := repo.ListByOwner(ctx, ref.owner, nil, nil, nil, 10)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("soft-deleted advert must be excluded: %+v", remaining)
	}

	draftStatus := domainadvert.StatusDraft
	filtered, err := repo.ListByOwner(ctx, ref.owner, &draftStatus, nil, nil, 10)
	if err != nil {
		t.Fatalf("list filtered: %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("filtered=%+v", filtered)
	}

	soldStatus := domainadvert.StatusSold
	empty, err := repo.ListByOwner(ctx, ref.owner, &soldStatus, nil, nil, 10)
	if err != nil {
		t.Fatalf("list sold: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("empty=%+v", empty)
	}
}

func TestRepositoryAdvertModerationIntegration(t *testing.T) {
	ctx, tx, cleanup := testutil.OpenTestTx(t)
	defer cleanup()

	now := time.Now().UTC().Truncate(time.Microsecond)
	ref := seedRefs(t, ctx, tx, now)
	repo := pgadvert.NewRepository(nil).WithTx(tx)

	adminID := uuid.New()
	users := pguser.NewRepository(tx)
	if err := users.Create(ctx, domainuser.User{
		ID: adminID, Email: "admin-" + adminID.String()[:8] + "@example.com",
		EmailNormalized: "admin-" + adminID.String()[:8] + "@example.com",
		PasswordHash:    "hash", Role: domainuser.RoleAdmin, Status: domainuser.StatusActive,
		FirstName: "A", LastName: "B", SecurityStamp: uuid.New(), CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create admin: %v", err)
	}

	title := "Moderasyon"
	desc := "Açıklama"
	pending := domainadvert.Advert{
		ID: uuid.New(), OwnerUserID: ref.owner,
		CategoryID: &ref.category, DistrictID: &ref.district,
		Title: &title, Description: &desc,
		Status: domainadvert.StatusPendingReview, Properties: domainadvert.EmptyProperties(),
		Version: 1, MediaVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Create(ctx, pending); err != nil {
		t.Fatalf("create pending: %v", err)
	}

	listed, err := repo.ListForModeration(ctx, domainadvert.StatusPendingReview, nil, nil, 10)
	if err != nil {
		t.Fatalf("list moderation: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != pending.ID {
		t.Fatalf("%+v", listed)
	}

	found, err := repo.FindByID(ctx, pending.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if found.OwnerUserID != ref.owner {
		t.Fatalf("%+v", found)
	}

	locked, err := repo.FindByIDForUpdate(ctx, pending.ID)
	if err != nil {
		t.Fatalf("for update: %v", err)
	}
	publishedAt := now.Add(time.Minute)
	updated, err := repo.TransitionStatus(
		ctx, locked.OwnerUserID, pending.ID,
		domainadvert.StatusPendingReview, domainadvert.StatusPublished,
		1, &publishedAt, publishedAt,
	)
	if err != nil {
		t.Fatalf("transition: %v", err)
	}
	if updated.Status != domainadvert.StatusPublished || updated.PublishedAt == nil || updated.Version != 2 {
		t.Fatalf("%+v", updated)
	}
	from := domainadvert.StatusPendingReview
	if err := repo.InsertHistory(ctx, domainadvert.StatusHistory{
		ID: uuid.New(), AdvertID: pending.ID, FromStatus: &from,
		ToStatus: domainadvert.StatusPublished, ActorUserID: &adminID,
		IsSystem: false, CreatedAt: publishedAt,
	}); err != nil {
		t.Fatalf("history: %v", err)
	}

	hist, err := repo.ListStatusHistory(ctx, pending.ID)
	if err != nil {
		t.Fatalf("list history: %v", err)
	}
	if len(hist) != 1 || hist[0].ActorUserID == nil || *hist[0].ActorUserID != adminID {
		t.Fatalf("%+v", hist)
	}

	if _, err := repo.TransitionStatus(
		ctx, ref.owner, pending.ID,
		domainadvert.StatusPendingReview, domainadvert.StatusPublished,
		2, nil, publishedAt.Add(time.Minute),
	); err == nil {
		t.Fatal("stale/invalid transition must fail")
	} else if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeStaleVersion {
		t.Fatalf("want STALE_VERSION, got %v", err)
	}

	if stillPublished, err := repo.FindByID(ctx, pending.ID); err != nil || stillPublished.Status != domainadvert.StatusPublished {
		t.Fatalf("published advert must remain visible: %+v err=%v", stillPublished, err)
	}

	draft := newDraft(ref.owner, publishedAt.Add(2*time.Minute))
	if err := repo.Create(ctx, draft); err != nil {
		t.Fatalf("create draft: %v", err)
	}
	deletedAt := publishedAt.Add(3 * time.Minute)
	if _, err := repo.SoftDeleteDraft(ctx, ref.owner, draft.ID, 1, deletedAt); err != nil {
		t.Fatalf("soft delete draft: %v", err)
	}
	if _, err := repo.FindByID(ctx, draft.ID); err == nil {
		t.Fatal("soft-deleted draft must be not found")
	} else if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("want NOT_FOUND, got %v", err)
	}
}

func statusPtr(s domainadvert.Status) *domainadvert.Status { return &s }
