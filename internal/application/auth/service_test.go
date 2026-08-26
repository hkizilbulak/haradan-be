package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
	"github.com/hkizilbulak/haradan-be/internal/platform/security/token"
)

func newTestSvc(t *testing.T) (*Service, *memStore, *token.FixedClock) {
	t.Helper()
	return NewMemoryServiceForTest(t)
}

func TestRegisterSuccess(t *testing.T) {
	svc, store, _ := newTestSvc(t)
	out, err := svc.Register(context.Background(), RegisterInput{
		Email: "User@Example.com", Password: "Password1", FirstName: "Ada", LastName: "Lovelace",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Message != registerSuccessMessage || len(store.users) != 1 || len(store.otc) != 0 {
		t.Fatalf("out=%+v users=%d otc=%d", out, len(store.users), len(store.otc))
	}
	for _, u := range store.users {
		if u.EmailNormalized != "user@example.com" || strings.Contains(u.PasswordHash, "Password1") || u.EmailVerifiedAt == nil {
			t.Fatalf("user=%+v", u)
		}
	}
}

func TestRegisterDuplicateEmailEnumerationSafe(t *testing.T) {
	svc, _, _ := newTestSvc(t)
	in := RegisterInput{Email: "a@example.com", Password: "Password1", FirstName: "A", LastName: "B"}
	if _, err := svc.Register(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(context.Background(), in); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterValidation(t *testing.T) {
	svc, _, _ := newTestSvc(t)
	_, err := svc.Register(context.Background(), RegisterInput{Email: "bad", Password: "short", FirstName: "", LastName: ""})
	ae, ok := apperr.As(err)
	if !ok || ae.Kind != apperr.KindValidation {
		t.Fatalf("err=%v", err)
	}
}

func TestRegisterRepositoryFailure(t *testing.T) {
	svc, store, _ := newTestSvc(t)
	store.failCreate = true
	_, err := svc.Register(context.Background(), RegisterInput{
		Email: "x@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	if _, ok := apperr.As(err); !ok {
		t.Fatalf("err=%v", err)
	}
}

func TestLoginSuccessAndFailures(t *testing.T) {
	svc, store, _ := newTestSvc(t)
	if _, err := svc.Register(context.Background(), RegisterInput{
		Email: "login@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	}); err != nil {
		t.Fatal(err)
	}
	tok, err := svc.Login(context.Background(), LoginInput{
		Email: "login@example.com", Password: "Password1", ClientContext: domainauth.ClientContextPublicWeb,
	})
	if err != nil || tok.AccessToken == "" || tok.RefreshToken == "" {
		t.Fatalf("tok=%+v err=%v", tok, err)
	}

	_, err = svc.Login(context.Background(), LoginInput{
		Email: "missing@example.com", Password: "Password1", ClientContext: domainauth.ClientContextPublicWeb,
	})
	ae, _ := apperr.As(err)
	if ae == nil || ae.Code != apperr.CodeUnauthenticated {
		t.Fatalf("err=%v", err)
	}

	_, err = svc.Login(context.Background(), LoginInput{
		Email: "login@example.com", Password: "WrongPass1", ClientContext: domainauth.ClientContextPublicWeb,
	})
	ae, _ = apperr.As(err)
	if ae == nil || ae.Code != apperr.CodeUnauthenticated {
		t.Fatalf("err=%v", err)
	}

	for id, u := range store.users {
		u.Status = domainuser.StatusDisabled
		store.users[id] = u
	}
	_, err = svc.Login(context.Background(), LoginInput{
		Email: "login@example.com", Password: "Password1", ClientContext: domainauth.ClientContextPublicWeb,
	})
	ae, _ = apperr.As(err)
	if ae == nil || ae.Code != apperr.CodeAccountInactive {
		t.Fatalf("err=%v", err)
	}
}

func TestLoginBOContextRejected(t *testing.T) {
	svc, _, _ := newTestSvc(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "user@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	_, err := svc.Login(context.Background(), LoginInput{
		Email: "user@example.com", Password: "Password1", ClientContext: domainauth.ClientContextAdminBO,
	})
	ae, _ := apperr.As(err)
	if ae == nil || ae.Code != apperr.CodeForbidden {
		t.Fatalf("err=%v", err)
	}
}

func TestRefreshRotateAndReplay(t *testing.T) {
	svc, _, _ := newTestSvc(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "r@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	first, err := svc.Login(context.Background(), LoginInput{
		Email: "r@example.com", Password: "Password1", ClientContext: domainauth.ClientContextMobile,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Refresh(context.Background(), RefreshInput{
		RefreshToken: first.RefreshToken, ClientContext: domainauth.ClientContextMobile,
	})
	if err != nil || second.RefreshToken == first.RefreshToken {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	_, err = svc.Refresh(context.Background(), RefreshInput{
		RefreshToken: first.RefreshToken, ClientContext: domainauth.ClientContextMobile,
	})
	ae, _ := apperr.As(err)
	if ae == nil || ae.Code != apperr.CodeRefreshReplayDetected {
		t.Fatalf("err=%v", err)
	}
	// Family revoke must invalidate the rotated successor token too.
	_, err = svc.Refresh(context.Background(), RefreshInput{
		RefreshToken: second.RefreshToken, ClientContext: domainauth.ClientContextMobile,
	})
	ae, _ = apperr.As(err)
	if ae == nil || ae.Kind != apperr.KindUnauthenticated {
		t.Fatalf("successor after family revoke err=%v", err)
	}
}

func TestRefreshReplayDoesNotAffectOtherFamily(t *testing.T) {
	svc, _, _ := newTestSvc(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "fam@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	web, err := svc.Login(context.Background(), LoginInput{
		Email: "fam@example.com", Password: "Password1", ClientContext: domainauth.ClientContextPublicWeb,
	})
	if err != nil {
		t.Fatal(err)
	}
	mobile, err := svc.Login(context.Background(), LoginInput{
		Email: "fam@example.com", Password: "Password1", ClientContext: domainauth.ClientContextMobile,
	})
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := svc.Refresh(context.Background(), RefreshInput{
		RefreshToken: web.RefreshToken, ClientContext: domainauth.ClientContextPublicWeb,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Refresh(context.Background(), RefreshInput{
		RefreshToken: web.RefreshToken, ClientContext: domainauth.ClientContextPublicWeb,
	})
	if ae, _ := apperr.As(err); ae == nil || ae.Code != apperr.CodeRefreshReplayDetected {
		t.Fatalf("replay err=%v", err)
	}
	if _, err := svc.Refresh(context.Background(), RefreshInput{
		RefreshToken: mobile.RefreshToken, ClientContext: domainauth.ClientContextMobile,
	}); err != nil {
		t.Fatalf("other family should survive: %v", err)
	}
	_ = rotated
}

func TestRegisterEmailFailureLeavesUserWithoutResendInThisSlice(t *testing.T) {
	fail := EmailSenderFunc(func(context.Context, string, string, string) error {
		return errors.New("smtp down")
	})
	svc, store, _ := NewMemoryServiceForTestWithEmail(t, fail)
	_, err := svc.Register(context.Background(), RegisterInput{
		Email: "mailfail@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	ae, _ := apperr.As(err)
	if ae == nil || ae.Code != apperr.CodeDependencyUnavailable {
		t.Fatalf("err=%v", err)
	}
	if len(store.users) != 1 || len(store.otc) != 0 {
		t.Fatalf("committed user/otc users=%d otc=%d", len(store.users), len(store.otc))
	}
	if len(store.sessions) != 0 {
		t.Fatal("register must not create sessions")
	}
	for _, u := range store.users {
		if u.EmailVerifiedAt == nil {
			t.Fatal("expected user to be verified upon registration")
		}
	}
	// Retry is enumeration-safe success and does not create OTC.
	out, err := svc.Register(context.Background(), RegisterInput{
		Email: "mailfail@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Message != registerSuccessMessage || len(store.otc) != 0 {
		t.Fatalf("retry must not create new OTC; otc=%d out=%+v", len(store.otc), out)
	}
}

func TestDuplicateRegisterDoesNotLeakTokens(t *testing.T) {
	svc, store, _ := newTestSvc(t)
	in := RegisterInput{Email: "dup@example.com", Password: "Password1", FirstName: "A", LastName: "B"}
	first, err := svc.Register(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Register(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if first.Message != second.Message || second.Message == "" {
		t.Fatalf("first=%q second=%q", first.Message, second.Message)
	}
	if len(store.users) != 1 || len(store.sessions) != 0 {
		t.Fatalf("users=%d sessions=%d", len(store.users), len(store.sessions))
	}
}

func TestLogoutRejectsRefreshTokenAndKeepsOtherSession(t *testing.T) {
	svc, store, _ := newTestSvc(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "multi@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	a, err := svc.Login(context.Background(), LoginInput{
		Email: "multi@example.com", Password: "Password1", ClientContext: domainauth.ClientContextPublicWeb,
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Login(context.Background(), LoginInput{
		Email: "multi@example.com", Password: "Password1", ClientContext: domainauth.ClientContextMobile,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Logout(context.Background(), LogoutInput{AccessToken: a.RefreshToken}); err == nil {
		t.Fatal("refresh token must not logout")
	}
	if _, err := svc.Logout(context.Background(), LogoutInput{AccessToken: a.AccessToken}); err != nil {
		t.Fatal(err)
	}
	active := 0
	for _, s := range store.sessions {
		if s.RevokedAt == nil {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("active sessions=%d want 1", active)
	}
	if _, err := svc.Refresh(context.Background(), RefreshInput{
		RefreshToken: b.RefreshToken, ClientContext: domainauth.ClientContextMobile,
	}); err != nil {
		t.Fatalf("other session should remain usable: %v", err)
	}
}

func TestRefreshExpired(t *testing.T) {
	svc, store, clock := newTestSvc(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "e@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	tok, err := svc.Login(context.Background(), LoginInput{
		Email: "e@example.com", Password: "Password1", ClientContext: domainauth.ClientContextPublicWeb,
	})
	if err != nil {
		t.Fatal(err)
	}
	clock.T = clock.T.Add(48 * time.Hour)
	for id, s := range store.sessions {
		s.IdleExpiresAt = clock.T.Add(-time.Minute)
		store.sessions[id] = s
	}
	_, err = svc.Refresh(context.Background(), RefreshInput{
		RefreshToken: tok.RefreshToken, ClientContext: domainauth.ClientContextPublicWeb,
	})
	ae, _ := apperr.As(err)
	if ae == nil || ae.Code != apperr.CodeTokenInvalid {
		t.Fatalf("err=%v", err)
	}
}

func TestLogoutIdempotent(t *testing.T) {
	svc, _, _ := newTestSvc(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "lo@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	tok, err := svc.Login(context.Background(), LoginInput{
		Email: "lo@example.com", Password: "Password1", ClientContext: domainauth.ClientContextPublicWeb,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Logout(context.Background(), LogoutInput{AccessToken: tok.AccessToken}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Logout(context.Background(), LogoutInput{AccessToken: tok.AccessToken}); err != nil {
		t.Fatal(err)
	}
}

func TestLogoutInvalidToken(t *testing.T) {
	svc, _, _ := newTestSvc(t)
	_, err := svc.Logout(context.Background(), LogoutInput{AccessToken: "not-a-jwt"})
	ae, _ := apperr.As(err)
	if ae == nil || ae.Kind != apperr.KindUnauthenticated {
		t.Fatalf("err=%v", err)
	}
}

func TestLoginUnknownAndWrongPasswordSameExternalError(t *testing.T) {
	svc, _, _ := newTestSvc(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "same@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	_, errUnknown := svc.Login(context.Background(), LoginInput{
		Email: "nosuch@example.com", Password: "Password1", ClientContext: domainauth.ClientContextPublicWeb,
	})
	_, errWrong := svc.Login(context.Background(), LoginInput{
		Email: "same@example.com", Password: "WrongPass1", ClientContext: domainauth.ClientContextPublicWeb,
	})
	a, _ := apperr.As(errUnknown)
	b, _ := apperr.As(errWrong)
	if a == nil || b == nil || a.Code != b.Code || a.Message != b.Message || a.Kind != b.Kind {
		t.Fatalf("unknown=%+v wrong=%+v", a, b)
	}
}

func TestRefreshRejectsAccessJWT(t *testing.T) {
	svc, _, _ := newTestSvc(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "jwt@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	tok, err := svc.Login(context.Background(), LoginInput{
		Email: "jwt@example.com", Password: "Password1", ClientContext: domainauth.ClientContextPublicWeb,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Refresh(context.Background(), RefreshInput{
		RefreshToken: tok.AccessToken, ClientContext: domainauth.ClientContextPublicWeb,
	})
	ae, _ := apperr.As(err)
	if ae == nil || ae.Code != apperr.CodeTokenInvalid {
		t.Fatalf("err=%v", err)
	}
}

func TestConcurrentRefreshOnlyOneSucceeds(t *testing.T) {
	svc, _, _ := newTestSvc(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "race@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	tok, err := svc.Login(context.Background(), LoginInput{
		Email: "race@example.com", Password: "Password1", ClientContext: domainauth.ClientContextPublicWeb,
	})
	if err != nil {
		t.Fatal(err)
	}
	const n = 8
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			_, err := svc.Refresh(context.Background(), RefreshInput{
				RefreshToken: tok.RefreshToken, ClientContext: domainauth.ClientContextPublicWeb,
			})
			errs <- err
		}()
	}
	success := 0
	for i := 0; i < n; i++ {
		if <-errs == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("success=%d want 1", success)
	}
}

func TestRefreshInactiveUser(t *testing.T) {
	svc, store, _ := newTestSvc(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "in@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	tok, _ := svc.Login(context.Background(), LoginInput{
		Email: "in@example.com", Password: "Password1", ClientContext: domainauth.ClientContextPublicWeb,
	})
	for id, u := range store.users {
		u.Status = domainuser.StatusClosed
		store.users[id] = u
	}
	_, err := svc.Refresh(context.Background(), RefreshInput{
		RefreshToken: tok.RefreshToken, ClientContext: domainauth.ClientContextPublicWeb,
	})
	ae, _ := apperr.As(err)
	if ae == nil || ae.Code != apperr.CodeAccountInactive {
		t.Fatalf("err=%v", err)
	}
}

type recordingEmail struct {
	mu        sync.Mutex
	calls     int
	last      string
	lastToken string
	fail      bool
	failN     int // fail first N calls when >0
}

func (r *recordingEmail) SendWelcome(_ context.Context, toEmail, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.fail || (r.failN > 0 && r.calls <= r.failN) {
		return errors.New("smtp down")
	}
	r.last = toEmail
	return nil
}

func (r *recordingEmail) SendRegistrationVerification(_ context.Context, _, plaintextToken, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.fail || (r.failN > 0 && r.calls <= r.failN) {
		return errors.New("smtp down")
	}
	r.lastToken = plaintextToken
	return nil
}

func (r *recordingEmail) SendPasswordReset(ctx context.Context, toEmail, plaintextToken, fullName string) error {
	return r.SendRegistrationVerification(ctx, toEmail, plaintextToken, fullName)
}

func TestRegisterEmailFailureRecoveredByResendThenVerify(t *testing.T) {
	mail := &recordingEmail{failN: 1}
	svc, store, _ := NewMemoryServiceForTestWithEmail(t, mail)
	_, err := svc.Register(context.Background(), RegisterInput{
		Email: "recover@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	ae, _ := apperr.As(err)
	if ae == nil || ae.Code != apperr.CodeDependencyUnavailable {
		t.Fatalf("register err=%v", err)
	}
	if len(store.users) != 1 || len(store.otc) != 0 {
		t.Fatalf("users=%d otc=%d", len(store.users), len(store.otc))
	}

	for id, u := range store.users {
		u.EmailVerifiedAt = nil
		store.users[id] = u
	}

	out, err := svc.ResendVerification(context.Background(), ResendVerificationInput{Email: "recover@example.com"})
	if err != nil || out.Message == "" {
		t.Fatalf("resend err=%v out=%+v", err, out)
	}
	mail.mu.Lock()
	tokenPlain := mail.lastToken
	calls := mail.calls
	mail.mu.Unlock()
	if calls != 2 || tokenPlain == "" {
		t.Fatalf("calls=%d token empty=%v", calls, tokenPlain == "")
	}

	verify, err := svc.VerifyEmail(context.Background(), VerifyEmailInput{Token: tokenPlain})
	if err != nil || verify.Message == "" {
		t.Fatalf("verify err=%v", err)
	}
	for _, u := range store.users {
		if u.EmailVerifiedAt == nil {
			t.Fatal("expected verified")
		}
	}
	active := 0
	for _, c := range store.otc {
		if c.IsActive() {
			active++
		}
	}
	if active != 0 {
		t.Fatalf("active otc=%d", active)
	}
	// Already verified + consumed token → idempotent success.
	if _, err := svc.VerifyEmail(context.Background(), VerifyEmailInput{Token: tokenPlain}); err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
}

func TestResendEnumerationSafe(t *testing.T) {
	mail := &recordingEmail{}
	svc, store, _ := NewMemoryServiceForTestWithEmail(t, mail)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "pending@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	for id, u := range store.users {
		if u.EmailNormalized == "pending@example.com" {
			u.EmailVerifiedAt = nil
			store.users[id] = u
		}
	}
	// Mark verified user.
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "done@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	for id, u := range store.users {
		if u.EmailNormalized == "done@example.com" {
			now := time.Now().UTC()
			u.EmailVerifiedAt = &now
			store.users[id] = u
		}
	}
	mail.mu.Lock()
	mail.calls = 0
	mail.mu.Unlock()

	unknown, err := svc.ResendVerification(context.Background(), ResendVerificationInput{Email: "ghost@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := svc.ResendVerification(context.Background(), ResendVerificationInput{Email: "done@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	pending, err := svc.ResendVerification(context.Background(), ResendVerificationInput{Email: "pending@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if unknown.Message != verified.Message || verified.Message != pending.Message || pending.Message == "" {
		t.Fatalf("messages differ: %q %q %q", unknown.Message, verified.Message, pending.Message)
	}
	mail.mu.Lock()
	defer mail.mu.Unlock()
	if mail.calls != 1 {
		t.Fatalf("sender calls=%d want 1 (pending only)", mail.calls)
	}
}

func TestResendEmailFailureThenRetry(t *testing.T) {
	mail := &recordingEmail{}
	svc, store, _ := NewMemoryServiceForTestWithEmail(t, mail)
	_, err := svc.Register(context.Background(), RegisterInput{
		Email: "retry@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	if err != nil {
		t.Fatal(err)
	}
	for id, u := range store.users {
		u.EmailVerifiedAt = nil
		store.users[id] = u
	}
	mail.mu.Lock()
	mail.fail = true
	mail.mu.Unlock()
	_, err = svc.ResendVerification(context.Background(), ResendVerificationInput{Email: "retry@example.com"})
	ae, _ := apperr.As(err)
	if ae == nil || ae.Code != apperr.CodeDependencyUnavailable {
		t.Fatalf("err=%v", err)
	}
	mail.mu.Lock()
	mail.fail = false
	mail.mu.Unlock()
	out, err := svc.ResendVerification(context.Background(), ResendVerificationInput{Email: "retry@example.com"})
	if err != nil || out.Message == "" {
		t.Fatalf("retry err=%v", err)
	}
}

func TestVerifyEmailSuccessAndErrors(t *testing.T) {
	mail := &recordingEmail{}
	svc, store, clock := NewMemoryServiceForTestWithEmail(t, mail)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "v@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	for id, u := range store.users {
		u.EmailVerifiedAt = nil
		store.users[id] = u
	}
	_, _ = svc.ResendVerification(context.Background(), ResendVerificationInput{Email: "v@example.com"})
	mail.mu.Lock()
	tok := mail.lastToken
	mail.mu.Unlock()

	if _, err := svc.VerifyEmail(context.Background(), VerifyEmailInput{Token: "nope"}); err == nil {
		t.Fatal("invalid")
	} else if ae, _ := apperr.As(err); ae.Code != apperr.CodeTokenInvalid {
		t.Fatalf("code=%s", ae.Code)
	}

	clock.T = clock.T.Add(48 * time.Hour)
	if _, err := svc.VerifyEmail(context.Background(), VerifyEmailInput{Token: tok}); err == nil {
		t.Fatal("expired")
	} else if ae, _ := apperr.As(err); ae.Code != apperr.CodeTokenExpired {
		t.Fatalf("code=%s", ae.Code)
	}
	clock.T = clock.T.Add(-48 * time.Hour)

	if _, err := svc.VerifyEmail(context.Background(), VerifyEmailInput{Token: tok}); err != nil {
		t.Fatal(err)
	}
	for _, u := range store.users {
		if u.EmailVerifiedAt == nil {
			t.Fatal("not verified")
		}
	}
	// Replay after verified: idempotent success.
	if _, err := svc.VerifyEmail(context.Background(), VerifyEmailInput{Token: tok}); err != nil {
		t.Fatalf("idempotent: %v", err)
	}
}

func TestConcurrentVerifyOnlyOneConsumes(t *testing.T) {
	mail := &recordingEmail{}
	svc, store, _ := NewMemoryServiceForTestWithEmail(t, mail)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "race@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	for id, u := range store.users {
		u.EmailVerifiedAt = nil
		store.users[id] = u
	}
	_, _ = svc.ResendVerification(context.Background(), ResendVerificationInput{Email: "race@example.com"})
	mail.mu.Lock()
	tok := mail.lastToken
	mail.mu.Unlock()

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.VerifyEmail(context.Background(), VerifyEmailInput{Token: tok})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	ok := 0
	for err := range errs {
		if err == nil {
			ok++
		}
	}
	if ok < 1 {
		t.Fatal("expected at least one success")
	}
	consumed := 0
	for _, c := range store.otc {
		if c.ConsumedAt != nil {
			consumed++
		}
	}
	if consumed != 1 {
		t.Fatalf("consumed=%d", consumed)
	}
}

func TestConcurrentResendKeepsOneActive(t *testing.T) {
	mail := &recordingEmail{}
	svc, store, _ := NewMemoryServiceForTestWithEmail(t, mail)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "cresend@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	for id, u := range store.users {
		u.EmailVerifiedAt = nil
		store.users[id] = u
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.ResendVerification(context.Background(), ResendVerificationInput{Email: "cresend@example.com"})
		}()
	}
	wg.Wait()
	active := 0
	for _, c := range store.otc {
		if c.Purpose == domainauth.PurposeEmailVerification && c.IsActive() {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("active=%d want 1", active)
	}
}

func TestRequestEmailChangeDirectUpdate(t *testing.T) {
	mail := &recordingEmail{}
	svc, store, _ := NewMemoryServiceForTestWithEmail(t, mail)
	_, err := svc.Register(context.Background(), RegisterInput{
		Email: "change@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	if err != nil {
		t.Fatal(err)
	}
	var uid uuid.UUID
	for id := range store.users {
		uid = id
	}

	out, err := svc.RequestEmailChange(context.Background(), RequestEmailChangeInput{
		UserID:   uid,
		NewEmail: "new-address@example.com",
		ClientIP: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("RequestEmailChange failed: %v", err)
	}
	if out.Message != "E-posta adresi güncellendi." {
		t.Fatalf("expected message in out, got %+v", out)
	}
	u := store.users[uid]
	if u.Email != "new-address@example.com" || u.EmailVerifiedAt == nil {
		t.Fatalf("user email not directly updated or not verified: %+v", u)
	}
}
