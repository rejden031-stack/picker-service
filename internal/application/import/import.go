package importcat

import (
	"context"

	"picker-service/internal/application/ports/cache"
	"picker-service/internal/application/ports/store"
	"picker-service/internal/domain/catalog"
)

// Import — сценарий «перезаписать каталог целиком».
type Import struct {
	catalog store.CatalogStore
	cache   cache.Port
}

func NewImport(catalog store.CatalogStore, cache cache.Port) *Import {
	return &Import{catalog: catalog, cache: cache}
}

// Run загружает товары и прописки, затем сбрасывает устаревший кэш.
func (uc *Import) Run(ctx context.Context, products []catalog.Product, placements []catalog.Placement) error {
	if err := uc.catalog.Import(ctx, products, placements); err != nil {
		return err
	}
	uc.cache.DeletePrefix(ctx, "cell-products:")
	uc.cache.DeletePrefix(ctx, "product-candidates:")
	return nil
}
