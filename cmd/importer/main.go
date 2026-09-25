package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"picker-service/internal/adapters/cache"
	"picker-service/internal/adapters/postgres"
	importcat "picker-service/internal/application/import"
	"picker-service/internal/domain/catalog"
)

type catalogFile struct {
	Products   []catalog.Product   `json:"products"`
	Placements []catalog.Placement `json:"placements"`
}

func main() {
	file := flag.String("file", "testdata/seed/catalog.json", "путь к файлу каталога")
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

	data, err := os.ReadFile(*file)
	if err != nil {
		slog.Error("read catalog file", "error", err)
		os.Exit(1)
	}

	var cf catalogFile
	if err := json.Unmarshal(data, &cf); err != nil {
		slog.Error("parse catalog file", "error", err)
		os.Exit(1)
	}

	uc := importcat.NewImport(postgres.NewCatalogStore(pool), cache.Noop{})
	if err := uc.Run(ctx, cf.Products, cf.Placements); err != nil {
		slog.Error("import catalog", "error", err)
		os.Exit(1)
	}

	slog.Info("catalog imported", "products", len(cf.Products), "placements", len(cf.Placements))
}
