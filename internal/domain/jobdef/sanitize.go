package jobdef

import (
	"strings"
	"unicode/utf8"
)

const (
	maxLastErrorRunes     = 500
	sanitizedErrorMessage = "İş hata ayrıntısı gizlendi."
)

// SanitizeLastError returns a BO-safe last_error projection: secret-like
// content is replaced, length is capped, and surrounding whitespace is trimmed.
func SanitizeLastError(raw *string) *string {
	if raw == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil
	}
	lower := strings.ToLower(trimmed)
	if containsSecretLike(lower) {
		msg := sanitizedErrorMessage
		return &msg
	}
	if utf8.RuneCountInString(trimmed) > maxLastErrorRunes {
		runes := []rune(trimmed)
		trimmed = string(runes[:maxLastErrorRunes]) + "..."
	}
	return &trimmed
}

func containsSecretLike(lower string) bool {
	needles := []string{
		"password",
		"passwd",
		"secret",
		"api_key",
		"apikey",
		"access_key",
		"secret_key",
		"authorization",
		"bearer ",
		"postgres://",
		"postgresql://",
		"mysql://",
		"mongodb://",
		"aws_secret",
		"private_key",
		"-----begin",
	}
	for _, n := range needles {
		if strings.Contains(lower, n) {
			return true
		}
	}
	return false
}
