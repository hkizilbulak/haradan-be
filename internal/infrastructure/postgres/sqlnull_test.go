package postgres_test

import (
	"testing"

	"github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres"
)

// Reproduce CHECK hrd_one_time_credentials_target_email_by_purpose_check:
// PASSWORD_RESET requires target_email IS NULL (not '').
func TestNullIfEmptyPasswordResetOTCSemantics(t *testing.T) {
	t.Parallel()
	if got := postgres.NullIfEmpty(""); got != nil {
		t.Fatalf("PASSWORD_RESET empty target must be SQL NULL, got %q", *got)
	}
	if got := postgres.NullIfEmpty("user@example.com"); got == nil || *got != "user@example.com" {
		t.Fatalf("EMAIL_CHANGE target must stay non-null, got %#v", got)
	}
}
