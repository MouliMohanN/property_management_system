package password

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/domain/user"
)

// BcryptHasher implements auth.PasswordHasher using bcrypt.
type BcryptHasher struct{}

// NewBcryptHasher constructs a BcryptHasher.
func NewBcryptHasher() *BcryptHasher {
	return &BcryptHasher{}
}

// Hash produces a bcrypt hash of the given plaintext password.
func (h *BcryptHasher) Hash(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hashing password with bcrypt: %w", err)
	}
	return string(hashed), nil
}

// Compare verifies that password matches the stored hash.
// Returns user.ErrInvalidCredentials on mismatch so callers do not need to
// import bcrypt directly to interpret the result.
func (h *BcryptHasher) Compare(hash, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return user.ErrInvalidCredentials
		}
		return fmt.Errorf("comparing password hash: %w", err)
	}
	return nil
}
