package catalog

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
)

func TestValidateReorderRejectsInvalidAndDuplicateItems(t *testing.T) {
	id := uuid.New()
	for _, items := range [][]domaincatalog.ReorderItem{
		{{ID: uuid.Nil, ExpectedVersion: 1, SortOrder: 0}},
		{{ID: id, ExpectedVersion: 0, SortOrder: 0}},
		{{ID: id, ExpectedVersion: 1, SortOrder: -1}},
		{{ID: id, ExpectedVersion: 1, SortOrder: 0}, {ID: id, ExpectedVersion: 2, SortOrder: 1}},
	} {
		err := validateReorder(items)
		var appErr *apperr.Error
		if err == nil || !errors.As(err, &appErr) || appErr.Code != apperr.CodeValidation {
			t.Fatalf("expected validation error, got %v", err)
		}
	}
}

func TestValidDataType(t *testing.T) {
	if !validDataType("SINGLE_SELECT") || validDataType("ENUM") {
		t.Fatal("property data-type validation mismatch")
	}
}
