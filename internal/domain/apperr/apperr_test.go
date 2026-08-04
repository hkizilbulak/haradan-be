package apperr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
)

func TestRateLimited(t *testing.T) {
	err := apperr.RateLimited("Çok fazla deneme yaptınız.")
	if err.Kind != apperr.KindRateLimited {
		t.Fatalf("kind=%v", err.Kind)
	}
	if err.Code != apperr.CodeRateLimited {
		t.Fatalf("code=%s", err.Code)
	}
	if apperr.CodeRateLimited != "RATE_LIMITED" {
		t.Fatalf("code value=%s", apperr.CodeRateLimited)
	}
	if err.Error() != "Çok fazla deneme yaptınız." {
		t.Fatalf("message=%q", err.Error())
	}
}

// TestKindsAreDistinct guards the iota block: a duplicated kind would silently
// reroute a whole error class to the wrong HTTP status.
func TestKindsAreDistinct(t *testing.T) {
	kinds := map[apperr.Kind]string{
		apperr.KindValidation:            "validation",
		apperr.KindNotFound:              "notFound",
		apperr.KindConflict:              "conflict",
		apperr.KindInternal:              "internal",
		apperr.KindUnauthenticated:       "unauthenticated",
		apperr.KindForbidden:             "forbidden",
		apperr.KindDependencyUnavailable: "dependencyUnavailable",
		apperr.KindBadRequest:            "badRequest",
		apperr.KindRateLimited:           "rateLimited",
	}
	if len(kinds) != 9 {
		t.Fatalf("two kinds share a value: %v", kinds)
	}
	var zero apperr.Kind
	if _, ok := kinds[zero]; ok {
		t.Fatal("no kind may take the zero value")
	}
}

func TestRateLimitedSurvivesWrapping(t *testing.T) {
	wrapped := fmt.Errorf("outer: %w", apperr.RateLimited("yavaşlayın"))
	ae, ok := apperr.As(wrapped)
	if !ok {
		t.Fatal("As must find the wrapped typed error")
	}
	if ae.Code != apperr.CodeRateLimited {
		t.Fatalf("code=%s", ae.Code)
	}
	// A typed error must not be re-wrapped as internal.
	if got := apperr.WrapInternal(wrapped); !errors.Is(got, wrapped) {
		t.Fatalf("WrapInternal must pass a typed error through, got %v", got)
	}
}
