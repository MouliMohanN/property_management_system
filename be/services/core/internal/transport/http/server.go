package http

import (
	"context"
	"net/http"
	"time"

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/transport/http/handler"
	"github.com/MouliMohanN/property_management_system/be/services/core/internal/usecase/auth"
)

// Server wraps the standard library http.Server with our application router.
type Server struct {
	httpServer *http.Server
}

// NewServer constructs a Server and wires up the router.
func NewServer(port string, authHandler *handler.AuthHandler, tokenSvc auth.TokenService) *Server {
	router := newRouter(authHandler, tokenSvc)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
		// Timeouts prevent slow or malicious clients from exhausting connections.
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{httpServer: srv}
}

// ListenAndServe starts the HTTP server. It blocks until the server is stopped.
// Returns http.ErrServerClosed on graceful shutdown.
func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully drains in-flight requests using the provided context.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
