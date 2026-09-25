package authports

import (
	"context"
	"time"

	"picker-service/internal/domain/auth"
)

// PasswordHasher — шифрование и проверка паролей.
type PasswordHasher interface {
	Hash(ctx context.Context, password string) (string, error)
	Verify(ctx context.Context, hash, password string) bool
}

// TokenIssuer — выпуск подписанных пропусков (JWT).
type TokenIssuer interface {
	Issue(ctx context.Context, u auth.User, ttl time.Duration) (string, error)
}
