package locations

import (
	"context"
	"errors"
	"testing"

	"picker-service/internal/domain/location"
)

type locationStoreStub struct {
	calls int
	got   [4]string
	err   error
}

func (s *locationStoreStub) ResolveCell(_ context.Context, floorCode, rackCode, shelfCode, cellCode string) (location.Cell, error) {
	s.calls++
	s.got = [4]string{floorCode, rackCode, shelfCode, cellCode}
	if s.err != nil {
		return location.Cell{}, s.err
	}
	return location.Cell{ID: 42, ShelfID: 7, Code: cellCode}, nil
}

func (s *locationStoreStub) Tree(_ context.Context) (location.Tree, error) {
	return location.Tree{}, nil
}

func (s *locationStoreStub) CellAddress(_ context.Context, _ int64) (location.Address, error) {
	return location.Address{}, nil
}

func TestResolveTrimsCodes(t *testing.T) {
	stub := &locationStoreStub{}
	uc := NewResolveCell(stub)

	cell, err := uc.Resolve(context.Background(), " 2 ", "A ", "01", " 07 ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.got != [4]string{"2", "A", "01", "07"} {
		t.Errorf("store received %v, want [2 A 01 07]", stub.got)
	}
	if cell.ID != 42 {
		t.Errorf("cell id = %d, want 42", cell.ID)
	}
}

func TestResolvePropagatesError(t *testing.T) {
	stub := &locationStoreStub{err: errors.New("db down")}
	uc := NewResolveCell(stub)

	_, err := uc.Resolve(context.Background(), "2", "A", "01", "07")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
