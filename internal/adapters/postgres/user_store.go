package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"picker-service/internal/application/ports/store"
	"picker-service/internal/domain/auth"
)

// UserStore — реализация портов store.UserStore и store.UserAdminStore на PostgreSQL.
type UserStore struct {
	pool *pgxpool.Pool
}

func NewUserStore(pool *pgxpool.Pool) *UserStore {
	return &UserStore{pool: pool}
}

// UserByUsername возвращает учётные данные по имени пользователя.
func (s *UserStore) UserByUsername(ctx context.Context, username string) (store.Credentials, error) {
	const q = `SELECT id, username, role, password_hash, blocked FROM users WHERE username = $1`

	var creds store.Credentials
	var role string
	if err := s.pool.QueryRow(ctx, q, username).
		Scan(&creds.User.ID, &creds.User.Username, &role, &creds.PasswordHash, &creds.Blocked); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Credentials{}, auth.ErrNotFound
		}
		return store.Credentials{}, fmt.Errorf("user by username: %w", err)
	}
	creds.User.Role = auth.Role(role)
	return creds, nil
}

// UserByID возвращает пользователя по id.
func (s *UserStore) UserByID(ctx context.Context, id int64) (auth.User, error) {
	const q = `SELECT id, username, role FROM users WHERE id = $1`

	var u auth.User
	var role string
	if err := s.pool.QueryRow(ctx, q, id).Scan(&u.ID, &u.Username, &role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.User{}, auth.ErrNotFound
		}
		return auth.User{}, fmt.Errorf("user by id: %w", err)
	}
	u.Role = auth.Role(role)
	return u, nil
}

func (s *UserStore) PasswordHashByID(ctx context.Context, id int64) (string, error) {
	const q = `SELECT password_hash FROM users WHERE id = $1`
	var hash string
	if err := s.pool.QueryRow(ctx, q, id).Scan(&hash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", auth.ErrNotFound
		}
		return "", fmt.Errorf("password hash by id: %w", err)
	}
	return hash, nil
}

// CreateUser создаёт сотрудника; заглушка на уникальность имени.
func (s *UserStore) CreateUser(ctx context.Context, username, passwordHash string, role auth.Role) (auth.User, error) {
	const q = `INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3) RETURNING id, username, role`

	var u auth.User
	var outRole string
	if err := s.pool.QueryRow(ctx, q, username, passwordHash, string(role)).
		Scan(&u.ID, &u.Username, &outRole); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return auth.User{}, auth.ErrUsernameTaken
		}
		return auth.User{}, fmt.Errorf("create user: %w", err)
	}
	u.Role = auth.Role(outRole)
	return u, nil
}

func (s *UserStore) ListUsers(ctx context.Context) ([]auth.User, error) {
	const q = `SELECT id, username, role FROM users ORDER BY id`

	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]auth.User, 0, 8)
	for rows.Next() {
		var u auth.User
		var role string
		if err := rows.Scan(&u.ID, &u.Username, &role); err != nil {
			return nil, fmt.Errorf("list users scan: %w", err)
		}
		u.Role = auth.Role(role)
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *UserStore) SetRole(ctx context.Context, id int64, role auth.Role) error {
	const q = `UPDATE users SET role = $1 WHERE id = $2`
	tag, err := s.pool.Exec(ctx, q, string(role), id)
	if err != nil {
		return fmt.Errorf("set role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound
	}
	return nil
}

func (s *UserStore) SetBlocked(ctx context.Context, id int64, blocked bool) error {
	const q = `UPDATE users SET blocked = $1 WHERE id = $2`
	tag, err := s.pool.Exec(ctx, q, blocked, id)
	if err != nil {
		return fmt.Errorf("set blocked: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound
	}
	return nil
}

func (s *UserStore) SetPasswordHash(ctx context.Context, id int64, hash string) error {
	const q = `UPDATE users SET password_hash = $1 WHERE id = $2`
	tag, err := s.pool.Exec(ctx, q, hash, id)
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound
	}
	return nil
}

var (
	_ store.UserStore      = (*UserStore)(nil)
	_ store.UserAdminStore = (*UserStore)(nil)
)
