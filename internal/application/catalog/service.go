package catalog

import (
	"context"
	"sort"

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
