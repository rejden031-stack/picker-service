package authimpl

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"picker-service/internal/domain/auth"
)

// TokenIssuer — реализация порта authports.TokenIssuer на JWT.
type TokenIssuer struct {
	secret []byte
}

func NewTokenIssuer(secret string) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret)}
}

func (t *TokenIssuer) Issue(_ context.Context, u auth.User, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":  u.Username,
		"uid":  u.ID,
		"role": string(u.Role),
		"iat":  now.Unix(),
		"exp":  now.Add(ttl).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(t.secret)
}
