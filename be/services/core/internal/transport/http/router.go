package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/domain/user"
	"github.com/MouliMohanN/property_management_system/be/services/core/internal/transport/http/handler"
	"github.com/MouliMohanN/property_management_system/be/services/core/internal/transport/http/middleware"
	"github.com/MouliMohanN/property_management_system/be/services/core/internal/usecase/auth"
)

// newRouter builds and returns the chi router with all routes and middleware registered.
func newRouter(authHandler *handler.AuthHandler, tokenSvc auth.TokenService, corsOrigins []string) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.CORS(corsOrigins))
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Route("/api/v1/auth", func(r chi.Router) {
		// Public endpoints
		r.Post("/login", authHandler.Login)
		r.Post("/refresh", authHandler.Refresh)

		// Admin-only: register a new user
		r.With(
			middleware.Authenticate(tokenSvc),
			middleware.Require(user.RoleAdmin),
		).Post("/register", authHandler.Register)

		// Authenticated endpoints
		r.Group(func(r chi.Router) {
			r.Use(middleware.Authenticate(tokenSvc))
			r.Post("/logout", authHandler.Logout)
			r.Get("/me", authHandler.Me)
		})
	})

	return r
}
