package resendemail

import (
	"strings"
	"testing"
	"time"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	valid := Config{
		APIKey:                  "unit-test-api-key-not-real",
		BaseURL:                 "https://api.resend.com",
		HTTPTimeout:             time.Second,
		FromEmail:               "noreply@example.com",
		FromName:                "Haradan",
		FrontendURL:             "https://app.example.com",
		WelcomeTemplateID:       "welcome-email",
		ResetPasswordTemplateID: "reset-password",
	}
	if err := valid.validate(); err != nil {
		t.Fatalf("valid config: %v", err)
	}

	cases := []struct {
		name string
		mut  func(*Config)
		want string
	}{
		{name: "empty api key", mut: func(c *Config) { c.APIKey = "" }, want: "API key"},
		{name: "crlf api key", mut: func(c *Config) { c.APIKey = "key\nvalue" }, want: "CR or LF"},
		{name: "timeout zero", mut: func(c *Config) { c.HTTPTimeout = 0 }, want: "timeout"},
		{name: "http base", mut: func(c *Config) { c.BaseURL = "http://api.resend.com" }, want: "https"},
		{name: "userinfo base", mut: func(c *Config) { c.BaseURL = "https://user:pass@api.resend.com" }, want: "userinfo"},
		{name: "query base", mut: func(c *Config) { c.BaseURL = "https://api.resend.com?x=1" }, want: "query"},
		{name: "fragment base", mut: func(c *Config) { c.BaseURL = "https://api.resend.com#x" }, want: "fragment"},
		{name: "path base", mut: func(c *Config) { c.BaseURL = "https://api.resend.com/v1" }, want: "path"},
		{name: "empty from email", mut: func(c *Config) { c.FromEmail = "" }, want: "from email"},
		{name: "invalid from email", mut: func(c *Config) { c.FromEmail = "not-an-email" }, want: "from email"},
		{name: "display from email", mut: func(c *Config) { c.FromEmail = "Name <a@b.com>" }, want: "from email"},
		{name: "crlf from email", mut: func(c *Config) { c.FromEmail = "a\r@b.com" }, want: "CR or LF"},
		{name: "empty from name", mut: func(c *Config) { c.FromName = "  " }, want: "from name"},
		{name: "crlf from name", mut: func(c *Config) { c.FromName = "Bad\nName" }, want: "CR or LF"},
		{name: "empty frontend", mut: func(c *Config) { c.FrontendURL = "" }, want: "frontend"},
		{name: "bad frontend scheme", mut: func(c *Config) { c.FrontendURL = "ftp://app.example.com" }, want: "http or https"},
		{name: "frontend userinfo", mut: func(c *Config) { c.FrontendURL = "https://u:p@app.example.com" }, want: "userinfo"},
		{name: "empty welcome", mut: func(c *Config) {
			c.WelcomeTemplateID = ""
			c.TemplateID = ""
			c.ResetPasswordTemplateID = "reset-password"
		}, want: ""}, // defaults fill welcome-email
		{name: "same templates", mut: func(c *Config) {
			c.WelcomeTemplateID = "same"
			c.ResetPasswordTemplateID = "same"
		}, want: "differ"},
		{name: "crlf welcome", mut: func(c *Config) { c.WelcomeTemplateID = "tmpl\nid" }, want: "CR or LF"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := valid
			tc.mut(&cfg)
			err := cfg.validate()
			if tc.name == "empty welcome" {
				if err != nil {
					t.Fatalf("defaults should fill welcome: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if strings.Contains(err.Error(), "unit-test-api-key") {
				t.Fatalf("error leaked api key: %v", err)
			}
			if tc.want != "" && !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.want)) &&
				!strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v want substring %q", err, tc.want)
			}
		})
	}
}
