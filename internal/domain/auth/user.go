package auth

import "errors"

// Role — роль пользователя.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleWorker Role = "worker"
	RoleSenior Role = "senior"
)

// Valid — признана ли роль штатной.
func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleWorker, RoleSenior:
		return true
	}
	return false
}

// User — сотрудник склада.
type User struct {
	ID       int64
	Username string
	Role     Role
}

// ErrInvalidRole — если в данные попала неизвестная роль.
var ErrInvalidRole = errors.New("invalid role")

// ErrBlocked — учётная запись заблокирована.
var ErrBlocked = errors.New("account blocked")

// ErrUsernameTaken — имя пользователя уже занято.
var ErrUsernameTaken = errors.New("username taken")
