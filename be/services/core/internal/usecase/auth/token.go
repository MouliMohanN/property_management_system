package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// generateOpaqueToken produces a cryptographically random 32-byte hex string
// suitable for use as an opaque refresh token. hex encoding doubles the byte
// length, yielding a 64-character URL-safe ASCII string.
func generateOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
