package user

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Status represents the lifecycle state of a user account.
type Status string

const (
	StatusActive    Status = "active"
	StatusInactive  Status = "inactive"
	StatusSuspended Status = "suspended"
)

// User is the central domain entity for the user bounded context.
// It carries no dependencies on infrastructure or application layers.
type User struct {
	ID           uuid.UUID
	Email        string
	PhoneNumber  *string
	PasswordHash string
	FirstName    string
	LastName     string
	Role         Role
	Status       Status
	Version      int // used for optimistic locking — increment on every update
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// IsActive reports whether the user account is in the active state and
// therefore allowed to authenticate.
func (u *User) IsActive() bool {
	return u.Status == StatusActive
}

// FullName returns the user's display name by joining first and last name.
func (u *User) FullName() string {
	return strings.TrimSpace(u.FirstName + " " + u.LastName)
}
