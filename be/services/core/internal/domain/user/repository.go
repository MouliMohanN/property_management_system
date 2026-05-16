package user

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines the persistence contract for the User aggregate.
// All implementations must reside in the infrastructure layer.
type Repository interface {
	Create(ctx context.Context, u *User) error
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	// Update applies the entity's current field values. It uses optimistic locking:
	// the WHERE clause includes the current version and the implementation increments
	// it. Returns ErrConflict if 0 rows were affected.
	Update(ctx context.Context, u *User) error
	Count(ctx context.Context) (int64, error)
}
