package authn_test

import (
	"testing"

	"github.com/hkizilbulak/haradan-be/internal/transport/http/middleware/authn"
)

func TestExtractBearer(t *testing.T) {
	tok, ok := authn.ExtractBearer("Bearer abc.def")
	if !ok || tok != "abc.def" {
		t.Fatalf("tok=%q ok=%v", tok, ok)
	}
	if _, ok := authn.ExtractBearer("Basic x"); ok {
		t.Fatal("expected false")
	}
	if _, ok := authn.ExtractBearer("Bearer "); ok {
		t.Fatal("expected false")
	}
}
