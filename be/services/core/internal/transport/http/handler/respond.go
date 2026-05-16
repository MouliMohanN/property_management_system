package handler

import (
	"encoding/json"
	"net/http"
)

// envelope is the top-level JSON wrapper for all responses.
// Success responses use the "data" key; error responses use the "error" key.
type envelope map[string]any

// respondJSON serialises v to JSON and writes it with the given status code.
// Internal serialisation failures fall back to a plain-text 500 to avoid
// recursion. Content-Type is always set before WriteHeader per the http spec.
func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"response encoding failed"}}`, http.StatusInternalServerError)
	}
}

// respondError writes a structured error response.
func respondError(w http.ResponseWriter, status int, code, message string) {
	respondJSON(w, status, envelope{
		"error": envelope{
			"code":    code,
			"message": message,
		},
	})
}
