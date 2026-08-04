package horse_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	apphorse "github.com/hkizilbulak/haradan-be/internal/application/horse"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainhorse "github.com/hkizilbulak/haradan-be/internal/domain/horse"
)

type fakeHorseRepo struct {
	byID       map[uuid.UUID]domainhorse.Horse
	byTJK      map[string]domainhorse.Horse
	prefixHits []domainhorse.Horse
	findIDErr  error
	findTJKErr error
	searchErr  error
	lastPrefix string
	lastLimit  int
	lastTJK    string
}

func (f *fakeHorseRepo) FindByID(_ context.Context, id uuid.UUID) (domainhorse.Horse, error) {
	if f.findIDErr != nil {
		return domainhorse.Horse{}, f.findIDErr
	}
	h, ok := f.byID[id]
	if !ok {
		return domainhorse.Horse{}, apperr.NotFound("At bulunamadı.")
	}
	return h, nil
}

func (f *fakeHorseRepo) FindByTJKNumber(_ context.Context, tjkNumber string) (domainhorse.Horse, error) {
	f.lastTJK = tjkNumber
	if f.findTJKErr != nil {
		return domainhorse.Horse{}, f.findTJKErr
	}
	h, ok := f.byTJK[tjkNumber]
	if !ok {
		return domainhorse.Horse{}, apperr.NotFound("At bulunamadı.")
	}
	return h, nil
}

