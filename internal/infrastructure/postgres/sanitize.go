package postgres

import (
	"fmt"
	"strings"
)

// SanitizeErr removes credential-like details from database errors.
func SanitizeErr(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "password") ||
		strings.Contains(msg, "postgres://") ||
		strings.Contains(msg, "postgresql://") ||
		strings.Contains(msg, "user=") ||
		strings.Contains(msg, "pwd=") {
		return fmt.Errorf("database error")
	}
	return err
}
