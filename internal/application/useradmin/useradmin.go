// Package useradmin — сценарии администрирования сотрудников и аудита.
package useradmin

import (
	"context"
	"fmt"

	authports "picker-service/internal/application/ports/auth"
	"picker-service/internal/application/ports/store"
	"picker-service/internal/domain/audit"
	"picker-service/internal/domain/auth"
)

// UserAdmin — управление сотрудниками (доступно роли admin).
type UserAdmin struct {
	users  store.UserAdminStore
	audit  store.AuditStore
	hasher authports.PasswordHasher
}

func NewUserAdmin(users store.UserAdminStore, audit store.AuditStore,
	hasher authports.PasswordHasher) *UserAdmin {
	return &UserAdmin{users: users, audit: audit, hasher: hasher}
}

// CreateUser заводит нового сотрудника.
func (uc *UserAdmin) CreateUser(ctx context.Context, actor auth.User, username, password string, role auth.Role) (auth.User, error) {
	if !role.Valid() {
		return auth.User{}, auth.ErrInvalidRole
	}
	hash, err := uc.hasher.Hash(ctx, password)
	if err != nil {
		return auth.User{}, fmt.Errorf("hash password: %w", err)
	}
	u, err := uc.users.CreateUser(ctx, username, hash, role)
	if err != nil {
		return auth.User{}, err
	}
	if err := uc.audit.Append(ctx, audit.Entry{
		ActorID: actor.ID, ActorName: actor.Username,
		Action: audit.UserCreated, EntityID: u.ID,
		Details: "username=" + u.Username + " role=" + string(u.Role),
	}); err != nil {
		return auth.User{}, fmt.Errorf("audit user_created: %w", err)
	}
	return u, nil
}

func (uc *UserAdmin) ListUsers(ctx context.Context) ([]auth.User, error) {
	return uc.users.ListUsers(ctx)
}

func (uc *UserAdmin) SetRole(ctx context.Context, actor auth.User, userID int64, role auth.Role) error {
	if !role.Valid() {
		return auth.ErrInvalidRole
	}
	if userID == actor.ID && role != auth.RoleAdmin {
		return auth.ErrLastAdmin // нельзя лишить себя админ-прав
	}
	if err := uc.users.SetRole(ctx, userID, role); err != nil {
		return err
	}
	return uc.audit.Append(ctx, audit.Entry{
		ActorID: actor.ID, ActorName: actor.Username,
		Action: audit.UserRoleChanged, EntityID: userID,
		Details: "role=" + string(role),
	})
}

func (uc *UserAdmin) SetBlocked(ctx context.Context, actor auth.User, userID int64, blocked bool) error {
	if userID == actor.ID {
		return auth.ErrSelfLockout // себя блокировать нельзя
	}
	if err := uc.users.SetBlocked(ctx, userID, blocked); err != nil {
		return err
	}
	action := audit.UserBlocked
	if !blocked {
		action = audit.UserUnblocked
	}
	return uc.audit.Append(ctx, audit.Entry{
		ActorID: actor.ID, ActorName: actor.Username,
		Action: action, EntityID: userID,
	})
}

// ResetPassword ставит новый пароль (без старого).
func (uc *UserAdmin) ResetPassword(ctx context.Context, actor auth.User, userID int64, newPass string) error {
	hash, err := uc.hasher.Hash(ctx, newPass)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := uc.users.SetPasswordHash(ctx, userID, hash); err != nil {
		return err
	}
	return uc.audit.Append(ctx, audit.Entry{
		ActorID: actor.ID, ActorName: actor.Username,
		Action: audit.PasswordChanged, EntityID: userID,
		Details: "actor_target=admin",
	})
}

func (uc *UserAdmin) ListAudit(ctx context.Context, limit, offset int) ([]audit.Entry, error) {
	return uc.audit.List(ctx, limit, offset)
}
