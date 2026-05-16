package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrTokenNotFound is returned by TokenStore.Get when the token key does not
// exist in the store — either it was never set, has expired, or was already
// deleted (e.g. after logout or rotation).
var ErrTokenNotFound = errors.New("token not found or expired")

// TokenStore manages the lifecycle of opaque refresh tokens in an external
// store (e.g. Redis). Each token maps to exactly one user ID.
type TokenStore interface {
	Set(ctx context.Context, token string, userID uuid.UUID, ttl time.Duration) error
	Get(ctx context.Context, token string) (uuid.UUID, error)
	Delete(ctx context.Context, token string) error
}

// PasswordHasher abstracts the password hashing algorithm so that the usecase
// layer never imports bcrypt directly — making it easy to swap algorithms.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

// TokenService handles JWT generation and validation for access tokens.
type TokenService interface {
	GenerateAccessToken(userID uuid.UUID, role string) (string, error)
	ValidateAccessToken(token string) (*AccessTokenClaims, error)
}

// AccessTokenClaims is the decoded payload of a validated access token.
type AccessTokenClaims struct {
	UserID uuid.UUID
	Role   string
	JTI    string
}