func (f *fakeHorseRepo) SearchByNormalizedPrefix(_ context.Context, prefix string, limit int) ([]domainhorse.Horse, error) {
	f.lastPrefix = prefix
	f.lastLimit = limit
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	out := append([]domainhorse.Horse(nil), f.prefixHits...)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func sampleHorse(id uuid.UUID, name, tjk, normalized string) domainhorse.Horse {
	year := 2018
	sire := "Baba"
	dam := "Anne"
	breed := "İngiliz"
	gender := "Erkek"
	coat := "Doru"
	return domainhorse.Horse{
		ID:             id,
		TJKNumber:      tjk,
		OriginalName:   name,
		NameNormalized: normalized,
		BirthYear:      &year,
		SireName:       &sire,
		DamName:        &dam,
		Breed:          &breed,
		Gender:         &gender,
		Coat:           &coat,
		Detail:         json.RawMessage(`{"pedigree":{"note":"x"}}`),
	}
}

func TestSearchPrefixNormalizesAndLimits(t *testing.T) {
	id := uuid.New()
	repo := &fakeHorseRepo{
		prefixHits: []domainhorse.Horse{sampleHorse(id, "İstanbul", "TJK-1", "istanbul")},
	}
	svc := apphorse.NewService(repo)
	q := "İST"
	limit := 5
	got, err := svc.SearchForSelection(context.Background(), &q, nil, &limit)
	if err != nil || len(got) != 1 {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if repo.lastPrefix != "ist" || repo.lastLimit != 5 {
		t.Fatalf("prefix=%q limit=%d", repo.lastPrefix, repo.lastLimit)
	}
	if got[0].OriginalName != "İstanbul" || got[0].TJKNumber != "TJK-1" {
		t.Fatalf("%+v", got[0])
	}
}

func TestSearchEmptyQueryReturnsEmptySlice(t *testing.T) {
	svc := apphorse.NewService(&fakeHorseRepo{})
	got, err := svc.SearchForSelection(context.Background(), nil, nil, nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestSearchByTJKNumberLocalHitAndMiss(t *testing.T) {
	id := uuid.New()
	h := sampleHorse(id, "Ada", " 12345 ", "ada")
	h.TJKNumber = "12345"
	repo := &fakeHorseRepo{byTJK: map[string]domainhorse.Horse{"12345": h}}
	svc := apphorse.NewService(repo)

	tjk := " 12345 "
	got, err := svc.SearchForSelection(context.Background(), nil, &tjk, nil)
	if err != nil || len(got) != 1 || got[0].ID != id {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if repo.lastTJK != "12345" {
		t.Fatalf("normalized tjk=%q", repo.lastTJK)
	}

	miss := "missing"
	got, err = svc.SearchForSelection(context.Background(), nil, &miss, nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("miss got=%v err=%v", got, err)
	}
}

func TestSearchTJKTooLongValidation(t *testing.T) {
	svc := apphorse.NewService(&fakeHorseRepo{})
	long := strings.Repeat("1", 65)
	_, err := svc.SearchForSelection(context.Background(), nil, &long, nil)
	ae, _ := apperr.As(err)
	if ae == nil || ae.Kind != apperr.KindValidation {
		t.Fatalf("err=%v", err)
	}
}

func TestSearchInvalidLimit(t *testing.T) {
	svc := apphorse.NewService(&fakeHorseRepo{})
	bad := 101
	_, err := svc.SearchForSelection(context.Background(), nil, nil, &bad)
	ae, _ := apperr.As(err)
	if ae == nil || ae.Kind != apperr.KindValidation {
		t.Fatalf("err=%v", err)
	}
}

func TestSearchBothFiltersRequirePrefixMatch(t *testing.T) {
	id := uuid.New()
	h := sampleHorse(id, "Ada", "99", "ada")
	repo := &fakeHorseRepo{byTJK: map[string]domainhorse.Horse{"99": h}}
	svc := apphorse.NewService(repo)
	q := "ada"
	tjk := "99"
	got, err := svc.SearchForSelection(context.Background(), &q, &tjk, nil)
	if err != nil || len(got) != 1 {
		t.Fatalf("got=%v err=%v", got, err)
	}
	q = "zzz"
	got, err = svc.SearchForSelection(context.Background(), &q, &tjk, nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("filtered got=%v err=%v", got, err)
	}
}

func TestGetPublicDetailSuccessAndNotFound(t *testing.T) {
	id := uuid.New()
	h := sampleHorse(id, "Ada", "1", "ada")
	repo := &fakeHorseRepo{byID: map[uuid.UUID]domainhorse.Horse{id: h}}
	svc := apphorse.NewService(repo)

	got, err := svc.GetPublicDetail(context.Background(), id)
	if err != nil || got.OriginalName != "Ada" || got.Breed == nil || *got.Breed != "İngiliz" {
		t.Fatalf("%+v err=%v", got, err)
	}
	if string(got.Detail) != `{"pedigree":{"note":"x"}}` {
		t.Fatalf("detail=%s", got.Detail)
	}
	// Internal sync fields are not part of PublicDetail type.
	if got.TJKNumber != "1" {
		t.Fatalf("%+v", got)
	}

	_, err = svc.GetPublicDetail(context.Background(), uuid.New())
	ae, _ := apperr.As(err)
	if ae == nil || ae.Code != apperr.CodeNotFound {
		t.Fatalf("err=%v", err)
	}
}

func TestGetPublicDetailRepositoryError(t *testing.T) {
	svc := apphorse.NewService(&fakeHorseRepo{findIDErr: errors.New("boom")})
	_, err := svc.GetPublicDetail(context.Background(), uuid.New())
	ae, _ := apperr.As(err)
	if ae == nil || ae.Kind != apperr.KindInternal {
		t.Fatalf("err=%v", err)
	}
}

func TestSelectionOmitsInternalFields(t *testing.T) {
	id := uuid.New()
	h := sampleHorse(id, "Ada", "1", "ada")
	repo := &fakeHorseRepo{prefixHits: []domainhorse.Horse{h}}
	svc := apphorse.NewService(repo)
	q := "a"
	got, err := svc.SearchForSelection(context.Background(), &q, nil, nil)
	if err != nil || len(got) != 1 {
		t.Fatalf("got=%v err=%v", got, err)
	}
	raw, _ := json.Marshal(got[0])
	s := string(raw)
	for _, leak := range []string{"nameNormalized", "lastSyncedAt", "password", "detail"} {
		if strings.Contains(s, leak) {
			t.Fatalf("leaked %s in %s", leak, s)
		}
	}
}
