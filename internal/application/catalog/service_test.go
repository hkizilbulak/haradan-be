package catalog_test

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	appcatalog "github.com/hkizilbulak/haradan-be/internal/application/catalog"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
)

type fakeCatalogRepo struct {
	categories []domaincatalog.Category
	props      map[uuid.UUID][]domaincatalog.Property
	listErr    error
	getErr     error
	countErr   error
	propsErr   error
	childCount map[uuid.UUID]int
}

func (f *fakeCatalogRepo) ListActiveCategories(context.Context) ([]domaincatalog.Category, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]domaincatalog.Category(nil), f.categories...), nil
}

func (f *fakeCatalogRepo) GetActiveCategory(_ context.Context, id uuid.UUID) (domaincatalog.Category, error) {
	if f.getErr != nil {
		return domaincatalog.Category{}, f.getErr
	}
	for _, c := range f.categories {
		if c.ID == id {
			return c, nil
		}
	}
	return domaincatalog.Category{}, apperr.NotFound("Kategori bulunamadı.")
}

func (f *fakeCatalogRepo) CountActiveChildren(_ context.Context, parentID uuid.UUID) (int, error) {
	if f.countErr != nil {
		return 0, f.countErr
	}
	if f.childCount != nil {
		return f.childCount[parentID], nil
	}
	n := 0
	for _, c := range f.categories {
		if c.ParentID != nil && *c.ParentID == parentID {
			n++
		}
	}
	return n, nil
}

func (f *fakeCatalogRepo) ListFormProperties(_ context.Context, categoryID uuid.UUID) ([]domaincatalog.Property, error) {
	if f.propsErr != nil {
		return nil, f.propsErr
	}
	var result []domaincatalog.Property
	seenCodes := make(map[string]struct{})
	visited := make(map[uuid.UUID]struct{})

	currID := &categoryID
	isChild := true
	for currID != nil {
		if _, ok := visited[*currID]; ok {
			break
		}
		visited[*currID] = struct{}{}

		props := f.props[*currID]
		for _, p := range props {
			if isChild {
				result = append(result, p)
			} else {
				if _, exists := seenCodes[p.Code]; !exists {
					seenCodes[p.Code] = struct{}{}
					result = append(result, p)
				}
			}
		}

		if isChild {
			for _, p := range props {
				seenCodes[p.Code] = struct{}{}
			}
			isChild = false
		}

		var parentID *uuid.UUID
		for _, c := range f.categories {
			if c.ID == *currID && c.IsActive {
				parentID = c.ParentID
				break
			}
		}
		currID = parentID
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].SortOrder != result[j].SortOrder {
			return result[i].SortOrder < result[j].SortOrder
		}
		if result[i].Code != result[j].Code {
			return result[i].Code < result[j].Code
		}
		return result[i].ID.String() < result[j].ID.String()
	})

	return result, nil
}

