package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"picker-service/internal/domain/catalog"
)

// CatalogStore — реализация порта store.CatalogStore на PostgreSQL.
type CatalogStore struct {
	pool *pgxpool.Pool
}

func NewCatalogStore(pool *pgxpool.Pool) *CatalogStore {
	return &CatalogStore{pool: pool}
}

// ProductsByCell возвращает товары, прописанные в ячейке.
func (s *CatalogStore) ProductsByCell(ctx context.Context, cellID int64) ([]catalog.Product, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.sku, p.name, p.barcode
		FROM placements pl
			JOIN products p ON p.id = pl.product_id
		WHERE pl.cell_id = $1
		ORDER BY p.sku`, cellID)
	if err != nil {
		return nil, fmt.Errorf("products by cell: %w", err)
	}
	defer rows.Close()

	products := make([]catalog.Product, 0)
	for rows.Next() {
		var p catalog.Product
		if err := rows.Scan(&p.ID, &p.SKU, &p.Name, &p.Barcode); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate products: %w", err)
	}
	return products, nil
}

// Candidates возвращает другие адреса товара с полными адресами.
func (s *CatalogStore) Candidates(ctx context.Context, productID, excludeCellID int64) ([]catalog.Candidate, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, f.code, r.code, sh.code, c.code, pl.qty, pl.is_primary
		FROM placements pl
			JOIN cells c ON c.id = pl.cell_id
			JOIN shelves sh ON sh.id = c.shelf_id
			JOIN racks r ON r.id = sh.rack_id
			JOIN floors f ON f.id = r.floor_id
		WHERE pl.product_id = $1 AND pl.cell_id <> $2
		ORDER BY pl.is_primary DESC, f.code, r.code, sh.code, c.code`,
		productID, excludeCellID)
	if err != nil {
		return nil, fmt.Errorf("candidates: %w", err)
	}
	defer rows.Close()

	candidates := make([]catalog.Candidate, 0)
	for rows.Next() {
		var c catalog.Candidate
		if err := rows.Scan(&c.CellID,
			&c.Address.FloorCode, &c.Address.RackCode, &c.Address.ShelfCode, &c.Address.CellCode,
			&c.Qty, &c.IsPrimary); err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate candidates: %w", err)
	}
	return candidates, nil
}

// Import перезаписывает каталог целиком в одной транзакции.
func (s *CatalogStore) Import(ctx context.Context, products []catalog.Product, placements []catalog.Placement) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin import tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const upsertProduct = `
		INSERT INTO products (id, sku, name, barcode)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE
			SET sku = EXCLUDED.sku, name = EXCLUDED.name, barcode = EXCLUDED.barcode`
	for _, p := range products {
		if _, err := tx.Exec(ctx, upsertProduct, p.ID, p.SKU, p.Name, p.Barcode); err != nil {
			return fmt.Errorf("upsert product %d: %w", p.ID, err)
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM placements`); err != nil {
		return fmt.Errorf("clear placements: %w", err)
	}

	const upsertPlacement = `
		INSERT INTO placements (product_id, cell_id, qty, is_primary)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (product_id, cell_id)
		DO UPDATE SET qty = EXCLUDED.qty, is_primary = EXCLUDED.is_primary`
	for _, pl := range placements {
		if _, err := tx.Exec(ctx, upsertPlacement, pl.ProductID, pl.CellID, pl.Qty, pl.IsPrimary); err != nil {
			return fmt.Errorf("upsert placement %d->%d: %w", pl.ProductID, pl.CellID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit import: %w", err)
	}
	return nil
}
