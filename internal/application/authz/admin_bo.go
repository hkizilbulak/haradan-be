// Package authz centralizes role and client-context authorization checks.
package authz

import (
	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainauth "github.com/hkizilbulak/haradan-be/internal/domain/auth"
	domainuser "github.com/hkizilbulak/haradan-be/internal/domain/user"
)

const forbiddenMessage = "Bu işlem için yetkiniz yok."

// RequireAdminBO accepts only role=admin with session clientContext=ADMIN_BO.
// No other moderator/staff roles are defined in phase one.
func RequireAdminBO(p domainauth.Principal) error {
	if p.Role != string(domainuser.RoleAdmin) || p.ClientContext != domainauth.ClientContextAdminBO {
		return apperr.Forbidden(apperr.CodeForbidden, forbiddenMessage)
	}
	return nil
}
