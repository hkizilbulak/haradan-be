package auth

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

func TestGetAndUpdateMyProfile(t *testing.T) {
	svc, store, _ := NewMemoryServiceForTest(t)
	_, err := svc.Register(context.Background(), RegisterInput{
		Email: "me@example.com", Password: "Password1", FirstName: "Ada", LastName: "Lovelace",
	})
	if err != nil {
		t.Fatal(err)
	}
	var userID uuid.UUID
	for id := range store.users {
		userID = id
	}
	view, err := svc.GetMyProfile(context.Background(), userID)
	if err != nil || view.FirstName != "Ada" || view.Email != "me@example.com" {
		t.Fatalf("%+v err=%v", view, err)
	}
	if view.Role != domainuser.RoleUser || view.Status != domainuser.StatusActive {
		t.Fatalf("%+v", view)
	}

	phone := "555"
	first := "Augusta"
	updated, err := svc.UpdateMyProfile(context.Background(), userID, ProfilePatch{
		FirstName: &first, PhoneSet: true, PhoneValue: &phone,
	})
	if err != nil || updated.FirstName != "Augusta" || updated.Phone == nil || *updated.Phone != "555" {
		t.Fatalf("%+v err=%v", updated, err)
	}
	cleared, err := svc.UpdateMyProfile(context.Background(), userID, ProfilePatch{PhoneSet: true, PhoneValue: nil})
	if err != nil || cleared.Phone != nil {
		t.Fatalf("%+v err=%v", cleared, err)
	}
}

