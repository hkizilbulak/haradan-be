package phone_test

import (
	"testing"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	"github.com/hkizilbulak/haradan-be/internal/platform/security/phone"
)

func TestNormalizeOptionalAccepted(t *testing.T) {
	t.Parallel()
	cases := []string{
		"05321234567",
		"0532 123 45 67",
		"+905321234567",
		"+90 532 123 45 67",
		"5321234567",
	}
	for _, raw := range cases {
		got, err := phone.NormalizeOptionalString(raw)
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if got == nil || *got != "+905321234567" {
			t.Fatalf("%q => %#v", raw, got)
		}
	}
}

func TestNormalizeOptionalBlank(t *testing.T) {
	t.Parallel()
	empty := ""
	got, err := phone.NormalizeOptional(&empty)
	if err != nil || got != nil {
		t.Fatalf("blank => %#v err=%v", got, err)
	}
	got, err = phone.NormalizeOptional(nil)
	if err != nil || got != nil {
		t.Fatalf("nil => %#v err=%v", got, err)
	}
}

func TestNormalizeOptionalRejectsGarbage(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"1", "11111111111111111111", "abc", "+90123", "02121234567"} {
		_, err := phone.NormalizeOptionalString(raw)
		if err == nil {
			t.Fatalf("expected validation for %q", raw)
		}
		ae, ok := apperr.As(err)
		if !ok || ae.Kind != apperr.KindValidation {
			t.Fatalf("%q => %#v", raw, err)
		}
	}
}

func TestFormatDisplayTR(t *testing.T) {
	t.Parallel()
	if got := phone.FormatDisplayTR("+905321234567"); got != "532 123 45 67" {
		t.Fatalf("got %q", got)
	}
}
