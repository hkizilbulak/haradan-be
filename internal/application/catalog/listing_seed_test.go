package catalog_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	appcatalog "github.com/hkizilbulak/haradan-be/internal/application/catalog"
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
)

func ptrUUID(id uuid.UUID) *uuid.UUID { return &id }

func TestPublicListingCatalogSeedShape(t *testing.T) {
	satilik := uuid.MustParse("c1000000-0000-4000-8000-000000000001")
	hizmet := uuid.MustParse("c1000000-0000-4000-8000-000000000002")
	asim := uuid.MustParse("c1000000-0000-4000-8000-000000000003")

	repo := &fakeCatalogRepo{categories: []domaincatalog.Category{
		{ID: hizmet, Slug: "at-hizmetleri", Name: "At Hizmetleri", SortOrder: 20, IsActive: true},
		{ID: asim, Slug: "asim-hizmetleri", Name: "Aşım Hizmetleri", SortOrder: 30, IsActive: true},
		{ID: satilik, Slug: "satilik-atlar", Name: "Satılık Atlar", SortOrder: 10, IsActive: true},
		{ID: uuid.MustParse("c1000000-0000-4000-8000-000000000015"), ParentID: ptrUUID(satilik), Slug: "satilik-pony", Name: "Satılık Pony", SortOrder: 50, IsActive: true},
		{ID: uuid.MustParse("c1000000-0000-4000-8000-000000000011"), ParentID: ptrUUID(satilik), Slug: "satilik-yaris-ati", Name: "Satılık Yarış Atı", SortOrder: 10, IsActive: true},
		{ID: uuid.MustParse("c1000000-0000-4000-8000-000000000012"), ParentID: ptrUUID(satilik), Slug: "satilik-kisrak", Name: "Satılık Kısrak", SortOrder: 20, IsActive: true},
		{ID: uuid.MustParse("c1000000-0000-4000-8000-000000000013"), ParentID: ptrUUID(satilik), Slug: "satilik-aygir", Name: "Satılık Aygır", SortOrder: 30, IsActive: true},
		{ID: uuid.MustParse("c1000000-0000-4000-8000-000000000014"), ParentID: ptrUUID(satilik), Slug: "satilik-binek-ati", Name: "Satılık Binek Atı", SortOrder: 40, IsActive: true},
		{ID: uuid.MustParse("c1000000-0000-4000-8000-000000000021"), ParentID: ptrUUID(hizmet), Slug: "pansiyon-haralar", Name: "Pansiyon Haralar", SortOrder: 10, IsActive: true},
		{ID: uuid.MustParse("c1000000-0000-4000-8000-000000000022"), ParentID: ptrUUID(hizmet), Slug: "at-nakliyesi", Name: "At Nakliyesi", SortOrder: 20, IsActive: true},
		{ID: uuid.MustParse("c1000000-0000-4000-8000-000000000023"), ParentID: ptrUUID(hizmet), Slug: "nalbantlar", Name: "Nalbantlar", SortOrder: 30, IsActive: true},
		{ID: uuid.MustParse("c1000000-0000-4000-8000-000000000031"), ParentID: ptrUUID(asim), Slug: "arap-aygir", Name: "Arap Aygır", SortOrder: 10, IsActive: true},
		{ID: uuid.MustParse("c1000000-0000-4000-8000-000000000032"), ParentID: ptrUUID(asim), Slug: "ingiliz-aygir", Name: "İngiliz Aygır", SortOrder: 20, IsActive: true},
	}}

	tree, err := appcatalog.NewService(repo).GetPublicCategoryTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 3 {
		t.Fatalf("roots=%d", len(tree))
	}

	wantRoots := []struct {
		slug     string
		name     string
		children []string
	}{
		{"satilik-atlar", "Satılık Atlar", []string{"satilik-yaris-ati", "satilik-kisrak", "satilik-aygir", "satilik-binek-ati", "satilik-pony"}},
		{"at-hizmetleri", "At Hizmetleri", []string{"pansiyon-haralar", "at-nakliyesi", "nalbantlar"}},
		{"asim-hizmetleri", "Aşım Hizmetleri", []string{"arap-aygir", "ingiliz-aygir"}},
	}
	for i, want := range wantRoots {
		got := tree[i]
		if got.Slug != want.slug || got.Name != want.name {
			t.Fatalf("root %d slug/name=%s/%s", i, got.Slug, got.Name)
		}
		if len(got.Children) != len(want.children) {
			t.Fatalf("root %s children=%d", want.slug, len(got.Children))
		}
		for j, childSlug := range want.children {
			if got.Children[j].Slug != childSlug {
				t.Fatalf("root %s child %d=%s", want.slug, j, got.Children[j].Slug)
			}
			if len(got.Children[j].Children) != 0 {
				t.Fatalf("leaf %s must have no children", childSlug)
			}
		}
	}

	leaf := tree[0].Children[0]
	form, err := appcatalog.NewService(repo).GetCategoryFormDefinition(context.Background(), leaf.ID)
	if err != nil {
		t.Fatal(err)
	}
	if form.Category.Slug != "satilik-yaris-ati" {
		t.Fatalf("form slug=%s", form.Category.Slug)
	}
}
