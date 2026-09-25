package locations

import (
	"context"

	"picker-service/internal/application/ports/store"
	"picker-service/internal/domain/location"
)

// Tree — сценарий «показать весь склад для форм выбора».
type Tree struct {
	locations store.LocationStore
}

func NewTree(locations store.LocationStore) *Tree {
	return &Tree{locations: locations}
}

func (uc *Tree) Get(ctx context.Context) (location.Tree, error) {
	return uc.locations.Tree(ctx)
}
