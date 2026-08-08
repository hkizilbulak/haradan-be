package packaging

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var nonCodeChars = regexp.MustCompile(`[^A-Z0-9]+`)

// GeneratePackageCodeBase builds a stable uppercase code stem from a display name.
// Example: "Premium Plus" → "PREMIUM_PLUS", "Ücretsiz Deneme" → "UCRETSIZ_DENEME".
func GeneratePackageCodeBase(displayName string) string {
	s := strings.TrimSpace(displayName)
	if s == "" {
		return "PACKAGE"
	}
	replacer := strings.NewReplacer(
		"İ", "I", "I\u0307", "I", "ı", "I",
		"Ş", "S", "ş", "S",
		"Ğ", "G", "ğ", "G",
		"Ü", "U", "ü", "U",
		"Ö", "O", "ö", "O",
		"Ç", "C", "ç", "C",
	)
	s = replacer.Replace(s)
	s = strings.ToUpper(s)
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	s, _, _ = transform.String(t, s)
	s = nonCodeChars.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	if s == "" {
		s = "PACKAGE"
	}
	if len(s) > 64 {
		s = s[:64]
		s = strings.TrimRight(s, "_")
	}
	if s[0] >= '0' && s[0] <= '9' {
		s = "P_" + s
		if len(s) > 64 {
			s = s[:64]
		}
	}
	return s
}
