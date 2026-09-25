package authapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"picker-service/internal/application/ports/store"
	"picker-service/internal/domain/audit"
	"picker-service/internal/domain/auth"
)

type fakeUser struct {
	user    auth.User
	hash    string
	blocked bool
}

type fakeUserStore struct {
	users map[string]fakeUser
	err   error
}

func (s *fakeUserStore) UserByUsername(_ context.Context, username string) (store.Credentials, error) {
	if s.err != nil {
		return store.Credentials{}, s.err
	}
	f, ok := s.users[username]
	if !ok {
		return store.Credentials{}, auth.ErrNotFound
	}
	return store.Credentials{User: f.user, PasswordHash: f.hash, Blocked: f.blocked}, nil
}

type fakeAuditStore struct {
	entries []audit.Entry
}

func (s *fakeAuditStore) Append(_ context.Context, e audit.Entry) error {
	s.entries = append(s.entries, e)
	return nil
}

func (s *fakeAuditStore) List(_ context.Context, _, _ int) ([]audit.Entry, error) {
	return s.entries, nil
}

type fakeHasher struct{}

func (fakeHasher) Hash(_ context.Context, password string) (string, error) { return password, nil }
func (fakeHasher) Verify(_ context.Context, hash, password string) bool    { return hash == password }

type fakeIssuer struct {
	token string
}

func (i fakeIssuer) Issue(_ context.Context, _ auth.User, _ time.Duration) (string, error) {
	return i.token, nil
}

func setupLogin(t *testing.T) *Login {
	t.Helper()
	store := &fakeUserStore{users: map[string]fakeUser{
		"senior": {user: auth.User{ID: 1, Username: "senior", Role: auth.RoleSenior}, hash: "123456"},
	}}
	return NewLogin(store, fakeHasher{}, fakeIssuer{token: "tok"}, &fakeAuditStore{})
}

func TestLoginSuccess(t *testing.T) {
	uc := setupLogin(t)

	token, role, err := uc.Login(context.Background(), "senior", "123456", "10.0.0.5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "tok" {
		t.Errorf("token = %q, want %q", token, "tok")
	}
	if role != auth.RoleSenior {
		t.Errorf("role = %q, want %q", role, auth.RoleSenior)
	}
}

func TestLoginUnknownUser(t *testing.T) {
	uc := setupLogin(t)

	_, _, err := uc.Login(context.Background(), "ghost", "whatever", "10.0.0.5")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	uc := setupLogin(t)

	_, _, err := uc.Login(context.Background(), "senior", "111111", "10.0.0.5")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginBlocked(t *testing.T) {
	uc := &Login{
		users: &fakeUserStore{users: map[string]fakeUser{
			"fired": {user: auth.User{ID: 9, Username: "fired", Role: auth.RoleWorker},
				hash: "x", blocked: true},
		}},
		hasher: fakeHasher{}, tokens: fakeIssuer{token: "tok"},
		audit: &fakeAuditStore{},
	}

	_, _, err := uc.Login(context.Background(), "fired", "x", "10.0.0.5")
	if !errors.Is(err, auth.ErrBlocked) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
}

func TestLoginStoreErrorPropagates(t *testing.T) {
	uc := NewLogin(&fakeUserStore{err: errors.New("db down")}, fakeHasher{},
		fakeIssuer{token: "tok"}, &fakeAuditStore{})

	_, _, err := uc.Login(context.Background(), "x", "y", "10.0.0.5")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("store error must not be masked as invalid credentials")
	}
}
