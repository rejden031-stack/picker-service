package catalogapp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"picker-service/internal/domain/auth"
	"picker-service/internal/domain/catalog"
	"picker-service/internal/domain/location"
)

type candidatesStoreStub struct {
	data  map[int64][]catalog.Candidate
	calls int
	err   error
}

func (s *candidatesStoreStub) Candidates(_ context.Context, productID, _ int64) ([]catalog.Candidate, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.data[productID], nil
}

func (s *candidatesStoreStub) ProductsByCell(_ context.Context, _ int64) ([]catalog.Product, error) {
	return nil, nil
}

func (s *candidatesStoreStub) Import(_ context.Context, _ []catalog.Product, _ []catalog.Placement) error {
	return nil
}

type candidatesCacheStub struct {
	store map[string]string
}

func newCandidatesCacheStub() *candidatesCacheStub {
	return &candidatesCacheStub{store: map[string]string{}}
}

func (c *candidatesCacheStub) Get(_ context.Context, key string) (string, bool) {
	v, ok := c.store[key]
	return v, ok
}

func (c *candidatesCacheStub) Set(_ context.Context, key, value string, _ time.Duration) {
	c.store[key] = value
}
func (c *candidatesCacheStub) Delete(_ context.Context, key string) { delete(c.store, key) }

func (c *candidatesCacheStub) DeletePrefix(_ context.Context, prefix string) {
	for k := range c.store {
		if strings.HasPrefix(k, prefix) {
			delete(c.store, k)
		}
	}
}

func TestFindCandidatesFromStore(t *testing.T) {
	store := &candidatesStoreStub{data: map[int64][]catalog.Candidate{
		1: {
			{CellID: 7, Address: location.Address{FloorCode: "2", RackCode: "A", ShelfCode: "01", CellCode: "07"}, Qty: 3, IsPrimary: false},
			{CellID: 10, Address: location.Address{FloorCode: "2", RackCode: "C", ShelfCode: "01", CellCode: "01"}, Qty: 2, IsPrimary: true},
		},
	}}
	uc := NewFindCandidates(store, newCandidatesCacheStub(), auditNoop{})

	got, err := uc.Find(context.Background(), auth.User{}, 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Address.String() != "2.A.01.07" {
		t.Errorf("first address = %q, want 2.A.01.07", got[0].Address.String())
	}
	if got[1].CellID != 10 {
		t.Errorf("second cell = %d, want 10", got[1].CellID)
	}
}

func TestFindCandidatesUsesCache(t *testing.T) {
	store := &candidatesStoreStub{data: map[int64][]catalog.Candidate{
		3: {{CellID: 9, Address: location.Address{FloorCode: "1", RackCode: "A", ShelfCode: "01", CellCode: "02"}}},
	}}
	uc := NewFindCandidates(store, newCandidatesCacheStub(), auditNoop{})

	if _, err := uc.Find(context.Background(), auth.User{}, 3, 1); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := uc.Find(context.Background(), auth.User{}, 3, 1); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if store.calls != 1 {
		t.Errorf("store calls = %d, want 1", store.calls)
	}
}

func TestFindNoCandidates(t *testing.T) {
	uc := NewFindCandidates(&candidatesStoreStub{}, newCandidatesCacheStub(), auditNoop{})

	got, err := uc.Find(context.Background(), auth.User{}, 99, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestFindStoreError(t *testing.T) {
	uc := NewFindCandidates(&candidatesStoreStub{err: errors.New("db down")}, newCandidatesCacheStub(), auditNoop{})

	_, err := uc.Find(context.Background(), auth.User{}, 1, 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
