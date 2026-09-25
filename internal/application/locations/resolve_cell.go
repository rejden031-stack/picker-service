package locations

import (
	"context"
	"strings"

	"picker-service/internal/application/ports/store"
	"picker-service/internal/domain/location"
)

// ResolveCell — сценарий «найти ячейку по кодам: этаж → стеллаж → полка → ячейка».
type ResolveCell struct {
	locations store.LocationStore
}

func NewResolveCell(locations store.LocationStore) *ResolveCell {
	return &ResolveCell{locations: locations}
}

func (uc *ResolveCell) Resolve(ctx context.Context, floorCode, rackCode, shelfCode, cellCode string) (location.Cell, error) {
	return uc.locations.ResolveCell(ctx,
		strings.TrimSpace(floorCode),
		strings.TrimSpace(rackCode),
		strings.TrimSpace(shelfCode),
		strings.TrimSpace(cellCode))
}
