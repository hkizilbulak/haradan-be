package catalog

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
)

// TreeNode is an active category tree node for public responses.
type TreeNode struct {
	ID       uuid.UUID
	Slug     string
	Name     string
	Children []TreeNode
}

// FormDefinition is the public category form payload.
type FormDefinition struct {
	Category   domaincatalog.FormCategory
	Properties []domaincatalog.Property
}

// Service implements Catalog use cases.
type Service struct {
	repo domaincatalog.Repository
}

// adminRepository extends the public read model without forcing public-only
// test doubles to implement management commands.
type adminRepository interface {
	ListCategoriesAdmin(context.Context, *bool, int) ([]domaincatalog.Category, error)
	GetCategoryAdmin(context.Context, uuid.UUID) (domaincatalog.Category, error)
	CreateCategory(context.Context, domaincatalog.Category) (domaincatalog.Category, error)
	UpdateCategory(context.Context, uuid.UUID, domaincatalog.CategoryPatch, int, time.Time) (domaincatalog.Category, error)
	SetCategoryActive(context.Context, uuid.UUID, bool, int, time.Time) (domaincatalog.Category, error)
	ReparentCategory(context.Context, uuid.UUID, *uuid.UUID, int, time.Time) (domaincatalog.Category, error)
	IsDescendant(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	ReorderCategories(context.Context, []domaincatalog.ReorderItem, time.Time) error
	ListPropertiesAdmin(context.Context, uuid.UUID) ([]domaincatalog.Property, error)
	CreateProperty(context.Context, domaincatalog.Property, time.Time) (domaincatalog.Property, error)
	UpdateProperty(context.Context, uuid.UUID, uuid.UUID, domaincatalog.PropertyPatch, int, time.Time) (domaincatalog.Property, error)
	SetPropertyActive(context.Context, uuid.UUID, uuid.UUID, bool, int, time.Time) (domaincatalog.Property, error)
	ReorderProperties(context.Context, []domaincatalog.ReorderItem, time.Time) error
}

// NewService constructs a Catalog application service.
func NewService(repo domaincatalog.Repository) *Service {
	return &Service{repo: repo}
}

// GetPublicCategoryTree returns the active category forest.
func (s *Service) GetPublicCategoryTree(ctx context.Context) ([]TreeNode, error) {
	cats, err := s.repo.ListActiveCategories(ctx)
	if err != nil {
		return nil, apperr.WrapInternal(err)
	}
	return buildTree(cats), nil
}

// GetCategoryFormDefinition returns form metadata for an active leaf category.
func (s *Service) GetCategoryFormDefinition(ctx context.Context, categoryID uuid.UUID) (FormDefinition, error) {
	cat, err := s.repo.GetActiveCategory(ctx, categoryID)
	if err != nil {
		return FormDefinition{}, mapRepoErr(err)
	}

	childCount, err := s.repo.CountActiveChildren(ctx, cat.ID)
	if err != nil {
		return FormDefinition{}, apperr.WrapInternal(err)
	}
	if childCount > 0 {
		return FormDefinition{}, apperr.InvalidState("Form tanımı yalnız yaprak kategoriler için kullanılabilir.")
	}

	props, err := s.repo.ListFormProperties(ctx, cat.ID)
	if err != nil {
		return FormDefinition{}, apperr.WrapInternal(err)
	}
	if props == nil {
		props = []domaincatalog.Property{}
	}
	sortProperties(props)

	return FormDefinition{
		Category: domaincatalog.FormCategory{
			ID:   cat.ID,
			Slug: cat.Slug,
			Name: cat.Name,
		},
		Properties: props,
	}, nil
}

func buildTree(cats []domaincatalog.Category) []TreeNode {
	activeIDs := make(map[uuid.UUID]struct{}, len(cats))
	for _, c := range cats {
		activeIDs[c.ID] = struct{}{}
	}

	byParent := map[uuid.UUID][]domaincatalog.Category{}
	var roots []domaincatalog.Category
	for _, c := range cats {
		if c.ParentID == nil {
			roots = append(roots, c)
			continue
		}
		// Drop orphans whose parent is missing/inactive so they neither leak nor
		// get promoted to invented roots.
		if _, ok := activeIDs[*c.ParentID]; !ok {
			continue
		}
		byParent[*c.ParentID] = append(byParent[*c.ParentID], c)
	}

	sortCategories(roots)
	nodes := make([]TreeNode, 0, len(roots))
	for _, root := range roots {
		nodes = append(nodes, toNode(root, byParent, nil))
	}
	return nodes
}

func toNode(cat domaincatalog.Category, byParent map[uuid.UUID][]domaincatalog.Category, ancestors map[uuid.UUID]struct{}) TreeNode {
	nextAncestors := make(map[uuid.UUID]struct{}, len(ancestors)+1)
	for id := range ancestors {
		nextAncestors[id] = struct{}{}
	}
	nextAncestors[cat.ID] = struct{}{}

	children := byParent[cat.ID]
	sortCategories(children)
	out := TreeNode{
		ID:       cat.ID,
		Slug:     cat.Slug,
		Name:     cat.Name,
		Children: make([]TreeNode, 0, len(children)),
	}
	for _, child := range children {
		if _, cycle := nextAncestors[child.ID]; cycle {
			continue
		}
		out.Children = append(out.Children, toNode(child, byParent, nextAncestors))
	}
	return out
}

func sortCategories(cats []domaincatalog.Category) {
	sort.SliceStable(cats, func(i, j int) bool {
		if cats[i].SortOrder != cats[j].SortOrder {
			return cats[i].SortOrder < cats[j].SortOrder
		}
		if cats[i].Name != cats[j].Name {
			return cats[i].Name < cats[j].Name
		}
		return cats[i].ID.String() < cats[j].ID.String()
	})
}

func sortProperties(props []domaincatalog.Property) {
	sort.SliceStable(props, func(i, j int) bool {
		if props[i].SortOrder != props[j].SortOrder {
			return props[i].SortOrder < props[j].SortOrder
		}
		if props[i].Code != props[j].Code {
			return props[i].Code < props[j].Code
		}
		return props[i].ID.String() < props[j].ID.String()
	})
}

func mapRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if e, ok := apperr.As(err); ok {
		return e
	}
	return apperr.WrapInternal(err)
}

