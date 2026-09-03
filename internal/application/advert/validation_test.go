package advert

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
)

func TestOptionAllowedTurkish(t *testing.T) {
	options := json.RawMessage(`[
		{"value": "Arap", "label": "Arap"},
		{"value": "İngiliz", "label": "İngiliz"}
	]`)

	valuesToTest := []string{
		"Arap",
		"İngiliz",
		"ingiliz",
		"İNGİLİZ",
		"INGILIZ",
		"Safkan Arap",
		"İngiliz (Thoroughbred)",
	}

	for _, v := range valuesToTest {
		allowed := optionAllowed(options, v)
		t.Logf("value %q allowed: %v", v, allowed)
	}
}

func TestOptionAllowedAge(t *testing.T) {
	options := json.RawMessage(`[
		{"value": "0", "label": "0"},
		{"value": "1", "label": "1"},
		{"value": "1.5", "label": "1.5"},
		{"value": "2", "label": "2"},
		{"value": "3", "label": "3"},
		{"value": "4", "label": "4"},
		{"value": "5", "label": "5"},
		{"value": "6", "label": "6"},
		{"value": "7", "label": "7"},
		{"value": "8", "label": "8"},
		{"value": "9", "label": "9"},
		{"value": "10-15 arası", "label": "10-15 arası"},
		{"value": "15 üzeri", "label": "15 üzeri"}
	]`)

	valuesToTest := []string{
		"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "15", "16", "20",
		"10-15 arası", "15 üzeri", "5+", "5+ Yaş", "Tay (0-1 Yaş)", "4 Yaş",
	}

	for _, v := range valuesToTest {
		allowed := optionAllowed(options, v)
		t.Logf("age %q allowed: %v", v, allowed)
	}
}

func TestValidateStudProperties(t *testing.T) {
	defs := []domaincatalog.Property{
		{
			ID:          uuid.New(),
			Code:        "STALLION_BREED",
			Title:       "At Irkı",
			DataType:    "SINGLE_SELECT",
			IsRequired:  true,
			Options:     json.RawMessage(`[{"value": "Arap", "label": "Arap"}, {"value": "İngiliz", "label": "İngiliz"}]`),
			IsActive:    true,
			IsFormVisible: true,
		},
		{
			ID:          uuid.New(),
			Code:        "STALLION_AGE",
			Title:       "Yaş",
			DataType:    "SINGLE_SELECT",
			IsRequired:  true,
			Options:     json.RawMessage(`[{"value": "0"}, {"value": "1"}, {"value": "1.5"}, {"value": "2"}, {"value": "3"}, {"value": "4"}, {"value": "5"}, {"value": "6"}, {"value": "7"}, {"value": "8"}, {"value": "9"}, {"value": "10-15 arası"}, {"value": "15 üzeri"}]`),
			IsActive:    true,
			IsFormVisible: true,
		},
		{
			ID:          uuid.New(),
			Code:        "COAT_COLOR",
			Title:       "Donu (Renk)",
			DataType:    "SINGLE_SELECT",
			IsRequired:  true,
			Options:     json.RawMessage(`[{"value": "Doru"}, {"value": "Al"}, {"value": "Kır"}, {"value": "Beyaz"}, {"value": "Yağız"}, {"value": "Kula"}, {"value": "Boz"}, {"value": "Kestane"}]`),
			IsActive:    true,
			IsFormVisible: true,
		},
		{
			ID:          uuid.New(),
			Code:        "studHorseName",
			Title:       "Aygır Adı",
			DataType:    "STRING",
			IsRequired:  false,
			IsActive:    true,
			IsFormVisible: true,
		},
	}

	payload := json.RawMessage(`{
		"COAT_COLOR": "Doru",
		"HORSE_BREED": "İngiliz (Thoroughbred)",
		"HORSE_AGE": "4",
		"HORSE_GENDER": "Erkek",
		"STALLION_BREED": "İngiliz",
		"studBreed": "İngiliz",
		"STALLION_AGE": "4",
		"studAge": 4,
		"studCoatColor": "Doru",
		"studHorseName": "BOLD BOY",
		"studSire": "NATIVE KHAN",
		"studDam": "LADY WONDER",
		"studDamsire": "UNACCOUNTED FOR",
		"TJK_NUMBER": "12345"
	}`)

	res, err := validateDynamicProperties(defs, payload, propertyModeSubmit)
	if err != nil {
		t.Fatalf("validation failed: %v", err)
	}
	t.Logf("validated: %s", string(res))
}
