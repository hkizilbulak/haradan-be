package catalog_test

import (
	"testing"

	domaincatalog "github.com/hkizilbulak/haradan-be/internal/domain/catalog"
)

func TestGeneratePropertyCodeBase(t *testing.T) {
	cases := map[string]string{
		"Motor Gücü":     "MOTOR_GUCU",
		"Renk":           "RENK",
		"  Model Yılı ":  "MODEL_YILI",
		"":               "PROPERTY",
		"123 Start":      "P_123_START",
		"İstanbul Şehir": "ISTANBUL_SEHIR",
	}
	for in, want := range cases {
		if got := domaincatalog.GeneratePropertyCodeBase(in); got != want {
			t.Fatalf("GeneratePropertyCodeBase(%q)=%q, want %q", in, got, want)
		}
	}
}
