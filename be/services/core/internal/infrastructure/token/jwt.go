package token

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/usecase/auth"
)

// jwtClaims extends the standard registered claims with our domain-specific fields.
type jwtClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"uid"`
	Role   string `json:"role"`
}

// JWTService implements auth.TokenService using HMAC-SHA256 signed JWTs.
type JWTService struct {
	secret    []byte
	accessTTL time.Duration
}

// NewJWTService constructs a JWTService.
func NewJWTService(secret string, accessTTL time.Duration) *JWTService {
	return &JWTService{
		secret:    []byte(secret),
		accessTTL: accessTTL,
	}
}

// GenerateAccessToken creates a signed JWT for the given user and role.
// The JTI is a random 16-byte hex string used for future token revocation lookups.
func (s *JWTService) GenerateAccessToken(userID uuid.UUID, role string) (string, error) {
	jti, err := randomHex(16)
	if err != nil {
		return "", fmt.Errorf("generating jti: %w", err)
	}

	now := time.Now()
	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
			ID:        jti,
		},
		UserID: userID.String(),
		Role:   role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("signing jwt: %w", err)
	}
	return signed, nil
}

// ValidateAccessToken parses and verifies a JWT string, returning the decoded claims.
func (s *JWTService) ValidateAccessToken(tokenStr string) (*auth.AccessTokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parsing jwt: %w", err)
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("parsing user id from claims: %w", err)
	}

	return &auth.AccessTokenClaims{
		UserID: userID,
		Role:   claims.Role,
		JTI:    claims.ID,
	}, nil
}

// randomHex returns a hex-encoded string of n random bytes.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
