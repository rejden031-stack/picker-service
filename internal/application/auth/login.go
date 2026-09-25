package authapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	authports "picker-service/internal/application/ports/auth"
	"picker-service/internal/application/ports/store"
	"picker-service/internal/domain/audit"
	"picker-service/internal/domain/auth"
)

// ErrInvalidCredentials — неверный логин или пароль.
var ErrInvalidCredentials = errors.New("invalid credentials")

// Login — сценарий «вход в систему».
type Login struct {
	users  store.UserStore
	hasher authports.PasswordHasher
	tokens authports.TokenIssuer
	audit  store.AuditStore
}

func NewLogin(users store.UserStore, hasher authports.PasswordHasher,
	tokens authports.TokenIssuer, audit store.AuditStore) *Login {
	return &Login{users: users, hasher: hasher, tokens: tokens, audit: audit}
}

// Login возвращает подписанный пропуск (JWT) и роль пользователя.
// remoteIP идёт в журнал аудита (для расследований по заблокированным входам).
func (uc *Login) Login(ctx context.Context, username, password, remoteIP string) (token string, role auth.Role, err error) {
	creds, err := uc.users.UserByUsername(ctx, username)
	if errors.Is(err, auth.ErrNotFound) {
		_ = uc.audit.Append(ctx, audit.Entry{
			ActorID: 0, ActorName: username, Action: audit.LoginFailed,
			Details: "reason=user_not_found ip=" + remoteIP,
		})
		return "", "", ErrInvalidCredentials
	}
	if err != nil {
		return "", "", fmt.Errorf("login fetch user: %w", err)
	}
	if creds.Blocked {
		_ = uc.audit.Append(ctx, audit.Entry{
			ActorID: creds.User.ID, ActorName: creds.User.Username,
			Action: audit.LoginBlocked, EntityID: creds.User.ID,
			Details: "ip=" + remoteIP,
		})
		return "", "", auth.ErrBlocked
	}
	if !uc.hasher.Verify(ctx, creds.PasswordHash, password) {
		_ = uc.audit.Append(ctx, audit.Entry{
			ActorID: creds.User.ID, ActorName: creds.User.Username,
			Action: audit.LoginFailed, EntityID: creds.User.ID,
			Details: "reason=bad_password ip=" + remoteIP,
		})
		return "", "", ErrInvalidCredentials
	}

	token, err = uc.tokens.Issue(ctx, creds.User, 12*time.Hour)
	if err != nil {
		return "", "", fmt.Errorf("issue token: %w", err)
	}
	if err := uc.audit.Append(ctx, audit.Entry{
		ActorID: creds.User.ID, ActorName: creds.User.Username,
		Action: audit.LoginOK, EntityID: creds.User.ID,
		Details: "ip=" + remoteIP,
	}); err != nil {
		return "", "", fmt.Errorf("audit login: %w", err)
	}
	return token, creds.User.Role, nil
}
