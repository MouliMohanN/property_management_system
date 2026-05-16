package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/domain/user"
)

// GetMeInput carries the authenticated user's ID, extracted from the JWT by middleware.
type GetMeInput struct {
	UserID uuid.UUID
}

// GetMeOutput carries the full user record.
type GetMeOutput struct {
	User *user.User
}

// GetMeUseCase retrieves the profile of the currently authenticated user.
type GetMeUseCase struct {
	userRepo user.Repository
}

// NewGetMeUseCase constructs a GetMeUseCase.
func NewGetMeUseCase(userRepo user.Repository) *GetMeUseCase {
	return &GetMeUseCase{userRepo: userRepo}
}

// Execute fetches the user record by ID.
func (uc *GetMeUseCase) Execute(ctx context.Context, input GetMeInput) (*GetMeOutput, error) {
	u, err := uc.userRepo.FindByID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("finding user by id: %w", err)
	}
	return &GetMeOutput{User: u}, nil
}
