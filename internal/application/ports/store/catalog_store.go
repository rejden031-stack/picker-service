package store

import (
	"context"

	"picker-service/internal/domain/catalog"
)

// CatalogStore — «разъём» для хранилища товаров и их прописки.
type CatalogStore interface {
	// ProductsByCell возвращает товары, прописанные в ячейке.
	ProductsByCell(ctx context.Context, cellID int64) ([]catalog.Product, error)

	// Candidates возвращает другие адреса товара:
	// все прописки, кроме ячейки excludeCellID, с полными адресами.
	Candidates(ctx context.Context, productID, excludeCellID int64) ([]catalog.Candidate, error)

	// Import заменяет каталог целиком: товары и прописки (одна транзакция).
	Import(ctx context.Context, products []catalog.Product, placements []catalog.Placement) error
}
