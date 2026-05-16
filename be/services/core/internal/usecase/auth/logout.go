package auth

import (
	"context"
	"errors"
	"fmt"
)

// LogoutInput carries the refresh token to invalidate.
type LogoutInput struct {
	RefreshToken string
}

// LogoutUseCase invalidates a refresh token. The operation is idempotent:
// if the token is already gone (expired or deleted), we still return success.
type LogoutUseCase struct {
	tokenStore TokenStore
}

// NewLogoutUseCase constructs a LogoutUseCase.
func NewLogoutUseCase(tokenStore TokenStore) *LogoutUseCase {
	return &LogoutUseCase{tokenStore: tokenStore}
}

// Execute deletes the refresh token from the store.
func (uc *LogoutUseCase) Execute(ctx context.Context, input LogoutInput) error {
	if err := uc.tokenStore.Delete(ctx, input.RefreshToken); err != nil {
		// A missing token means the client is already logged out — treat as success.
		if errors.Is(err, ErrTokenNotFound) {
			return nil
		}
		return fmt.Errorf("deleting refresh token: %w", err)
	}
	return nil
}
