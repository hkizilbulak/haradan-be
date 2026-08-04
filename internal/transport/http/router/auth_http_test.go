package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/router"
)

func newAuthEngine(t *testing.T) (*httptest.ResponseRecorder, func(method, path, body, auth string) *httptest.ResponseRecorder) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, _, _ := appauth.NewMemoryServiceForTest(t)
	engine := router.New(handler.NewServer(log, fakeDeps{}, nil, nil, nil, svc), log)
	do := func(method, path, body, auth string) *httptest.ResponseRecorder {
		var rdr io.Reader
		if body != "" {
			rdr = bytes.NewReader([]byte(body))
		}
		req := httptest.NewRequest(method, path, rdr)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Request-ID", "auth-http-1")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}
	return nil, do
}

func TestAuthRegisterLoginRefreshLogoutHTTP(t *testing.T) {
	_, do := newAuthEngine(t)

	rec := do(http.MethodPost, "/api/v1/auth/register", `{"email":"http@example.com","password":"Password1","firstName":"A","lastName":"B"}`, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatal("content-type")
	}
	var msg generated.GenericAuthMessageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &msg); err != nil || msg.Message == "" {
		t.Fatalf("body=%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Password1") || strings.Contains(strings.ToLower(rec.Body.String()), "password_hash") {
		t.Fatal("secret leaked")
	}

	rec = do(http.MethodPost, "/api/v1/auth/login", `{"email":"http@example.com","password":"Password1","clientContext":"PUBLIC_WEB"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", rec.Code, rec.Body.String())
	}
	var tokens generated.AuthTokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &tokens); err != nil {
		t.Fatal(err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" || tokens.TokenType != generated.Bearer {
		t.Fatalf("%+v", tokens)
	}

	rec = do(http.MethodPost, "/api/v1/auth/refresh", `{"refreshToken":"`+tokens.RefreshToken+`","clientContext":"PUBLIC_WEB"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh status=%d body=%s", rec.Code, rec.Body.String())
	}
	var refreshed generated.AuthTokenResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &refreshed)

	rec = do(http.MethodPost, "/api/v1/auth/logout", "", "Bearer "+refreshed.AccessToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthMalformedJSON400(t *testing.T) {
	_, do := newAuthEngine(t)
	for _, path := range []string{
		"/api/v1/auth/register", "/api/v1/auth/login", "/api/v1/auth/refresh",
		"/api/v1/auth/verify-email", "/api/v1/auth/resend-verification",
	} {
		rec := do(http.MethodPost, path, `{"email":`, "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d", path, rec.Code)
		}
		var body generated.ErrorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Code != generated.DomainErrorCodeVALIDATIONERROR || body.TraceId != "auth-http-1" {
			t.Fatalf("%+v", body)
		}
		if strings.Contains(rec.Body.String(), "unexpected") || strings.Contains(rec.Body.String(), "offset") {
			t.Fatalf("parser leak: %s", rec.Body.String())
		}
	}
}

func TestAuthValidation422(t *testing.T) {
	_, do := newAuthEngine(t)
	rec := do(http.MethodPost, "/api/v1/auth/register", `{"email":"ok@example.com","password":"short","firstName":"A","lastName":"B"}`, "")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthLoginUnauthorized(t *testing.T) {
	_, do := newAuthEngine(t)
	_ = do(http.MethodPost, "/api/v1/auth/register", `{"email":"u@example.com","password":"Password1","firstName":"A","lastName":"B"}`, "")
	rec := do(http.MethodPost, "/api/v1/auth/login", `{"email":"u@example.com","password":"WrongPass1","clientContext":"PUBLIC_WEB"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
	var body generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Code != generated.DomainErrorCodeUNAUTHENTICATED {
		t.Fatalf("%+v", body)
	}
}

func TestAuthLogoutUnauthenticated(t *testing.T) {
	_, do := newAuthEngine(t)
	rec := do(http.MethodPost, "/api/v1/auth/logout", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAuthEmptyBody400(t *testing.T) {
	_, do := newAuthEngine(t)
	for _, path := range []string{
		"/api/v1/auth/register", "/api/v1/auth/login", "/api/v1/auth/refresh",
		"/api/v1/auth/verify-email", "/api/v1/auth/resend-verification",
	} {
		rec := do(http.MethodPost, path, "", "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
		var body generated.ErrorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Code != generated.DomainErrorCodeVALIDATIONERROR || body.TraceId != "auth-http-1" {
			t.Fatalf("%+v", body)
		}
	}
}

func TestAuthRefreshRejectsAccessToken(t *testing.T) {
	_, do := newAuthEngine(t)
	_ = do(http.MethodPost, "/api/v1/auth/register", `{"email":"ra@example.com","password":"Password1","firstName":"A","lastName":"B"}`, "")
	rec := do(http.MethodPost, "/api/v1/auth/login", `{"email":"ra@example.com","password":"Password1","clientContext":"PUBLIC_WEB"}`, "")
	var tokens generated.AuthTokenResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &tokens)
	rec = do(http.MethodPost, "/api/v1/auth/refresh", `{"refreshToken":"`+tokens.AccessToken+`","clientContext":"PUBLIC_WEB"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthOpsNoLonger501(t *testing.T) {
	_, do := newAuthEngine(t)
	paths := []string{
		"/api/v1/auth/register",
		"/api/v1/auth/login",
		"/api/v1/auth/refresh",
		"/api/v1/auth/logout",
		"/api/v1/auth/verify-email",
		"/api/v1/auth/resend-verification",
	}
	for _, path := range paths {
		rec := do(http.MethodPost, path, `{}`, "")
		if rec.Code == http.StatusNotImplemented {
			t.Fatalf("%s still 501", path)
		}
	}
}

func TestAuthVerifyAndResendHTTP(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	var lastToken string
	var mu sync.Mutex
	sender := appauth.EmailSenderFunc(func(_ context.Context, _, plaintext string) error {
		mu.Lock()
		lastToken = plaintext
		mu.Unlock()
		return nil
	})
	svc, _, _ := appauth.NewMemoryServiceForTestWithEmail(t, sender)
	engine := router.New(handler.NewServer(log, fakeDeps{}, nil, nil, nil, svc), log)
	do := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Request-ID", "auth-http-1")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}

	rec := do(http.MethodPost, "/api/v1/auth/register", `{"email":"verifyhttp@example.com","password":"Password1","firstName":"A","lastName":"B"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register=%d %s", rec.Code, rec.Body.String())
	}
	mu.Lock()
	tok := lastToken
	mu.Unlock()
	if tok == "" {
		t.Fatal("missing verification token")
	}

	unknown := do(http.MethodPost, "/api/v1/auth/resend-verification", `{"email":"nobody@example.com"}`)
	pending := do(http.MethodPost, "/api/v1/auth/resend-verification", `{"email":"verifyhttp@example.com"}`)
	if unknown.Code != http.StatusOK || pending.Code != http.StatusOK {
		t.Fatalf("resend unknown=%d pending=%d", unknown.Code, pending.Code)
	}
	var uMsg, pMsg generated.GenericAuthMessageResponse
	_ = json.Unmarshal(unknown.Body.Bytes(), &uMsg)
	_ = json.Unmarshal(pending.Body.Bytes(), &pMsg)
	if uMsg.Message == "" || uMsg.Message != pMsg.Message {
		t.Fatalf("enumeration mismatch %q vs %q", uMsg.Message, pMsg.Message)
	}

	mu.Lock()
	tok = lastToken
	mu.Unlock()
	rec = do(http.MethodPost, "/api/v1/auth/verify-email", `{"token":"`+tok+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("verify=%d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), tok) {
		t.Fatal("token leaked")
	}

	rec = do(http.MethodPost, "/api/v1/auth/verify-email", `{"token":"invalid-token-value"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid status=%d", rec.Code)
	}
	var errBody generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &errBody)
	if errBody.Code != generated.DomainErrorCodeTOKENINVALID || errBody.TraceId != "auth-http-1" {
		t.Fatalf("%+v", errBody)
	}
}
