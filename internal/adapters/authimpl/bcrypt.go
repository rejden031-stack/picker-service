package authimpl

import (
	"context"

	"golang.org/x/crypto/bcrypt"
)

// PasswordHasher — реализация порта authports.PasswordHasher на bcrypt.
type PasswordHasher struct{}

func NewPasswordHasher() *PasswordHasher {
	return &PasswordHasher{}
}

func (h *PasswordHasher) Hash(_ context.Context, password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (h *PasswordHasher) Verify(_ context.Context, hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
