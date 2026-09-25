package store

import (
	"context"

	"picker-service/internal/domain/auth"
)

// UserAdminStore — «разъём» для администрирования сотрудников.
type UserAdminStore interface {
	UserByID(ctx context.Context, id int64) (auth.User, error)
	PasswordHashByID(ctx context.Context, id int64) (string, error)
	CreateUser(ctx context.Context, username, passwordHash string, role auth.Role) (auth.User, error)
	ListUsers(ctx context.Context) ([]auth.User, error)
	SetRole(ctx context.Context, id int64, role auth.Role) error
	SetBlocked(ctx context.Context, id int64, blocked bool) error
	SetPasswordHash(ctx context.Context, id int64, hash string) error
}
