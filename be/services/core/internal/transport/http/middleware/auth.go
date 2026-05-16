package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/usecase/auth"
)

// contextKey is an unexported type for context keys scoped to this package.
// Using a typed key prevents collisions with keys set by other packages.
type contextKey int

const (
	contextKeyUserID contextKey = iota
	contextKeyRole
)

// Authenticate validates the Bearer JWT in the Authorization header and injects
// the parsed UserID and Role into the request context. Requests without a valid
// token receive a 401.
func Authenticate(tokenSvc auth.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"missing authorization header"}}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"invalid authorization header format"}}`, http.StatusUnauthorized)
				return
			}

			claims, err := tokenSvc.ValidateAccessToken(parts[1])
			if err != nil {
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"invalid or expired token"}}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), contextKeyUserID, claims.UserID)
			ctx = context.WithValue(ctx, contextKeyRole, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext retrieves the authenticated user's UUID from the context.
// Returns uuid.Nil if the Authenticate middleware was not applied.
func UserIDFromContext(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(contextKeyUserID).(uuid.UUID); ok {
		return v
	}
	return uuid.Nil
}

// RoleFromContext retrieves the authenticated user's role string from the context.
func RoleFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(contextKeyRole).(string); ok {
		return v
	}
	return ""
}
