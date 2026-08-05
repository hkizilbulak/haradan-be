package resendemail

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
)

const (
	testAPIKey      = "unit-test-api-key-not-real"
	testToken       = "unit-test-plaintext-token-not-real"
	testRecipient   = "user@example.com"
	testTemplateID  = "tmpl_registration_verify"
	testFrontendURL = "https://app.example.com"
	testFromEmail   = "noreply@example.com"
	testFromName    = "Haradan"
)

func testConfig(baseURL string) Config {
	return Config{
		APIKey:      testAPIKey,
		BaseURL:     baseURL,
		HTTPTimeout: 5 * time.Second,
		FromEmail:   testFromEmail,
		FromName:    testFromName,
		FrontendURL: testFrontendURL,
		TemplateID:  testTemplateID,
	}
}

func TestNewValidatesWithoutNetwork(t *testing.T) {
	t.Parallel()

	_, err := New(Config{
		APIKey:      "",
		BaseURL:     "https://api.example.invalid",
		HTTPTimeout: time.Second,
		FromEmail:   testFromEmail,
		FromName:    testFromName,
		FrontendURL: testFrontendURL,
		TemplateID:  testTemplateID,
	})
	if err == nil {
		t.Fatal("expected missing key error")
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Fatalf("error leaked key: %v", err)
	}

	_, err = New(Config{
		APIKey:      testAPIKey,
		BaseURL:     "http://api.example.invalid",
		HTTPTimeout: time.Second,
		FromEmail:   testFromEmail,
		FromName:    testFromName,
		FrontendURL: testFrontendURL,
		TemplateID:  testTemplateID,
	})
	if err == nil {
		t.Fatal("expected http base URL error")
	}
}

func TestSendRegistrationVerificationSuccessContract(t *testing.T) {
	t.Parallel()

	var gotAuth string
	var gotBody map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/emails" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Header.Get("Accept") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_test"}`))
	}))
	defer srv.Close()

	sender, err := New(testConfig(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	// Preserve production CheckRedirect/timeout while trusting the test TLS cert.
	client := srv.Client()
	client.Timeout = sender.client.http.(*http.Client).Timeout
	client.CheckRedirect = sender.client.http.(*http.Client).CheckRedirect
	sender.client.http = client

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := sender.SendRegistrationVerification(ctx, "  "+testRecipient+"  ", testToken); err != nil {
		t.Fatalf("send: %v", err)
	}

	if gotAuth != "Bearer "+testAPIKey {
		t.Fatalf("authorization mismatch")
	}
	wantFrom := (&mail.Address{Name: testFromName, Address: testFromEmail}).String()
	if gotBody["from"] != wantFrom {
		t.Fatalf("from=%v want %q", gotBody["from"], wantFrom)
	}
	to, ok := gotBody["to"].([]any)
	if !ok || len(to) != 1 || to[0] != testRecipient {
		t.Fatalf("to=%v", gotBody["to"])
	}
	tmpl, ok := gotBody["template"].(map[string]any)
	if !ok {
		t.Fatalf("template=%v", gotBody["template"])
	}
	if tmpl["id"] != testTemplateID {
		t.Fatalf("template id=%v", tmpl["id"])
	}
	vars, ok := tmpl["variables"].(map[string]any)
	if !ok {
		t.Fatalf("variables=%v", tmpl["variables"])
	}
	if vars["verificationToken"] != testToken {
		t.Fatalf("verificationToken mismatch")
	}
	if vars["recipientEmail"] != testRecipient {
		t.Fatalf("recipientEmail=%v", vars["recipientEmail"])
	}
	if vars["frontendUrl"] != testFrontendURL {
		t.Fatalf("frontendUrl=%v", vars["frontendUrl"])
	}
}

func TestSendRejectsInvalidRecipientsAndToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("provider must not be called")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender, err := New(testConfig(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	sender.client.http = srv.Client()

	cases := []struct {
		name  string
		email string
		token string
	}{
		{name: "empty recipient", email: "  ", token: testToken},
		{name: "undefined recipient", email: "undefined", token: testToken},
		{name: "null recipient", email: "null", token: testToken},
		{name: "crlf recipient", email: "user@exam\nple.com", token: testToken},
		{name: "display name", email: "User <user@example.com>", token: testToken},
		{name: "comma separated", email: "a@example.com,b@example.com", token: testToken},
		{name: "empty token", email: testRecipient, token: "  "},
		{name: "crlf token", email: testRecipient, token: "tok\nen"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := sender.SendRegistrationVerification(context.Background(), tc.email, tc.token)
			ae, ok := apperr.As(err)
			if !ok || ae.Kind != apperr.KindValidation {
				t.Fatalf("err=%v", err)
			}
			assertNoSensitiveLeak(t, err)
		})
	}
}

func TestSendMapsProviderStatuses(t *testing.T) {
	t.Parallel()

	statuses := []int{
		http.StatusFound,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
	}
	for _, status := range statuses {
		status := status
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"message":"provider secret detail","request_id":"req_leak"}`))
			}))
			defer srv.Close()

			sender, err := New(testConfig(srv.URL))
			if err != nil {
				t.Fatal(err)
			}
			sender.client.http = srv.Client()

			err = sender.SendRegistrationVerification(context.Background(), testRecipient, testToken)
			ae, ok := apperr.As(err)
			if !ok || ae.Code != apperr.CodeDependencyUnavailable {
				t.Fatalf("err=%v", err)
			}
			assertNoSensitiveLeak(t, err)
			if strings.Contains(err.Error(), "provider secret") || strings.Contains(err.Error(), "req_leak") {
				t.Fatalf("provider body leaked: %v", err)
			}
		})
	}
}

