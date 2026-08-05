package catalog

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Category is a catalog category node.
type Category struct {
	ID          uuid.UUID
	ParentID    *uuid.UUID
	Slug        string
	Name        string
	Description *string
	IsActive    bool
	SortOrder   int
	Version     int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Property is a category property definition exposed for forms.
type Property struct {
	ID              uuid.UUID
	CategoryID      uuid.UUID
	Code            string
	Title           string
	HelpText        *string
	DataType        string
	IsRequired      bool
	IsFilterable    bool
	SortOrder       int
	Options         json.RawMessage
	DefaultValue    json.RawMessage
	UIMetadata      json.RawMessage
	IsActive        bool
	IsFormVisible   bool
	IsPublicVisible bool
	Validation      json.RawMessage
	Version         int
}

// CategoryPatch is the mutable subset of a category.
type CategoryPatch struct {
	SlugSet, NameSet, DescriptionSet, SortOrderSet bool
	Slug, Name                                     string
	Description                                    *string
	SortOrder                                      int
}

// PropertyPatch is the mutable subset of a category property.
type PropertyPatch struct {
	TitleSet, HelpTextSet, IsRequiredSet, IsPublicVisibleSet, IsFormVisibleSet, IsFilterableSet, SortOrderSet, OptionsSet, ValidationSet, DefaultValueSet, UIMetadataSet bool
	Title                                                                                                                                                                string
	HelpText                                                                                                                                                             *string
	IsRequired, IsPublicVisible, IsFormVisible, IsFilterable                                                                                                             bool
	SortOrder                                                                                                                                                            int
	Options, Validation, DefaultValue, UIMetadata                                                                                                                        json.RawMessage
}

// ReorderItem applies one optimistic sort-order update.
type ReorderItem struct {
	ID              uuid.UUID
	ExpectedVersion int
	SortOrder       int
}

// FormCategory is the category header used by form definition responses.
type FormCategory struct {
	ID   uuid.UUID
	Slug string
	Name string
}

// Repository reads category tree and form metadata.
type Repository interface {
	ListActiveCategories(ctx context.Context) ([]Category, error)
	GetActiveCategory(ctx context.Context, id uuid.UUID) (Category, error)
	CountActiveChildren(ctx context.Context, parentID uuid.UUID) (int, error)
	ListFormProperties(ctx context.Context, categoryID uuid.UUID) ([]Property, error)
}
