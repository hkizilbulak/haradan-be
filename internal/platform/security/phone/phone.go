// Package phone normalizes and validates Turkey-focused optional mobile numbers.
package phone

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
)

var (
	nonDigit = regexp.MustCompile(`\D+`)
	trMobile = regexp.MustCompile(`^5\d{9}$`)
)

// NormalizeOptional returns canonical +905XXXXXXXXX, or nil when input is blank.
// Invalid non-empty input returns a field validation error.
func NormalizeOptional(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	return NormalizeOptionalString(*raw)
}

// NormalizeOptionalString trims and normalizes a phone string.
func NormalizeOptionalString(raw string) (*string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	canonical, err := normalizeTRMobile(trimmed)
	if err != nil {
		return nil, err
	}
	return &canonical, nil
}

func normalizeTRMobile(raw string) (string, error) {
	compact := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '-' || r == '(' || r == ')' {
			return -1
		}
		return r
	}, raw)
	digits := nonDigit.ReplaceAllString(compact, "")
	switch {
	case strings.HasPrefix(digits, "90") && len(digits) == 12:
		digits = digits[2:]
	case strings.HasPrefix(digits, "0") && len(digits) == 11:
		digits = digits[1:]
	}
	if !trMobile.MatchString(digits) {
		return "", apperr.Validation("Geçersiz istek.", apperr.FieldError{
			Field:   "phone",
			Message: "Geçerli bir Türkiye cep telefonu girin (örn. 532 123 45 67).",
		})
	}
	return "+90" + digits, nil
}

// FormatDisplayTR formats canonical +905XXXXXXXXX as "532 123 45 67" (no leading 0).
func FormatDisplayTR(canonical string) string {
	digits := nonDigit.ReplaceAllString(canonical, "")
	if strings.HasPrefix(digits, "90") && len(digits) == 12 {
		digits = digits[2:]
	} else if strings.HasPrefix(digits, "0") && len(digits) == 11 {
		digits = digits[1:]
	}
	if len(digits) != 10 || digits[0] != '5' {
		return canonical
	}
	return digits[0:3] + " " + digits[3:6] + " " + digits[6:8] + " " + digits[8:10]
}
