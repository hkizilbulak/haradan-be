package favorite_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainfavorite "github.com/hkizilbulak/haradan-be/internal/domain/favorite"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	pgfavorite "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/favorite"
	"github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/testutil"
	pguser "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/user"
)

func seedFavoriteFixture(t *testing.T, ctx context.Context, tx pgx.Tx, now time.Time) (userID, strangerID, publishedID, draftID, provinceID, districtID, categoryID uuid.UUID) {
	t.Helper()
	users := pguser.NewRepository(tx)
	newUser := func(prefix string) uuid.UUID {
		email := prefix + "-" + uuid.NewString() + "@example.com"
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
	userID = newUser("fav-owner")
	strangerID = newUser("fav-stranger")

	provinceID = uuid.New()
	districtID = uuid.New()
	categoryID = uuid.New()
	if _, err := tx.Exec(ctx, `
INSERT INTO hrd_provinces (id, name, name_normalized, is_active, sort_order, created_at, updated_at)
VALUES ($1, 'FavProv', 'favprov', true, 1, $2, $2)`, provinceID, now); err != nil {
		t.Fatalf("province: %v", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO hrd_districts (id, province_id, name, name_normalized, is_active, sort_order, created_at, updated_at)
VALUES ($1, $2, 'FavDist', 'favdist', true, 1, $3, $3)`, districtID, provinceID, now); err != nil {
		t.Fatalf("district: %v", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO hrd_categories (
  id, parent_id, slug, name, is_active, sort_order, version, created_at, updated_at
) VALUES (
  $1, NULL, $2, 'FavCat', true, 1, 1, $3, $3
)`, categoryID, "fav-cat-"+uuid.NewString(), now); err != nil {
		t.Fatalf("category: %v", err)
	}

	publishedID = uuid.New()
	draftID = uuid.New()
	if _, err := tx.Exec(ctx, `
INSERT INTO hrd_adverts (
  id, owner_user_id, category_id, district_id, title, description, status,
  price_amount_minor, price_currency, properties, published_at, version, media_version,
  created_at, updated_at
) VALUES (
  $1, $2, $3, $4, 'Yayında', 'Açıklama', 'PUBLISHED',
  1000, 'TRY', '{}'::jsonb, $5, 1, 1, $5, $5
)`, publishedID, userID, categoryID, districtID, now); err != nil {
		t.Fatalf("published advert: %v", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO hrd_adverts (
  id, owner_user_id, status, properties, version, media_version, created_at, updated_at
) VALUES (
  $1, $2, 'DRAFT', '{}'::jsonb, 1, 1, $3, $3
)`, draftID, userID, now); err != nil {
		t.Fatalf("draft advert: %v", err)
	}
	return
}

func TestFavoriteRepositoryAddRemoveListIntegration(t *testing.T) {
	ctx, tx, cleanup := testutil.OpenTestTx(t)
	defer cleanup()

	now := time.Now().UTC().Truncate(time.Microsecond)
	userID, strangerID, publishedID, draftID, _, _, _ := seedFavoriteFixture(t, ctx, tx, now)
	repo := pgfavorite.NewRepository(tx)

	advert, err := repo.FindAdvertForFavoriteLookup(ctx, publishedID)
	if err != nil || advert.Status != "PUBLISHED" {
		t.Fatalf("%+v err=%v", advert, err)
	}
	_, err = repo.FindAdvertForFavoriteLookup(ctx, uuid.New())
	ae, _ := apperr.As(err)
	if ae == nil || ae.Code != apperr.CodeNotFound {
		t.Fatalf("err=%v", err)
	}

	fav := domainfavorite.Favorite{
		ID: uuid.New(), UserID: userID, AdvertID: publishedID, CreatedAt: now,
	}
	if err := repo.InsertFavorite(ctx, fav); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := repo.InsertFavorite(ctx, domainfavorite.Favorite{
		ID: uuid.New(), UserID: userID, AdvertID: publishedID, CreatedAt: now,
	}); !errors.Is(err, domainfavorite.ErrDuplicate) {
		t.Fatalf("duplicate=%v", err)
	}

	rows, err := repo.ListFavoritesByUser(ctx, userID, nil, nil, 10)
	if err != nil || len(rows) != 1 || rows[0].Advert.ProvinceID == nil {
		t.Fatalf("%+v err=%v", rows, err)
	}
	rows, err = repo.ListFavoritesByUser(ctx, strangerID, nil, nil, 10)
	if err != nil || len(rows) != 0 {
		t.Fatalf("stranger list=%+v err=%v", rows, err)
	}

	if err := repo.DeleteFavorite(ctx, strangerID, publishedID); err != nil {
		t.Fatalf("stranger delete: %v", err)
	}
	rows, err = repo.ListFavoritesByUser(ctx, userID, nil, nil, 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("owner favorite must remain: %+v", rows)
	}
	if err := repo.DeleteFavorite(ctx, userID, publishedID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := repo.DeleteFavorite(ctx, userID, publishedID); err != nil {
		t.Fatalf("idempotent delete: %v", err)
	}

	_ = draftID
}
