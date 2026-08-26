package packaging_test

import (
	"testing"

	domainpackaging "github.com/hkizilbulak/haradan-be/internal/domain/packaging"
)

func TestGeneratePackageCodeBase(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Premium Plus":    "PREMIUM_PLUS",
		"Ücretsiz Deneme": "UCRETSIZ_DENEME",
		"  Avantajlı  ":   "AVANTAJLI",
	}
	for in, want := range cases {
		if got := domainpackaging.GeneratePackageCodeBase(in); got != want {
			t.Fatalf("%q => %q want %q", in, got, want)
		}
	}
}
