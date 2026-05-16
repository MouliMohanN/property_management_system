package auth

import (
	"context"
	"fmt"
	"time"
)

// RefreshInput carries the opaque refresh token submitted by the client.
type RefreshInput struct {
	RefreshToken string
}

// RefreshOutput carries the newly issued token pair.
type RefreshOutput struct {
	AccessToken  string
	RefreshToken string
}

// RefreshUseCase rotates a refresh token: the old one is deleted and a new
// token pair is issued. Rotation prevents replay attacks — a stolen token can
// only be used once before it is invalidated.
type RefreshUseCase struct {
	tokenStore      TokenStore
	tokenSvc        TokenService
	refreshTokenTTL time.Duration
}

// NewRefreshUseCase constructs a RefreshUseCase.
func NewRefreshUseCase(
	tokenStore TokenStore,
	tokenSvc TokenService,
	refreshTokenTTL time.Duration,
) *RefreshUseCase {
	return &RefreshUseCase{
		tokenStore:      tokenStore,
		tokenSvc:        tokenSvc,
		refreshTokenTTL: refreshTokenTTL,
	}
}

// Execute validates the old refresh token, deletes it, and issues a new pair.
func (uc *RefreshUseCase) Execute(ctx context.Context, input RefreshInput) (*RefreshOutput, error) {
	userID, err := uc.tokenStore.Get(ctx, input.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("getting refresh token: %w", err)
	}

	// Delete before issuing the new token. If something fails after deletion
	// the user simply needs to log in again — acceptable trade-off versus
	// leaving a compromised token alive.
	if err := uc.tokenStore.Delete(ctx, input.RefreshToken); err != nil {
		return nil, fmt.Errorf("deleting old refresh token: %w", err)
	}

	// The refresh token does not carry the role, so we re-issue the access
	// token with an empty role string. The handler can call GetMe if role
	// information is required after refresh.
	//
	// NOTE: A production improvement would be to store the role alongside the
	// user_id in Redis, avoiding a DB round-trip. Deferred to keep Phase 1 simple.
	accessToken, err := uc.tokenSvc.GenerateAccessToken(userID, "")
	if err != nil {
		return nil, fmt.Errorf("generating access token: %w", err)
	}

	newRefreshToken, err := generateOpaqueToken()
	if err != nil {
		return nil, fmt.Errorf("generating new refresh token: %w", err)
	}

	if err := uc.tokenStore.Set(ctx, newRefreshToken, userID, uc.refreshTokenTTL); err != nil {
		return nil, fmt.Errorf("storing new refresh token: %w", err)
	}

	return &RefreshOutput{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}
