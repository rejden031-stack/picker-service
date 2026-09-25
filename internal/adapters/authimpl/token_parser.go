package authimpl

import (
	"context"
	"fmt"

	"github.com/golang-jwt/jwt/v5"

	"picker-service/internal/domain/auth"
)

// Parse проверяет подпись, срок действия и возвращает пользователя.
func (t *TokenIssuer) Parse(_ context.Context, tokenString string) (auth.User, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return t.secret, nil
	})
	if err != nil {
		return auth.User{}, auth.ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return auth.User{}, auth.ErrInvalidToken
	}

	uid, ok := claims["uid"].(float64)
	if !ok {
		return auth.User{}, auth.ErrInvalidToken
	}
	role, ok := claims["role"].(string)
	if !ok {
		return auth.User{}, auth.ErrInvalidToken
	}

	var u auth.User
	u.ID = int64(uid)
	u.Username, _ = claims["sub"].(string)
	u.Role = auth.Role(role)
	return u, nil
}
