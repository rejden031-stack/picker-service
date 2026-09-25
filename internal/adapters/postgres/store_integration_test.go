package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"picker-service/internal/domain/auth"
)

func TestStoresIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration in -short mode")
	}
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to run integration tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000_000)
	floorCode, rackCode, shelfCode := "F"+suffix, "R"+suffix, "S"+suffix
	cell1Code, cell2Code := "C1-"+suffix, "C2-"+suffix
	sku := "SKU-" + suffix

	floorID := insertRow(t, pool, `INSERT INTO floors (code, name) VALUES ($1, '') RETURNING id`, floorCode)
	rackID := insertRow(t, pool, `INSERT INTO racks (floor_id, code) VALUES ($1, $2) RETURNING id`, floorID, rackCode)
	shelfID := insertRow(t, pool, `INSERT INTO shelves (rack_id, code) VALUES ($1, $2) RETURNING id`, rackID, shelfCode)
	cell1ID := insertRow(t, pool, `INSERT INTO cells (shelf_id, code) VALUES ($1, $2) RETURNING id`, shelfID, cell1Code)
	cell2ID := insertRow(t, pool, `INSERT INTO cells (shelf_id, code) VALUES ($1, $2) RETURNING id`, shelfID, cell2Code)
	productID := insertRow(t, pool, `INSERT INTO products (sku, name, barcode) VALUES ($1, $2, $3) RETURNING id`,
		sku, "Тест", "B-"+suffix)

	if _, err := pool.Exec(ctx, `INSERT INTO placements (product_id, cell_id, qty, is_primary)
		VALUES ($1, $2, 2, true), ($1, $3, 1, false)`, productID, cell1ID, cell2ID); err != nil {
		t.Fatalf("insert placements: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM placements WHERE product_id = $1`, productID)
		_, _ = pool.Exec(ctx, `DELETE FROM products WHERE id = $1`, productID)
		_, _ = pool.Exec(ctx, `DELETE FROM cells WHERE id IN ($1, $2)`, cell1ID, cell2ID)
		_, _ = pool.Exec(ctx, `DELETE FROM shelves WHERE id = $1`, shelfID)
		_, _ = pool.Exec(ctx, `DELETE FROM racks WHERE id = $1`, rackID)
		_, _ = pool.Exec(ctx, `DELETE FROM floors WHERE id = $1`, floorID)
	})

	locationStore := NewLocationStore(pool)
	cell, err := locationStore.ResolveCell(ctx, floorCode, rackCode, shelfCode, cell1Code)
	if err != nil {
		t.Fatalf("ResolveCell: %v", err)
	}
	if cell.ID != cell1ID {
		t.Errorf("resolve cell id = %d, want %d", cell.ID, cell1ID)
	}

	addr, err := locationStore.CellAddress(ctx, cell2ID)
	if err != nil {
		t.Fatalf("CellAddress: %v", err)
	}
	expectedAddr := floorCode + "." + rackCode + "." + shelfCode + "." + cell2Code
	if addr.String() != expectedAddr {
		t.Errorf("address = %q, want %q", addr.String(), expectedAddr)
	}

	catalogStore := NewCatalogStore(pool)
	products, err := catalogStore.ProductsByCell(ctx, cell1ID)
	if err != nil {
		t.Fatalf("ProductsByCell: %v", err)
	}
	if len(products) != 1 || products[0].SKU != sku {
		t.Errorf("products = %+v, want one %s", products, sku)
	}

	candidates, err := catalogStore.Candidates(ctx, productID, cell1ID)
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	if len(candidates) != 1 || candidates[0].CellID != cell2ID {
		t.Errorf("candidates = %+v, want only cell %d", candidates, cell2ID)
	}

	userStore := NewUserStore(pool)
	testUser := "senior-" + suffix
	userID := insertRow(t, pool, `INSERT INTO users (username, password_hash, role) VALUES ($1, 'x', 'senior') RETURNING id`, testUser)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID) })

	creds, err := userStore.UserByUsername(ctx, testUser)
	if err != nil {
		t.Fatalf("UserByUsername: %v", err)
	}
	if creds.User.Role != auth.RoleSenior {
		t.Errorf("role = %q, want senior", creds.User.Role)
	}
}

func insertRow(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&id); err != nil {
		t.Fatalf("insert: %v", err)
	}
	return id
}
