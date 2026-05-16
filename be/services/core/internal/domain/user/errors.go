package user

import "errors"

var (
	// ErrNotFound is returned when a user lookup yields no result.
	ErrNotFound = errors.New("user not found")

	// ErrEmailTaken is returned when registration is attempted with an email
	// that already exists in the system.
	ErrEmailTaken = errors.New("email already taken")

	// ErrInvalidCredentials is returned when a password comparison fails.
	// Kept intentionally vague to prevent user enumeration attacks.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrAccountInactive is returned when an authenticated action is attempted
	// on a user account that is not in the active state.
	ErrAccountInactive = errors.New("account is not active")

	// ErrConflict is returned by Update when the version check fails, indicating
	// a concurrent modification occurred. The caller should reload and retry.
	ErrConflict = errors.New("update conflict: user was modified concurrently")
)