func TestSendContextCanceledAndDeadline(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender, err := New(testConfig(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	sender.client.http = srv.Client()

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	err = sender.SendRegistrationVerification(canceled, testRecipient, testToken)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled err=%v", err)
	}

	deadline, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)
	err = sender.SendRegistrationVerification(deadline, testRecipient, testToken)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline err=%v", err)
	}
}

func TestSendNetworkError(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("simulated dial failure")
		}),
	}
	sender, err := newWithHTTPClient(testConfig("https://api.example.invalid"), client)
	if err != nil {
		t.Fatal(err)
	}
	err = sender.SendRegistrationVerification(context.Background(), testRecipient, testToken)
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeDependencyUnavailable {
		t.Fatalf("err=%v", err)
	}
	assertNoSensitiveLeak(t, err)
	if strings.Contains(err.Error(), "simulated dial") {
		t.Fatalf("raw network text leaked: %v", err)
	}
}

func TestSendAcceptsNonOK2xx(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	sender, err := New(testConfig(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	sender.client.http = srv.Client()
	if err := sender.SendRegistrationVerification(context.Background(), testRecipient, testToken); err != nil {
		t.Fatalf("202 should succeed: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestSendLargeErrorBodyLimited(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(strings.Repeat("x", 20<<10)))
	}))
	defer srv.Close()

	sender, err := New(testConfig(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	sender.client.http = srv.Client()
	err = sender.SendRegistrationVerification(context.Background(), testRecipient, testToken)
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeDependencyUnavailable {
		t.Fatalf("err=%v", err)
	}
}

func TestSendDoesNotFollowRedirectWithCredentials(t *testing.T) {
	t.Parallel()

	var secondHits atomic.Int32
	leakSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondHits.Add(1)
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("authorization followed redirect: present")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer leakSrv.Close()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", leakSrv.URL+"/emails")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	sender, err := New(testConfig(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	// Shared client rejects auto-follow via CheckRedirect; inject TLS transport only.
	client := srv.Client()
	client.CheckRedirect = sender.client.http.(*http.Client).CheckRedirect
	sender.client.http = client

	err = sender.SendRegistrationVerification(context.Background(), testRecipient, testToken)
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeDependencyUnavailable {
		t.Fatalf("err=%v", err)
	}
	if secondHits.Load() != 0 {
		t.Fatalf("redirect was followed")
	}
}

func TestSendReusesHTTPClient(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender, err := New(testConfig(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	client := srv.Client()
	sender.client.http = client

	for i := 0; i < 3; i++ {
		if err := sender.SendRegistrationVerification(context.Background(), testRecipient, testToken); err != nil {
			t.Fatal(err)
		}
	}
	if hits.Load() != 3 {
		t.Fatalf("hits=%d", hits.Load())
	}
	if sender.client.http != client {
		t.Fatal("http client was replaced between sends")
	}
}

func TestFormatFromHeaderQuotesSpecialNames(t *testing.T) {
	t.Parallel()

	got := formatFromHeader(`Evil <x@y.com>`, testFromEmail)
	want := (&mail.Address{Name: `Evil <x@y.com>`, Address: testFromEmail}).String()
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if strings.HasPrefix(got, "Evil <") {
		t.Fatalf("unquoted angle brackets in from display name: %q", got)
	}
}

func TestSenderImplementsEmailSender(t *testing.T) {
	t.Parallel()
	var _ appauth.EmailSender = (*Sender)(nil)
}

func TestSendTemplateEmailSetsIdempotencyKey(t *testing.T) {
	t.Parallel()
	var gotKey string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender, err := newWithHTTPClient(testConfig(srv.URL), srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("k", 256)
	if err := sender.SendTemplateEmail(context.Background(), testRecipient, "tmpl_notify", nil, map[string]string{"title": "Hi"}, key); err != nil {
		t.Fatal(err)
	}
	if gotKey != key {
		t.Fatalf("idempotency key mismatch")
	}
}

func TestSendTemplateEmailTreats409AsSuccess(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	sender, err := newWithHTTPClient(testConfig(srv.URL), srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.SendTemplateEmail(context.Background(), testRecipient, "tmpl_notify", nil, map[string]string{"title": "Hi"}, "dup-key"); err != nil {
		t.Fatalf("409 should be success: %v", err)
	}
}

func TestSendTemplateEmailRejectsInvalidKeyWithoutLeak(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("provider must not be called")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender, err := newWithHTTPClient(testConfig(srv.URL), srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	err = sender.SendTemplateEmail(context.Background(), testRecipient, "tmpl_notify", nil, map[string]string{"title": "Hi"}, "")
	if err == nil {
		t.Fatal("expected invalid key error")
	}
	assertNoSensitiveLeak(t, err)
}

func assertNoSensitiveLeak(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	msg := err.Error()
	for _, secret := range []string{
		testAPIKey,
		"Bearer ",
		testToken,
		testRecipient,
		testTemplateID,
		testFrontendURL,
		"api.resend.com",
	} {
		if strings.Contains(msg, secret) {
			t.Fatalf("error leaked %q: %v", secret, err)
		}
	}
}
