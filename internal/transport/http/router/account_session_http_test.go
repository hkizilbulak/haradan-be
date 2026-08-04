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
	"testing"

	"github.com/google/uuid"

	appauth "github.com/hkizilbulak/haradan-be/internal/application/auth"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/handler"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/router"
)

func newAccountEngine(t *testing.T) (*appauth.Service, func(method, path, body, auth string) *httptest.ResponseRecorder) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, _, _ := appauth.NewMemoryServiceForTest(t)
	engine := router.New(handler.NewServer(log, fakeDeps{}, nil, nil, nil, nil, svc), log, router.Options{AuthService: svc})
	do := func(method, path, body, auth string) *httptest.ResponseRecorder {
		var rdr io.Reader
		if body != "" {
			rdr = bytes.NewReader([]byte(body))
		}
		req := httptest.NewRequest(method, path, rdr)
		if body != "" || method == http.MethodPatch || method == http.MethodPost {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("X-Request-ID", "acct-http-1")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}
	return svc, do
}

func TestAccountSessionHappyPathHTTP(t *testing.T) {
	_, do := newAccountEngine(t)
	rec := do(http.MethodPost, "/api/v1/auth/register", `{"email":"acct@example.com","password":"Password1","firstName":"Ada","lastName":"Lovelace"}`, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("register=%d %s", rec.Code, rec.Body.String())
	}
	rec = do(http.MethodPost, "/api/v1/auth/login", `{"email":"acct@example.com","password":"Password1","clientContext":"PUBLIC_WEB"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login=%d", rec.Code)
	}
	var tokens generated.AuthTokenResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &tokens)
	auth := "Bearer " + tokens.AccessToken

	rec = do(http.MethodGet, "/api/v1/me", "", auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile=%d %s", rec.Code, rec.Body.String())
	}
	var profile generated.MyProfileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &profile); err != nil || profile.FirstName != "Ada" {
		t.Fatalf("%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "password") || strings.Contains(rec.Body.String(), "security") {
		t.Fatal("secret leak")
	}

	rec = do(http.MethodPatch, "/api/v1/me", `{"firstName":"Augusta","phone":null}`, auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch=%d %s", rec.Code, rec.Body.String())
	}

	rec = do(http.MethodGet, "/api/v1/me/sessions", "", auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("sessions=%d %s", rec.Code, rec.Body.String())
	}
	var sessions generated.SessionListResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &sessions)
	if len(sessions.Items) < 1 || !sessions.Items[0].IsCurrent {
		t.Fatalf("%+v", sessions)
	}

	rec = do(http.MethodPost, "/api/v1/auth/login", `{"email":"acct@example.com","password":"Password1","clientContext":"MOBILE"}`, "")
	var other generated.AuthTokenResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &other)
	rec = do(http.MethodGet, "/api/v1/me/sessions", "", auth)
	_ = json.Unmarshal(rec.Body.Bytes(), &sessions)
	var otherID string
	for _, item := range sessions.Items {
		if !item.IsCurrent {
			otherID = item.Id.String()
			break
		}
	}
	if otherID == "" {
		t.Fatal("missing other session")
	}
	rec = do(http.MethodDelete, "/api/v1/me/sessions/"+otherID, "", auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke=%d %s", rec.Code, rec.Body.String())
	}

	rec = do(http.MethodPost, "/api/v1/auth/logout-all", "", auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout-all=%d %s", rec.Code, rec.Body.String())
	}
	rec = do(http.MethodGet, "/api/v1/me", "", auth)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("after logout-all status=%d", rec.Code)
	}
}

func TestAccountAuthRequiredAndPublicUnaffected(t *testing.T) {
	_, do := newAccountEngine(t)
	rec := do(http.MethodGet, "/api/v1/me", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("me=%d", rec.Code)
	}
	var body generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.TraceId != "acct-http-1" || body.Code != generated.DomainErrorCodeUNAUTHENTICATED {
		t.Fatalf("%+v", body)
	}

	rec = do(http.MethodGet, "/api/health", "", "")
	if rec.Code == http.StatusUnauthorized {
		t.Fatal("health must stay public")
	}

	rec = do(http.MethodGet, "/api/v1/me/favorites", "", "")
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("favorites still out of scope want 501 got %d", rec.Code)
	}
}

func TestAccountOpsNoLonger501(t *testing.T) {
	_, do := newAccountEngine(t)
	_ = do(http.MethodPost, "/api/v1/auth/register", `{"email":"n501@example.com","password":"Password1","firstName":"A","lastName":"B"}`, "")
	rec := do(http.MethodPost, "/api/v1/auth/login", `{"email":"n501@example.com","password":"Password1","clientContext":"PUBLIC_WEB"}`, "")
	var tokens generated.AuthTokenResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &tokens)
	auth := "Bearer " + tokens.AccessToken
	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodGet, "/api/v1/me"},
		{http.MethodPatch, "/api/v1/me"},
		{http.MethodGet, "/api/v1/me/sessions"},
		{http.MethodPost, "/api/v1/auth/logout-all"},
	} {
		body := ""
		if tc.method == http.MethodPatch {
			body = `{}`
		}
		rec := do(tc.method, tc.path, body, auth)
		if rec.Code == http.StatusNotImplemented {
			t.Fatalf("%s %s still 501", tc.method, tc.path)
		}
	}
}

func TestUpdateMyProfileMalformed400(t *testing.T) {
	_, do := newAccountEngine(t)
	_ = do(http.MethodPost, "/api/v1/auth/register", `{"email":"badbody@example.com","password":"Password1","firstName":"A","lastName":"B"}`, "")
	rec := do(http.MethodPost, "/api/v1/auth/login", `{"email":"badbody@example.com","password":"Password1","clientContext":"PUBLIC_WEB"}`, "")
	var tokens generated.AuthTokenResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &tokens)
	rec = do(http.MethodPatch, "/api/v1/me", `{"firstName":`, "Bearer "+tokens.AccessToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSelectiveMiddlewareBoundariesHTTP(t *testing.T) {
	_, do := newAccountEngine(t)
	_ = do(http.MethodPost, "/api/v1/auth/register", `{"email":"bound@example.com","password":"Password1","firstName":"A","lastName":"B"}`, "")
	rec := do(http.MethodPost, "/api/v1/auth/login", `{"email":"bound@example.com","password":"Password1","clientContext":"PUBLIC_WEB"}`, "")
	var tokens generated.AuthTokenResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &tokens)
	auth := "Bearer " + tokens.AccessToken

	// Out-of-scope FE_AUTH stub must stay 501 even with a valid access token.
	rec = do(http.MethodGet, "/api/v1/me/favorites", "", auth)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("favorites with token=%d", rec.Code)
	}

	// Refresh token must not authenticate protected account routes.
	rec = do(http.MethodGet, "/api/v1/me", "", "Bearer "+tokens.RefreshToken)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("refresh bearer status=%d", rec.Code)
	}

	// Wrong method on protected path must not hit auth as GetMyProfile.
	rec = do(http.MethodPost, "/api/v1/me", `{}`, "")
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("POST /me should not be selectively authed as GET/PATCH; got %d", rec.Code)
	}
}

func TestCrossUserSessionRevokeHTTP(t *testing.T) {
	_, do := newAccountEngine(t)
	_ = do(http.MethodPost, "/api/v1/auth/register", `{"email":"ua@example.com","password":"Password1","firstName":"A","lastName":"B"}`, "")
	_ = do(http.MethodPost, "/api/v1/auth/register", `{"email":"ub@example.com","password":"Password1","firstName":"C","lastName":"D"}`, "")
	rec := do(http.MethodPost, "/api/v1/auth/login", `{"email":"ua@example.com","password":"Password1","clientContext":"PUBLIC_WEB"}`, "")
	var a generated.AuthTokenResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &a)
	rec = do(http.MethodPost, "/api/v1/auth/login", `{"email":"ub@example.com","password":"Password1","clientContext":"PUBLIC_WEB"}`, "")
	var b generated.AuthTokenResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &b)

	rec = do(http.MethodGet, "/api/v1/me/sessions", "", "Bearer "+b.AccessToken)
	var sessions generated.SessionListResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &sessions)
	if len(sessions.Items) != 1 {
		t.Fatalf("%+v", sessions)
	}
	victimSession := sessions.Items[0].Id.String()

	rec = do(http.MethodDelete, "/api/v1/me/sessions/"+victimSession, "", "Bearer "+a.AccessToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-user revoke status=%d body=%s", rec.Code, rec.Body.String())
	}
	var errBody generated.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &errBody)
	if errBody.Code != generated.DomainErrorCodeNOTFOUND {
		t.Fatalf("%+v", errBody)
	}

	rec = do(http.MethodGet, "/api/v1/me", "", "Bearer "+b.AccessToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("victim session must remain active status=%d", rec.Code)
	}
}

func TestEmptySessionsJSONArrayHTTP(t *testing.T) {
	svc, do := newAccountEngine(t)
	_ = do(http.MethodPost, "/api/v1/auth/register", `{"email":"arr@example.com","password":"Password1","firstName":"A","lastName":"B"}`, "")
	rec := do(http.MethodPost, "/api/v1/auth/login", `{"email":"arr@example.com","password":"Password1","clientContext":"PUBLIC_WEB"}`, "")
	var tokens generated.AuthTokenResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &tokens)
	p, err := svc.AuthenticateAccessToken(context.Background(), tokens.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.ListMySessions(context.Background(), p.UserID, uuid.Nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Items == nil {
		t.Fatal("items nil")
	}
	rec = do(http.MethodGet, "/api/v1/me/sessions", "", "Bearer "+tokens.AccessToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"items":[`) {
		t.Fatalf("items must be JSON array: %s", rec.Body.String())
	}
}