func (s *Service) admin() (adminRepository, error) {
	r, ok := s.repo.(adminRepository)
	if !ok {
		return nil, apperr.Internal(nil)
	}
	return r, nil
}

func (s *Service) ListCategoriesAdmin(ctx context.Context, active *bool, limit int) ([]domaincatalog.Category, error) {
	r, err := s.admin()
	if err != nil {
		return nil, err
	}
	return r.ListCategoriesAdmin(ctx, active, limit)
}
func (s *Service) GetCategoryAdminDetail(ctx context.Context, id uuid.UUID) (domaincatalog.Category, error) {
	r, err := s.admin()
	if err != nil {
		return domaincatalog.Category{}, err
	}
	return r.GetCategoryAdmin(ctx, id)
}
func (s *Service) CreateCategory(ctx context.Context, c domaincatalog.Category) (domaincatalog.Category, error) {
	r, err := s.admin()
	if err != nil {
		return c, err
	}
	if c.ParentID != nil {
		if _, err := r.GetCategoryAdmin(ctx, *c.ParentID); err != nil {
			return c, err
		}
	}
	c.ID = uuid.New()
	c.CreatedAt = time.Now().UTC()
	c.Version = 1
	return r.CreateCategory(ctx, c)
}
func (s *Service) UpdateCategory(ctx context.Context, id uuid.UUID, p domaincatalog.CategoryPatch, expected int) (domaincatalog.Category, error) {
	if expected < 1 {
		return domaincatalog.Category{}, apperr.Validation("expectedVersion geçersiz.")
	}
	r, err := s.admin()
	if err != nil {
		return domaincatalog.Category{}, err
	}
	return r.UpdateCategory(ctx, id, p, expected, time.Now().UTC())
}
func (s *Service) SetCategoryActive(ctx context.Context, id uuid.UUID, active bool, expected int) (domaincatalog.Category, error) {
	if expected < 1 {
		return domaincatalog.Category{}, apperr.Validation("expectedVersion geçersiz.")
	}
	r, err := s.admin()
	if err != nil {
		return domaincatalog.Category{}, err
	}
	return r.SetCategoryActive(ctx, id, active, expected, time.Now().UTC())
}
func (s *Service) ReparentCategory(ctx context.Context, id uuid.UUID, parent *uuid.UUID, expected int) (domaincatalog.Category, error) {
	if expected < 1 {
		return domaincatalog.Category{}, apperr.Validation("expectedVersion geçersiz.")
	}
	r, err := s.admin()
	if err != nil {
		return domaincatalog.Category{}, err
	}
	if parent != nil {
		if *parent == id {
			return domaincatalog.Category{}, apperr.Validation("Kategori kendisinin üst kategorisi olamaz.")
		}
		if _, err := r.GetCategoryAdmin(ctx, *parent); err != nil {
			return domaincatalog.Category{}, err
		}
		descendant, err := r.IsDescendant(ctx, id, *parent)
		if err != nil {
			return domaincatalog.Category{}, apperr.WrapInternal(err)
		}
		if descendant {
			return domaincatalog.Category{}, apperr.Validation("Kategori kendi alt kategorisine taşınamaz.")
		}
	}
	return r.ReparentCategory(ctx, id, parent, expected, time.Now().UTC())
}
func (s *Service) ReorderCategories(ctx context.Context, items []domaincatalog.ReorderItem) error {
	r, err := s.admin()
	if err != nil {
		return err
	}
	if err := validateReorder(items); err != nil {
		return err
	}
	return r.ReorderCategories(ctx, items, time.Now().UTC())
}
func (s *Service) ListCategoryPropertiesAdmin(ctx context.Context, categoryID uuid.UUID) ([]domaincatalog.Property, error) {
	r, err := s.admin()
	if err != nil {
		return nil, err
	}
	if _, err := r.GetCategoryAdmin(ctx, categoryID); err != nil {
		return nil, err
	}
	return r.ListPropertiesAdmin(ctx, categoryID)
}
func (s *Service) CreateCategoryProperty(ctx context.Context, p domaincatalog.Property) (domaincatalog.Property, error) {
	r, err := s.admin()
	if err != nil {
		return p, err
	}
	if _, err := r.GetCategoryAdmin(ctx, p.CategoryID); err != nil {
		return p, err
	}
	if !validDataType(p.DataType) {
		return p, apperr.Validation("Geçersiz özellik türü.")
	}
	p.ID = uuid.New()
	return r.CreateProperty(ctx, p, time.Now().UTC())
}
func (s *Service) UpdateCategoryProperty(ctx context.Context, cid, pid uuid.UUID, p domaincatalog.PropertyPatch, expected int) (domaincatalog.Property, error) {
	if expected < 1 {
		return domaincatalog.Property{}, apperr.Validation("expectedVersion geçersiz.")
	}
	r, err := s.admin()
	if err != nil {
		return domaincatalog.Property{}, err
	}
	return r.UpdateProperty(ctx, pid, cid, p, expected, time.Now().UTC())
}
func (s *Service) SetCategoryPropertyActive(ctx context.Context, cid, pid uuid.UUID, active bool, expected int) (domaincatalog.Property, error) {
	if expected < 1 {
		return domaincatalog.Property{}, apperr.Validation("expectedVersion geçersiz.")
	}
	r, err := s.admin()
	if err != nil {
		return domaincatalog.Property{}, err
	}
	return r.SetPropertyActive(ctx, pid, cid, active, expected, time.Now().UTC())
}
func (s *Service) ReorderCategoryProperties(ctx context.Context, cid uuid.UUID, items []domaincatalog.ReorderItem) error {
	r, err := s.admin()
	if err != nil {
		return err
	}
	if _, err := r.GetCategoryAdmin(ctx, cid); err != nil {
		return err
	}
	if err := validateReorder(items); err != nil {
		return err
	}
	properties, err := r.ListPropertiesAdmin(ctx, cid)
	if err != nil {
		return err
	}
	owned := make(map[uuid.UUID]struct{}, len(properties))
	for _, property := range properties {
		owned[property.ID] = struct{}{}
	}
	for _, item := range items {
		if _, ok := owned[item.ID]; !ok {
			return apperr.Validation("Özellik kategoriye ait değil.")
		}
	}
	return r.ReorderProperties(ctx, items, time.Now().UTC())
}
func validateReorder(items []domaincatalog.ReorderItem) error {
	seen := map[uuid.UUID]bool{}
	for _, i := range items {
		if i.ID == uuid.Nil || i.ExpectedVersion < 1 || i.SortOrder < 0 || seen[i.ID] {
			return apperr.Validation("Sıralama öğeleri geçersiz.")
		}
		seen[i.ID] = true
	}
	return nil
}
func validDataType(t string) bool {
	for _, v := range []string{"STRING", "TEXT", "INTEGER", "DECIMAL", "BOOLEAN", "SINGLE_SELECT", "YEAR"} {
		if t == v {
			return true
		}
	}
	return false
}
func jsonOr(raw json.RawMessage, fallback string) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(fallback)
	}
	return raw
}
