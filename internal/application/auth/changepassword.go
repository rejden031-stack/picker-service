package authapp

import (
	"context"
	"fmt"

	authports "picker-service/internal/application/ports/auth"
	"picker-service/internal/application/ports/store"
	"picker-service/internal/domain/audit"
	"picker-service/internal/domain/auth"
)

// ChangePassword — смена собственного пароля (старый пароль обязателен).
type ChangePassword struct {
	users  store.UserAdminStore
	hasher authports.PasswordHasher
	audit  store.AuditStore
}

func NewChangePassword(users store.UserAdminStore, hasher authports.PasswordHasher,
	audit store.AuditStore) *ChangePassword {
	return &ChangePassword{users: users, hasher: hasher, audit: audit}
}

func (uc *ChangePassword) Change(ctx context.Context, actor auth.User, oldPass, newPass string) error {
	if len(newPass) < 8 {
		return auth.ErrWeakPassword
	}
	hash, err := uc.users.PasswordHashByID(ctx, actor.ID)
	if err != nil {
		return fmt.Errorf("fetch hash: %w", err)
	}
	if !uc.hasher.Verify(ctx, hash, oldPass) {
		return auth.ErrWrongPassword
	}
	newHash, err := uc.hasher.Hash(ctx, newPass)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}
	if err := uc.users.SetPasswordHash(ctx, actor.ID, newHash); err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	if err := uc.audit.Append(ctx, audit.Entry{
		ActorID: actor.ID, ActorName: actor.Username,
		Action: audit.PasswordChanged, EntityID: actor.ID,
	}); err != nil {
		return fmt.Errorf("audit password: %w", err)
	}
	return nil
}
