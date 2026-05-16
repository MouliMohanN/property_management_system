package middleware

import (
	"net/http"

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/domain/user"
)

// Require returns a middleware that allows only requests from users with one of
// the specified roles. It must be placed after Authenticate in the handler chain
// so that the role is already present in context.
func Require(roles ...user.Role) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r.String()] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := RoleFromContext(r.Context())
			if _, ok := allowed[role]; !ok {
				http.Error(w, `{"error":{"code":"FORBIDDEN","message":"insufficient permissions"}}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
