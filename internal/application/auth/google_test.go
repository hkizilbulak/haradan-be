package auth

import (
	"context"
	"testing"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
)

func TestGoogleLoginRejectsAdminBO(t *testing.T) {
	svc, _, _ := newTestSvc(t)
	_, err := svc.GoogleLogin(context.Background(), GoogleLoginInput{
		IDToken:       "dummy-token",
		ClientContext: domainauth.ClientContextAdminBO,
	})
	ae, ok := apperr.As(err)
	if !ok || ae.Kind != apperr.KindForbidden {
		t.Fatalf("expected Forbidden error for ADMIN_BO, got: %v", err)
	}
}

func TestGoogleLoginRejectsEmptyToken(t *testing.T) {
	svc, _, _ := newTestSvc(t)
	_, err := svc.GoogleLogin(context.Background(), GoogleLoginInput{
		IDToken:       "",
		ClientContext: domainauth.ClientContextPublicWeb,
	})
	ae, ok := apperr.As(err)
	if !ok || ae.Kind != apperr.KindValidation {
		t.Fatalf("expected Validation error for empty token, got: %v", err)
	}
}

func TestGoogleLoginRejectsInvalidToken(t *testing.T) {
	svc, _, _ := newTestSvc(t)
	_, err := svc.GoogleLogin(context.Background(), GoogleLoginInput{
		IDToken:       "invalid-google-token",
		ClientContext: domainauth.ClientContextPublicWeb,
	})
	if err == nil {
		t.Fatal("expected error for invalid token, got nil")
	}
}
