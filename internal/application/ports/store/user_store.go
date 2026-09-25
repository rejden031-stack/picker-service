package store

import (
	"context"

	"picker-service/internal/domain/auth"
)

// Credentials — учётные данные пользователя из хранилища.
type Credentials struct {
	User         auth.User
	PasswordHash string
	Blocked      bool
}

// UserStore — «разъём» для хранилища сотрудников (вход в систему).
type UserStore interface {
	// UserByUsername возвращает учётные данные по имени пользователя.
	UserByUsername(ctx context.Context, username string) (Credentials, error)
}