func (f *fakeCatalogRepo) ListCategoriesAdmin(_ context.Context, active *bool, limit int) ([]domaincatalog.Category, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := []domaincatalog.Category{}
	for _, c := range f.categories {
		if active == nil || c.IsActive == *active {
			out = append(out, c)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeCatalogRepo) GetCategoryAdmin(_ context.Context, id uuid.UUID) (domaincatalog.Category, error) {
	if f.getErr != nil {
		return domaincatalog.Category{}, f.getErr
	}
	for _, c := range f.categories {
		if c.ID == id {
			return c, nil
		}
	}
	return domaincatalog.Category{}, apperr.NotFound("Kategori bulunamadı.")
}

func (f *fakeCatalogRepo) CreateCategory(_ context.Context, c domaincatalog.Category) (domaincatalog.Category, error) {
	f.categories = append(f.categories, c)
	return c, nil
}

func (f *fakeCatalogRepo) UpdateCategory(_ context.Context, id uuid.UUID, p domaincatalog.CategoryPatch, expected int, now time.Time) (domaincatalog.Category, error) {
	for i, c := range f.categories {
		if c.ID == id {
			if c.Version != expected {
				return domaincatalog.Category{}, apperr.StaleVersion("stale version")
			}
			if p.SlugSet {
				c.Slug = p.Slug
			}
			if p.NameSet {
				c.Name = p.Name
			}
			if p.DescriptionSet {
				c.Description = p.Description
			}
			if p.SortOrderSet {
				c.SortOrder = p.SortOrder
			}
			c.Version++
			c.UpdatedAt = now
			f.categories[i] = c
			return c, nil
		}
	}
	return domaincatalog.Category{}, apperr.NotFound("Kategori bulunamadı.")
}

func (f *fakeCatalogRepo) SetCategoryActive(_ context.Context, id uuid.UUID, active bool, expected int, now time.Time) (domaincatalog.Category, error) {
	for i, c := range f.categories {
		if c.ID == id {
			c.IsActive = active
			c.Version++
			c.UpdatedAt = now
			f.categories[i] = c
			return c, nil
		}
	}
	return domaincatalog.Category{}, apperr.NotFound("Kategori bulunamadı.")
}

func (f *fakeCatalogRepo) ReparentCategory(_ context.Context, id uuid.UUID, parent *uuid.UUID, expected int, now time.Time) (domaincatalog.Category, error) {
	for i, c := range f.categories {
		if c.ID == id {
			c.ParentID = parent
			c.Version++
			c.UpdatedAt = now
			f.categories[i] = c
			return c, nil
		}
	}
	return domaincatalog.Category{}, apperr.NotFound("Kategori bulunamadı.")
}

func (f *fakeCatalogRepo) IsDescendant(_ context.Context, child, parent uuid.UUID) (bool, error) {
	return false, nil
}

func (f *fakeCatalogRepo) ReorderCategories(_ context.Context, items []domaincatalog.ReorderItem, now time.Time) error {
	return nil
}

func (f *fakeCatalogRepo) ListPropertiesAdmin(_ context.Context, cid uuid.UUID) ([]domaincatalog.Property, error) {
	return append([]domaincatalog.Property(nil), f.props[cid]...), nil
}

func (f *fakeCatalogRepo) CreateProperty(_ context.Context, p domaincatalog.Property, now time.Time) (domaincatalog.Property, error) {
	if f.props == nil {
		f.props = map[uuid.UUID][]domaincatalog.Property{}
	}
	f.props[p.CategoryID] = append(f.props[p.CategoryID], p)
	return p, nil
}

func (f *fakeCatalogRepo) UpdateProperty(_ context.Context, pid, cid uuid.UUID, p domaincatalog.PropertyPatch, expected int, now time.Time) (domaincatalog.Property, error) {
	return domaincatalog.Property{}, nil
}

func (f *fakeCatalogRepo) SetPropertyActive(_ context.Context, pid, cid uuid.UUID, active bool, expected int, now time.Time) (domaincatalog.Property, error) {
	return domaincatalog.Property{}, nil
}

func (f *fakeCatalogRepo) ReorderProperties(_ context.Context, items []domaincatalog.ReorderItem, now time.Time) error {
	return nil
}

func TestGetPublicCategoryTreeBuildsDeterministicForest(t *testing.T) {
	rootA := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	rootB := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	child := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	repo := &fakeCatalogRepo{categories: []domaincatalog.Category{
		{ID: rootB, Slug: "b", Name: "B", SortOrder: 2},
		{ID: rootA, Slug: "a", Name: "A", SortOrder: 1},
		{ID: child, ParentID: &rootA, Slug: "a-child", Name: "Child", SortOrder: 1},
	}}
	svc := appcatalog.NewService(repo)
	tree, err := svc.GetPublicCategoryTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 2 || tree[0].Slug != "a" || tree[1].Slug != "b" {
		t.Fatalf("roots=%+v", tree)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].Slug != "a-child" {
		t.Fatalf("children=%+v", tree[0].Children)
	}
}

func TestGetPublicCategoryTreeEmptyAndRepoError(t *testing.T) {
	svc := appcatalog.NewService(&fakeCatalogRepo{})
	tree, err := svc.GetPublicCategoryTree(context.Background())
	if err != nil || len(tree) != 0 {
		t.Fatalf("got=%v err=%v", tree, err)
	}

	svc = appcatalog.NewService(&fakeCatalogRepo{listErr: errors.New("boom")})
	_, err = svc.GetPublicCategoryTree(context.Background())
	ae, ok := apperr.As(err)
	if !ok || ae.Kind != apperr.KindInternal {
		t.Fatalf("want internal, got %v", err)
	}
}

func TestGetCategoryFormDefinitionOrderingNotFoundInvalidState(t *testing.T) {
	leaf := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	parent := uuid.MustParse("00000000-0000-0000-0000-000000000011")
	repo := &fakeCatalogRepo{
		categories: []domaincatalog.Category{
			{ID: leaf, Slug: "leaf", Name: "Leaf", SortOrder: 1},
			{ID: parent, Slug: "parent", Name: "Parent", SortOrder: 1},
		},
		childCount: map[uuid.UUID]int{parent: 1, leaf: 0},
		props: map[uuid.UUID][]domaincatalog.Property{
			leaf: {
				{ID: uuid.New(), CategoryID: leaf, Code: "b", Title: "B", DataType: "STRING", SortOrder: 2, Options: json.RawMessage(`[]`)},
				{ID: uuid.New(), CategoryID: leaf, Code: "a", Title: "A", DataType: "STRING", SortOrder: 1, Options: json.RawMessage(`[]`)},
			},
		},
	}
	svc := appcatalog.NewService(repo)

	def, err := svc.GetCategoryFormDefinition(context.Background(), leaf)
	if err != nil {
		t.Fatal(err)
	}
	if def.Category.Slug != "leaf" || len(def.Properties) != 2 {
		t.Fatalf("def=%+v", def)
	}
	if def.Properties[0].Code != "a" || def.Properties[1].Code != "b" {
		t.Fatalf("order=%v %v", def.Properties[0].Code, def.Properties[1].Code)
	}

	_, err = svc.GetCategoryFormDefinition(context.Background(), parent)
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeInvalidState {
		t.Fatalf("want invalid state, got %v", err)
	}

	_, err = svc.GetCategoryFormDefinition(context.Background(), uuid.New())
	ae, ok = apperr.As(err)
	if !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestGetCategoryFormDefinitionInheritance(t *testing.T) {
	rootID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	parentID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	childID := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	repo := &fakeCatalogRepo{
		categories: []domaincatalog.Category{
			{ID: rootID, Slug: "root", Name: "Root", IsActive: true, SortOrder: 1},
			{ID: parentID, ParentID: &rootID, Slug: "parent", Name: "Parent", IsActive: true, SortOrder: 1},
			{ID: childID, ParentID: &parentID, Slug: "child", Name: "Child", IsActive: true, SortOrder: 1},
		},
		childCount: map[uuid.UUID]int{rootID: 1, parentID: 1, childID: 0},
		props: map[uuid.UUID][]domaincatalog.Property{
			rootID: {
				{ID: uuid.New(), CategoryID: rootID, Code: "rootProp", Title: "Root Prop", DataType: "STRING", SortOrder: 1},
				{ID: uuid.New(), CategoryID: rootID, Code: "sharedCode", Title: "Root Shared", DataType: "STRING", SortOrder: 2},
			},
			parentID: {
				{ID: uuid.New(), CategoryID: parentID, Code: "parentProp", Title: "Parent Prop", DataType: "BOOLEAN", SortOrder: 3},
			},
			childID: {
				{ID: uuid.New(), CategoryID: childID, Code: "childProp", Title: "Child Prop", DataType: "INTEGER", SortOrder: 4},
				{ID: uuid.New(), CategoryID: childID, Code: "sharedCode", Title: "Child Shared Override", DataType: "STRING", SortOrder: 5},
			},
		},
	}
	svc := appcatalog.NewService(repo)

	def, err := svc.GetCategoryFormDefinition(context.Background(), childID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should resolve 4 properties: rootProp, parentProp, childProp, sharedCode (child override)
	if len(def.Properties) != 4 {
		t.Fatalf("expected 4 properties, got %d: %+v", len(def.Properties), def.Properties)
	}

	codes := make([]string, len(def.Properties))
	for i, p := range def.Properties {
		codes[i] = p.Code
	}

	// Check that sharedCode is overridden by child (Title == "Child Shared Override")
	for _, p := range def.Properties {
		if p.Code == "sharedCode" && p.Title != "Child Shared Override" {
			t.Fatalf("expected child override for sharedCode, got title %q", p.Title)
		}
	}
}

func TestGetPublicCategoryTreeOrphanAndCycleSafe(t *testing.T) {
	root := uuid.MustParse("00000000-0000-0000-0000-000000000021")
	orphanParent := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	orphan := uuid.MustParse("00000000-0000-0000-0000-000000000022")
	child := uuid.MustParse("00000000-0000-0000-0000-000000000023")

	repo := &fakeCatalogRepo{categories: []domaincatalog.Category{
		{ID: root, Slug: "root", Name: "Root", SortOrder: 1},
		{ID: orphan, ParentID: &orphanParent, Slug: "orphan", Name: "Orphan", SortOrder: 1},
		{ID: child, ParentID: &root, Slug: "child", Name: "Child", SortOrder: 1},
		// Corrupt duplicate of root pointing at child creates a reachable cycle.
		{ID: root, ParentID: &child, Slug: "root-cycle", Name: "RootCycle", SortOrder: 1},
	}}
	svc := appcatalog.NewService(repo)

	done := make(chan struct{})
	var tree []appcatalog.TreeNode
	var err error
	go func() {
		tree, err = svc.GetPublicCategoryTree(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("category tree build hung; likely cycle recursion")
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 1 || tree[0].Slug != "root" {
		t.Fatalf("tree=%+v", tree)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].Slug != "child" {
		t.Fatalf("children=%+v", tree[0].Children)
	}
	if len(tree[0].Children[0].Children) != 0 {
		t.Fatalf("cycle edge must be skipped, got %+v", tree[0].Children[0].Children)
	}
}

func TestPropertyTieBreakByCodeThenID(t *testing.T) {
	leaf := uuid.MustParse("00000000-0000-0000-0000-000000000030")
	idLow := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	idHigh := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	repo := &fakeCatalogRepo{
		categories: []domaincatalog.Category{{ID: leaf, Slug: "leaf", Name: "Leaf"}},
		childCount: map[uuid.UUID]int{leaf: 0},
		props: map[uuid.UUID][]domaincatalog.Property{
			leaf: {
				{ID: idHigh, CategoryID: leaf, Code: "same", Title: "High", DataType: "STRING", SortOrder: 1, Options: json.RawMessage(`[]`)},
				{ID: idLow, CategoryID: leaf, Code: "same", Title: "Low", DataType: "STRING", SortOrder: 1, Options: json.RawMessage(`[]`)},
			},
		},
	}
	def, err := appcatalog.NewService(repo).GetCategoryFormDefinition(context.Background(), leaf)
	if err != nil {
		t.Fatal(err)
	}
	if def.Properties[0].ID != idLow || def.Properties[1].ID != idHigh {
		t.Fatalf("tie-break failed: %+v", def.Properties)
	}
}

func TestCreateCategoryAutoGeneratesUniqueSlugAndSortOrder(t *testing.T) {
	rootID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := &fakeCatalogRepo{
		categories: []domaincatalog.Category{
			{ID: rootID, Slug: "satilik-atlar", Name: "Satılık Atlar", SortOrder: 1, IsActive: true},
			{ID: uuid.New(), ParentID: &rootID, Slug: "satilik-yaris-ati", Name: "Satılık Yarış Atı", SortOrder: 1, IsActive: true},
		},
	}
	svc := appcatalog.NewService(repo)

	// 1. Create subcategory with same slug: should automatically resolve collision with -2
	child1, err := svc.CreateCategory(context.Background(), domaincatalog.Category{
		ParentID: &rootID,
		Name:     "Satılık Yarış Atı",
		Slug:     "satilik-yaris-ati",
	}, nil)
	if err != nil {
		t.Fatalf("failed to create subcategory with duplicate slug: %v", err)
	}
	if child1.Slug != "satilik-yaris-ati-2" {
		t.Fatalf("expected slug satilik-yaris-ati-2, got %s", child1.Slug)
	}
	if child1.SortOrder != 2 {
		t.Fatalf("expected auto sortOrder 2, got %d", child1.SortOrder)
	}

	// 2. Create subcategory with Turkish name and empty slug: should slugify correctly
	child2, err := svc.CreateCategory(context.Background(), domaincatalog.Category{
		ParentID: &rootID,
		Name:     "İngiliz & Arap Aygırı",
	}, nil)
	if err != nil {
		t.Fatalf("failed to create subcategory with Turkish name: %v", err)
	}
	if child2.Slug != "ingiliz-arap-aygiri" {
		t.Fatalf("expected slug ingiliz-arap-aygiri, got %s", child2.Slug)
	}
	if child2.SortOrder != 3 {
		t.Fatalf("expected auto sortOrder 3, got %d", child2.SortOrder)
	}
}
