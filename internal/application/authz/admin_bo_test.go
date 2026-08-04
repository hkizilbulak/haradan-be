package authz_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/application/authz"
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

func TestRequireAdminBO(t *testing.T) {
	t.Parallel()
	adminBO := domainauth.Principal{
		UserID: uuid.New(), SessionID: uuid.New(),
		Role: string(domainuser.RoleAdmin), ClientContext: domainauth.ClientContextAdminBO,
	}
	if err := authz.RequireAdminBO(adminBO); err != nil {
		t.Fatalf("admin+ADMIN_BO must pass: %v", err)
	}

	cases := []domainauth.Principal{
		{Role: string(domainuser.RoleUser), ClientContext: domainauth.ClientContextAdminBO},
		{Role: string(domainuser.RoleAdmin), ClientContext: domainauth.ClientContextPublicWeb},
		{Role: string(domainuser.RoleUser), ClientContext: domainauth.ClientContextPublicWeb},
		{Role: "moderator", ClientContext: domainauth.ClientContextAdminBO},
	}
	for _, p := range cases {
		err := authz.RequireAdminBO(p)
		ae, ok := apperr.As(err)
		if !ok || ae.Code != apperr.CodeForbidden {
			t.Fatalf("p=%+v err=%v", p, err)
		}
	}
}
