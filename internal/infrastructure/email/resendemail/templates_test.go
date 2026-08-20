package resendemail

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendPasswordResetUsesSeparateTemplate(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/emails" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_reset"}`))
	}))
	defer srv.Close()

	sender, err := New(testConfig(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	client := srv.Client()
	client.Timeout = sender.client.http.(*http.Client).Timeout
	client.CheckRedirect = sender.client.http.(*http.Client).CheckRedirect
	sender.client.http = client

	if err := sender.SendPasswordReset(context.Background(), testRecipient, testToken, ""); err != nil {
		t.Fatalf("send: %v", err)
	}
	tmpl, ok := gotBody["template"].(map[string]any)
	if !ok {
		t.Fatalf("template=%v", gotBody["template"])
	}
	if tmpl["id"] != "haradan-reset-password" {
		t.Fatalf("template id=%v want haradan-reset-password", tmpl["id"])
	}
	vars, ok := tmpl["variables"].(map[string]any)
	if !ok {
		t.Fatalf("variables=%v", tmpl["variables"])
	}
	if vars["resetUrl"] != testFrontendURL+"/reset-password?token="+testToken {
		t.Fatalf("resetUrl=%v", vars["resetUrl"])
	}
	if tmpl["id"] == testTemplateID {
		t.Fatal("password reset must not reuse registration template")
	}
}

func TestListTemplatesAndGetTemplateVariables(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/templates":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":"tmpl_1","name":"welcome-email","alias":"welcome-email","status":"published"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/templates/tmpl_1":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"tmpl_1","html":"<p>Hello {{ fullName }} <a href=\"{{ verificationUrl }}\">x</a></p>","subject":"Hi {{ fullName }}"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	sender, err := New(testConfig(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	client := srv.Client()
	client.Timeout = sender.client.http.(*http.Client).Timeout
	client.CheckRedirect = sender.client.http.(*http.Client).CheckRedirect
	sender.client.http = client

	list, err := sender.ListTemplates(context.Background())
	if err != nil || len(list) != 1 || list[0].ID != "tmpl_1" {
		t.Fatalf("list=%v err=%v", list, err)
	}
	vars, err := sender.GetTemplateVariables(context.Background(), "tmpl_1")
	if err != nil {
		t.Fatalf("vars: %v", err)
	}
	seen := map[string]bool{}
	for _, v := range vars {
		seen[v] = true
	}
	if !seen["fullName"] || !seen["verificationUrl"] {
		t.Fatalf("vars=%v", vars)
	}
}