func TestUpdateMyProfileValidation(t *testing.T) {
	svc, store, _ := NewMemoryServiceForTest(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "v@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	var userID uuid.UUID
	for id := range store.users {
		userID = id
	}
	empty := ""
	_, err := svc.UpdateMyProfile(context.Background(), userID, ProfilePatch{FirstName: &empty})
	ae, _ := apperr.As(err)
	if ae == nil || ae.Kind != apperr.KindValidation {
		t.Fatalf("err=%v", err)
	}
}

func TestListAndRevokeSessions(t *testing.T) {
	svc, _, _ := NewMemoryServiceForTest(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "sess@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	a, err := svc.Login(context.Background(), LoginInput{
		Email: "sess@example.com", Password: "Password1", ClientContext: domainauth.ClientContextPublicWeb,
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Login(context.Background(), LoginInput{
		Email: "sess@example.com", Password: "Password1", ClientContext: domainauth.ClientContextMobile,
	})
	if err != nil {
		t.Fatal(err)
	}
	pa, err := svc.AuthenticateAccessToken(context.Background(), a.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	list, err := svc.ListMySessions(context.Background(), pa.UserID, pa.SessionID, nil, nil)
	if err != nil || len(list.Items) != 2 {
		t.Fatalf("%+v err=%v", list, err)
	}
	currentCount := 0
	for _, item := range list.Items {
		if item.IsCurrent {
			currentCount++
		}
	}
	if currentCount != 1 {
		t.Fatalf("current=%d", currentCount)
	}

	pb, err := svc.AuthenticateAccessToken(context.Background(), b.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.RevokeMySession(context.Background(), pa, pb.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AuthenticateAccessToken(context.Background(), b.AccessToken); err == nil {
		t.Fatal("revoked session must fail auth")
	}
	_, err = svc.RevokeMySession(context.Background(), pa, pb.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RevokeMySession(context.Background(), pa, uuid.New()); err == nil {
		t.Fatal("unknown session")
	} else if ae, _ := apperr.As(err); ae.Code != apperr.CodeNotFound {
		t.Fatalf("code=%s", ae.Code)
	}
}

func TestLogoutAllSessions(t *testing.T) {
	svc, _, _ := NewMemoryServiceForTest(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "all@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	a, _ := svc.Login(context.Background(), LoginInput{
		Email: "all@example.com", Password: "Password1", ClientContext: domainauth.ClientContextPublicWeb,
	})
	b, _ := svc.Login(context.Background(), LoginInput{
		Email: "all@example.com", Password: "Password1", ClientContext: domainauth.ClientContextMobile,
	})
	pa, _ := svc.AuthenticateAccessToken(context.Background(), a.AccessToken)
	_, err := svc.LogoutAllSessions(context.Background(), pa)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AuthenticateAccessToken(context.Background(), a.AccessToken); err == nil {
		t.Fatal("current revoked")
	}
	if _, err := svc.AuthenticateAccessToken(context.Background(), b.AccessToken); err == nil {
		t.Fatal("other revoked")
	}
}

func TestRevokeOtherUserSessionOwnership(t *testing.T) {
	svc, _, _ := NewMemoryServiceForTest(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "a1@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "a2@example.com", Password: "Password1", FirstName: "C", LastName: "D",
	})
	tok1, _ := svc.Login(context.Background(), LoginInput{
		Email: "a1@example.com", Password: "Password1", ClientContext: domainauth.ClientContextPublicWeb,
	})
	tok2, _ := svc.Login(context.Background(), LoginInput{
		Email: "a2@example.com", Password: "Password1", ClientContext: domainauth.ClientContextPublicWeb,
	})
	p1, _ := svc.AuthenticateAccessToken(context.Background(), tok1.AccessToken)
	p2, _ := svc.AuthenticateAccessToken(context.Background(), tok2.AccessToken)
	_, err := svc.RevokeMySession(context.Background(), p1, p2.SessionID)
	ae, _ := apperr.As(err)
	if ae == nil || ae.Code != apperr.CodeNotFound {
		t.Fatalf("err=%v", err)
	}
	if _, err := svc.AuthenticateAccessToken(context.Background(), tok2.AccessToken); err != nil {
		t.Fatalf("victim session must remain usable: %v", err)
	}
}

func TestListSessionsPagination(t *testing.T) {
	svc, _, clock := NewMemoryServiceForTest(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "page@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	var last TokenResult
	for i := 0; i < 3; i++ {
		clock.T = clock.T.Add(time.Minute)
		tok, err := svc.Login(context.Background(), LoginInput{
			Email: "page@example.com", Password: "Password1", ClientContext: domainauth.ClientContextPublicWeb,
		})
		if err != nil {
			t.Fatal(err)
		}
		last = tok
	}
	p, _ := svc.AuthenticateAccessToken(context.Background(), last.AccessToken)
	lim := 2
	page1, err := svc.ListMySessions(context.Background(), p.UserID, p.SessionID, nil, &lim)
	if err != nil || !page1.HasMore || len(page1.Items) != 2 || page1.NextCursor == nil {
		t.Fatalf("%+v err=%v", page1, err)
	}
	page2, err := svc.ListMySessions(context.Background(), p.UserID, p.SessionID, page1.NextCursor, &lim)
	if err != nil || page2.HasMore || len(page2.Items) != 1 {
		t.Fatalf("%+v err=%v", page2, err)
	}
}

func TestConcurrentLogoutAll(t *testing.T) {
	svc, _, _ := NewMemoryServiceForTest(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "c@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	tok, _ := svc.Login(context.Background(), LoginInput{
		Email: "c@example.com", Password: "Password1", ClientContext: domainauth.ClientContextPublicWeb,
	})
	p, _ := svc.AuthenticateAccessToken(context.Background(), tok.AccessToken)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.LogoutAllSessions(context.Background(), p)
		}()
	}
	wg.Wait()
}

func TestEmptySessionListIsEmptySlice(t *testing.T) {
	svc, store, _ := NewMemoryServiceForTest(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "empty@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	var userID uuid.UUID
	for id := range store.users {
		userID = id
	}
	out, err := svc.ListMySessions(context.Background(), userID, uuid.New(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Items == nil || len(out.Items) != 0 || out.HasMore {
		t.Fatalf("%+v", out)
	}
}

func TestListSessionsInvalidCursorBadRequest(t *testing.T) {
	svc, store, _ := NewMemoryServiceForTest(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "cur@example.com", Password: "Password1", FirstName: "A", LastName: "B",
	})
	var userID uuid.UUID
	for id := range store.users {
		userID = id
	}
	bad := "%%%not-a-cursor%%%"
	_, err := svc.ListMySessions(context.Background(), userID, uuid.New(), &bad, nil)
	ae, _ := apperr.As(err)
	if ae == nil || ae.Kind != apperr.KindBadRequest || ae.Code != apperr.CodeValidation {
		t.Fatalf("err=%v", err)
	}
}

func TestUpdateMyProfileEmptyPatch(t *testing.T) {
	svc, store, _ := NewMemoryServiceForTest(t)
	_, _ = svc.Register(context.Background(), RegisterInput{
		Email: "patch@example.com", Password: "Password1", FirstName: "Ada", LastName: "Lovelace",
	})
	var userID uuid.UUID
	for id := range store.users {
		userID = id
	}
	out, err := svc.UpdateMyProfile(context.Background(), userID, ProfilePatch{})
	if err != nil || out.FirstName != "Ada" {
		t.Fatalf("%+v err=%v", out, err)
	}
}
