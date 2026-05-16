package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/domain/user"
)

// LoginInput carries the credentials submitted by the client.
type LoginInput struct {
	Email    string
	Password string
}

// LoginOutput carries the authenticated user and both token types.
type LoginOutput struct {
	User         *user.User
	AccessToken  string
	RefreshToken string
}

// LoginUseCase authenticates a user with email + password and issues tokens.
type LoginUseCase struct {
	userRepo       user.Repository
	hasher         PasswordHasher
	tokenSvc       TokenService
	tokenStore     TokenStore
	refreshTokenTTL time.Duration
}

// NewLoginUseCase constructs a LoginUseCase.
func NewLoginUseCase(
	userRepo user.Repository,
	hasher PasswordHasher,
	tokenSvc TokenService,
	tokenStore TokenStore,
	refreshTokenTTL time.Duration,
) *LoginUseCase {
	return &LoginUseCase{
		userRepo:        userRepo,
		hasher:          hasher,
		tokenSvc:        tokenSvc,
		tokenStore:      tokenStore,
		refreshTokenTTL: refreshTokenTTL,
	}
}

// Execute authenticates the user and returns access + refresh tokens on success.
func (uc *LoginUseCase) Execute(ctx context.Context, input LoginInput) (*LoginOutput, error) {
	u, err := uc.userRepo.FindByEmail(ctx, input.Email)
	if err != nil {
		// Return the same error for "not found" and "wrong password" to prevent
		// user enumeration: an attacker cannot distinguish between the two cases.
		return nil, user.ErrInvalidCredentials
	}

	if err := uc.hasher.Compare(u.PasswordHash, input.Password); err != nil {
		return nil, user.ErrInvalidCredentials
	}

	if !u.IsActive() {
		return nil, user.ErrAccountInactive
	}

	accessToken, err := uc.tokenSvc.GenerateAccessToken(u.ID, u.Role.String())
	if err != nil {
		return nil, fmt.Errorf("generating access token: %w", err)
	}

	refreshToken, err := generateOpaqueToken()
	if err != nil {
		return nil, fmt.Errorf("generating refresh token: %w", err)
	}

	if err := uc.tokenStore.Set(ctx, refreshToken, u.ID, uc.refreshTokenTTL); err != nil {
		return nil, fmt.Errorf("storing refresh token: %w", err)
	}

	return &LoginOutput{
		User:         u,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
