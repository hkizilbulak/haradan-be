package catalog

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var nonPropertyCodeChars = regexp.MustCompile(`[^A-Z0-9]+`)

// GeneratePropertyCodeBase builds a stable uppercase code stem from a property title.
// Example: "Motor Gücü" → "MOTOR_GUCU", "Renk" → "RENK".
func GeneratePropertyCodeBase(title string) string {
	s := strings.TrimSpace(title)
	if s == "" {
		return "PROPERTY"
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
	s = nonPropertyCodeChars.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	if s == "" {
		s = "PROPERTY"
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
