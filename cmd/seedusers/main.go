package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"picker-service/internal/adapters/authimpl"
	"picker-service/internal/domain/auth"
)

func main() {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		dsn = "postgres://picker:picker@localhost:5433/picker?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		slog.Error("connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	hasher := authimpl.NewPasswordHasher()

	users := []struct {
		username string
		password string
		role     auth.Role
	}{
		{"senior", "123456", auth.RoleSenior},
		{"worker", "123456", auth.RoleWorker},
	}

	if adminUser, adminPass := os.Getenv("ADMIN_USERNAME"), os.Getenv("ADMIN_PASSWORD"); adminUser != "" && adminPass != "" {
		users = append(users, struct {
			username string
			password string
			role     auth.Role
		}{adminUser, adminPass, auth.RoleAdmin})
	}

	for _, u := range users {
		hash, err := hasher.Hash(ctx, u.password)
		if err != nil {
			slog.Error("hash password", "username", u.username, "error", err)
			os.Exit(1)
		}
		const q = `
			INSERT INTO users (username, password_hash, role)
			VALUES ($1, $2, $3)
			ON CONFLICT (username)
			DO UPDATE SET password_hash = EXCLUDED.password_hash, role = EXCLUDED.role`
		if _, err := pool.Exec(ctx, q, u.username, hash, string(u.role)); err != nil {
			slog.Error("insert user", "username", u.username, "error", err)
			os.Exit(1)
		}
		slog.Info("user ready", "username", u.username, "role", u.role)
	}
}
