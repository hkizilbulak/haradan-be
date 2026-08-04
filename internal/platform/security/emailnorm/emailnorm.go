package emailnorm

import (
	"net/mail"
	"strings"
)

// Normalize trims and lowercases an email for unique login lookup.
func Normalize(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidFormat performs a lightweight RFC5322 address parse after trim.
func ValidFormat(email string) bool {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" || len(trimmed) > 320 {
		return false
	}
	addr, err := mail.ParseAddress(trimmed)
	if err != nil {
		return false
	}
	return addr.Address != "" && !strings.Contains(addr.Address, " ")
}
