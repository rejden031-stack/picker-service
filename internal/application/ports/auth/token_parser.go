package authports

import (
	"context"

	"picker-service/internal/domain/auth"
)

// TokenParser — проверка и разбор пропуска (JWT).
type TokenParser interface {
	Parse(ctx context.Context, tokenString string) (auth.User, error)
}
