package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dir := flag.String("path", "migrations", "папка с миграциями")
	flag.Parse()

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

	files, err := filepath.Glob(filepath.Join(*dir, "*.up.sql"))
	if err != nil {
		slog.Error("list migrations", "error", err)
		os.Exit(1)
	}
	sort.Strings(files)

	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		slog.Error("create schema_migrations", "error", err)
		os.Exit(1)
	}

	re := regexp.MustCompile(`^([0-9]+)_`)
	for _, file := range files {
		base := filepath.Base(file)
		m := re.FindStringSubmatch(base)
		if m == nil {
			continue
		}
		version := m[1]

		var applied bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version).
			Scan(&applied); err != nil {
			slog.Error("check migration", "version", version, "error", err)
			os.Exit(1)
		}
		if applied {
			continue
		}

		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			slog.Error("read migration", "file", file, "error", err)
			os.Exit(1)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			slog.Error("begin tx", "file", file, "error", err)
			os.Exit(1)
		}
		if _, err := tx.Conn().PgConn().Exec(ctx, string(sqlBytes)).ReadAll(); err != nil {
			_ = tx.Rollback(ctx)
			slog.Error("apply migration", "file", file, "error", err)
			os.Exit(1)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
			_ = tx.Rollback(ctx)
			slog.Error("record migration", "file", file, "error", err)
			os.Exit(1)
		}
		if err := tx.Commit(ctx); err != nil {
			slog.Error("commit migration", "file", file, "error", err)
			os.Exit(1)
		}
		slog.Info("applied", "file", file)
	}
}
