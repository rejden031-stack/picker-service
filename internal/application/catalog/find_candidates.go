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

// FindCandidates — сценарий «где ещё может лежать пропавший товар».
type FindCandidates struct {
	catalog store.CatalogStore
	cache   cache.Port
	audit   audit.Poster
}

func NewFindCandidates(catalog store.CatalogStore, cache cache.Port, audit audit.Poster) *FindCandidates {
	return &FindCandidates{catalog: catalog, cache: cache, audit: audit}
}

func (uc *FindCandidates) Find(ctx context.Context, actor auth.User, productID, excludeCellID int64) ([]catalog.Candidate, error) {
	key := "product-candidates:" + strconv.FormatInt(productID, 10) + ":" + strconv.FormatInt(excludeCellID, 10)

	if raw, ok := uc.cache.Get(ctx, key); ok {
		var candidates []catalog.Candidate
		if err := json.Unmarshal([]byte(raw), &candidates); err == nil {
			return candidates, nil
		}
	}

	candidates, err := uc.catalog.Candidates(ctx, productID, excludeCellID)
	if err != nil {
		return nil, err
	}

	if data, err := json.Marshal(candidates); err == nil {
		uc.cache.Set(ctx, key, string(data), 30*time.Second)
	}

	uc.audit.Post(auditdomain.Entry{
		ActorID: actor.ID, ActorName: actor.Username,
		Action: auditdomain.CandidatesViewed, EntityID: productID,
		Details: "product=" + strconv.FormatInt(productID, 10) +
			" exclude_cell=" + strconv.FormatInt(excludeCellID, 10),
	})
	return candidates, nil
}
