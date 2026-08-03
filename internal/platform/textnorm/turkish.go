package textnorm

import (
	"strings"
	"unicode"
)

// TurkishFold lowercases text with Turkish I/İ rules for prefix search.
func TurkishFold(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.TrimSpace(s) {
		switch r {
		case 'I':
			b.WriteRune('ı')
		case 'İ':
			b.WriteRune('i')
		default:
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}
