package textnorm_test

import (
	"testing"

	"github.com/hkizilbulak/haradan-be/internal/platform/textnorm"
)

func TestTurkishFold(t *testing.T) {
	cases := map[string]string{
		"İstanbul": "istanbul",
		"IĞDIR":    "ığdır",
		" Ank ":    "ank",
		"":         "",
	}
	for in, want := range cases {
		if got := textnorm.TurkishFold(in); got != want {
			t.Fatalf("TurkishFold(%q)=%q want %q", in, got, want)
		}
	}
}
