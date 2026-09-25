package catalogapp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	auditdomain "picker-service/internal/domain/audit"
	"picker-service/internal/domain/auth"
	"picker-service/internal/domain/catalog"
)

// auditNoop вЂ” Poster-Р·Р°РіР»СѓС€РєР° РґР»СЏ С‚РµСЃС‚РѕРІ.
type auditNoop struct{}

func (auditNoop) Post(auditdomain.Entry) {}

type fakeCatalogStore struct {
	products   map[int64][]catalog.Product
	candidates map[int64][]catalog.Candidate
	calls      int
	err        error
}

func (s *fakeCatalogStore) ProductsByCell(_ context.Context, cellID int64) ([]catalog.Product, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.products[cellID], nil
}

func (s *fakeCatalogStore) Candidates(_ context.Context, _, _ int64) ([]catalog.Candidate, error) {
	if s.err != nil {
		return nil, s.err
	}
	return nil, nil
}

func (s *fakeCatalogStore) Import(_ context.Context, _ []catalog.Product, _ []catalog.Placement) error {
	return nil
}

type fakeCache struct {
	store map[string]string
}

func newFakeCache() *fakeCache { return &fakeCache{store: map[string]string{}} }

func (c *fakeCache) Get(_ context.Context, key string) (string, bool) {
	v, ok := c.store[key]
	return v, ok
}

func (c *fakeCache) Set(_ context.Context, key, value string, _ time.Duration) { c.store[key] = value }
func (c *fakeCache) Delete(_ context.Context, key string)                      { delete(c.store, key) }

func (c *fakeCache) DeletePrefix(_ context.Context, prefix string) {
	for k := range c.store {
		if strings.HasPrefix(k, prefix) {
			delete(c.store, k)
		}
	}
}

func TestListProductsFromStore(t *testing.T) {
	store := &fakeCatalogStore{products: map[int64][]catalog.Product{
		5: {
			{ID: 1, SKU: "SKU-1001", Name: "РџРѕРґС€РёРїРЅРёРє 6203", Barcode: "4600000000001"},
			{ID: 2, SKU: "SKU-1002", Name: "Р’С‚СѓР»РєР°", Barcode: "4600000000002"},
		},
	}}
	uc := NewListCellProducts(store, newFakeCache(), auditNoop{})

	got, err := uc.List(context.Background(), auth.User{}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].SKU != "SKU-1001" {
		t.Errorf("first sku = %q, want SKU-1001", got[0].SKU)
	}
}

func TestListEmptyCell(t *testing.T) {
	uc := NewListCellProducts(&fakeCatalogStore{}, newFakeCache(), auditNoop{})

	got, err := uc.List(context.Background(), auth.User{}, 9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestListUsesCache(t *testing.T) {
	store := &fakeCatalogStore{products: map[int64][]catalog.Product{
		1: {{ID: 1, SKU: "SKU-1001"}},
	}}
	uc := NewListCellProducts(store, newFakeCache(), auditNoop{})

	if _, err := uc.List(context.Background(), auth.User{}, 1); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := uc.List(context.Background(), auth.User{}, 1); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if store.calls != 1 {
		t.Errorf("store calls = %d, want 1 (second read from cache)", store.calls)
	}
}

func TestListStoreError(t *testing.T) {
	uc := NewListCellProducts(&fakeCatalogStore{err: errors.New("db down")}, newFakeCache(), auditNoop{})

	_, err := uc.List(context.Background(), auth.User{}, 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
