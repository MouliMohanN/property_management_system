package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/domain/user"
)

// RegisterInput carries the caller-supplied fields required to create a new user.
type RegisterInput struct {
	Email       string
	Password    string
	FirstName   string
	LastName    string
	Role        user.Role
	PhoneNumber *string
}

// RegisterOutput carries the newly created user.
type RegisterOutput struct {
	User *user.User
}

// RegisterUseCase handles user creation. In production, this endpoint is
// restricted to admins; access control is enforced at the transport layer.
type RegisterUseCase struct {
	userRepo user.Repository
	hasher   PasswordHasher
}

// NewRegisterUseCase constructs a RegisterUseCase with its required dependencies.
func NewRegisterUseCase(userRepo user.Repository, hasher PasswordHasher) *RegisterUseCase {
	return &RegisterUseCase{userRepo: userRepo, hasher: hasher}
}

// Execute registers a new user account.
func (uc *RegisterUseCase) Execute(ctx context.Context, input RegisterInput) (*RegisterOutput, error) {
	if !input.Role.IsValid() {
		return nil, fmt.Errorf("invalid role %q: %w", input.Role, user.ErrNotFound)
	}

	// Check uniqueness before hashing to fail fast without spending CPU on bcrypt.
	existing, err := uc.userRepo.FindByEmail(ctx, input.Email)
	if err != nil && !errors.Is(err, user.ErrNotFound) {
		return nil, fmt.Errorf("checking email uniqueness: %w", err)
	}
	if existing != nil {
		return nil, user.ErrEmailTaken
	}

	hash, err := uc.hasher.Hash(input.Password)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	u := &user.User{
		Email:        input.Email,
		PhoneNumber:  input.PhoneNumber,
		PasswordHash: hash,
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		Role:         input.Role,
		Status:       user.StatusActive,
	}

	if err := uc.userRepo.Create(ctx, u); err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	return &RegisterOutput{User: u}, nil
}
