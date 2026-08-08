package richhtml_test

import (
	"strings"
	"testing"

	"github.com/hkizilbulak/haradan-be/internal/platform/security/richhtml"
)

func TestSanitizePlainText(t *testing.T) {
	t.Parallel()
	in := "Bu paket ilanınızı öne çıkarır."
	got := richhtml.Sanitize(in)
	if got != in {
		t.Fatalf("plain text changed: %q", got)
	}
}

func TestSanitizeAllowedFormatting(t *testing.T) {
	t.Parallel()
	in := `<p>Merhaba <strong>kalın</strong> ve <em>italik</em></p><ul><li>bir</li></ul><ol><li>iki</li></ol><blockquote>alıntı</blockquote><a href="https://example.com" target="_blank">link</a>`
	got := richhtml.Sanitize(in)
	for _, want := range []string{"<p>", "<strong>", "<em>", "<ul>", "<ol>", "<li>", "<blockquote>", `href="https://example.com"`, `rel="noopener noreferrer"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestSanitizeRejectsXSS(t *testing.T) {
	t.Parallel()
	cases := []string{
		`<script>alert(1)</script>`,
		`<p><img src=x onerror="alert(1)"></p>`,
		`<a href="javascript:alert(1)">x</a>`,
		`<p onclick="alert(1)">click</p>`,
		`<iframe src="https://evil"></iframe>`,
		`<object data="x"></object>`,
		`<embed src="x">`,
		`<a href="data:text/html,hi">x</a>`,
	}
	for _, in := range cases {
		got := richhtml.Sanitize(in)
		lower := strings.ToLower(got)
		if strings.Contains(lower, "<script") ||
			strings.Contains(lower, "onerror") ||
			strings.Contains(lower, "onclick") ||
			strings.Contains(lower, "javascript:") ||
			strings.Contains(lower, "<iframe") ||
			strings.Contains(lower, "<object") ||
			strings.Contains(lower, "<embed") ||
			strings.Contains(lower, "data:") {
			t.Fatalf("unsafe content survived for %q → %q", in, got)
		}
	}
}

func TestSanitizeOptionalNil(t *testing.T) {
	t.Parallel()
	if richhtml.SanitizeOptional(nil) != nil {
		t.Fatal("expected nil")
	}
	empty := "   "
	if richhtml.SanitizeOptional(&empty) != nil {
		t.Fatal("expected nil for blank")
	}
}
