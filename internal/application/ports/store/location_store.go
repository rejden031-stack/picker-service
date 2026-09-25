package store

import (
	"context"

	"picker-service/internal/domain/location"
)

// LocationStore — «разъём» для хранилища схемы склада.
type LocationStore interface {
	// Tree возвращает весь склад: этажи → стеллажи → полки → ячейки.
	Tree(ctx context.Context) (location.Tree, error)

	// ResolveCell находит ячейку по цепочке кодов:
	// этаж → стеллаж → полка → ячейка.
	ResolveCell(ctx context.Context, floorCode, rackCode, shelfCode, cellCode string) (location.Cell, error)

	// CellAddress собирает полный адрес ячейки по её ID.
	CellAddress(ctx context.Context, cellID int64) (location.Address, error)
}
