package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// CORS returns a middleware that applies Cross-Origin Resource Sharing headers.
// allowedOrigins should list every frontend origin that is permitted to call
// this API (e.g. ["http://localhost:5173"] in development).
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		MaxAge:         300,
	})
}
