package catalogapp

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"picker-service/internal/application/audit"
	"picker-service/internal/application/ports/cache"
	"picker-service/internal/application/ports/store"
	auditdomain "picker-service/internal/domain/audit"
	"picker-service/internal/domain/auth"
	"picker-service/internal/domain/catalog"
)

// ListCellProducts — сценарий «товары, прописанные в ячейке».
type ListCellProducts struct {
	catalog store.CatalogStore
	cache   cache.Port
	audit   audit.Poster
}

func NewListCellProducts(catalog store.CatalogStore, cache cache.Port, audit audit.Poster) *ListCellProducts {
	return &ListCellProducts{catalog: catalog, cache: cache, audit: audit}
}

func (uc *ListCellProducts) List(ctx context.Context, actor auth.User, cellID int64) ([]catalog.Product, error) {
	key := "cell-products:" + strconv.FormatInt(cellID, 10)

	if raw, ok := uc.cache.Get(ctx, key); ok {
		var products []catalog.Product
		if err := json.Unmarshal([]byte(raw), &products); err == nil {
			return products, nil
		}
	}

	products, err := uc.catalog.ProductsByCell(ctx, cellID)
	if err != nil {
		return nil, err
	}

	if data, err := json.Marshal(products); err == nil {
		uc.cache.Set(ctx, key, string(data), 30*time.Second)
	}

	uc.audit.Post(auditdomain.Entry{
		ActorID: actor.ID, ActorName: actor.Username,
		Action: auditdomain.ProductViewed, EntityID: cellID,
		Details: "cell=" + strconv.FormatInt(cellID, 10),
	})
	return products, nil
}
