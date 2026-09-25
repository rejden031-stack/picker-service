package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"picker-service/internal/domain/location"
)

// LocationStore — реализация порта store.LocationStore на PostgreSQL.
type LocationStore struct {
	pool *pgxpool.Pool
}

func NewLocationStore(pool *pgxpool.Pool) *LocationStore {
	return &LocationStore{pool: pool}
}

// Tree возвращает весь склад вложенно: этажи → стеллажи → полки → ячейки.
func (s *LocationStore) Tree(ctx context.Context) (location.Tree, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT f.id, f.code, f.name,
		       r.id, r.floor_id, r.code,
		       sh.id, sh.rack_id, sh.code,
		       c.id, c.shelf_id, c.code
		FROM floors f
			JOIN racks r ON r.floor_id = f.id
			JOIN shelves sh ON sh.rack_id = r.id
			JOIN cells c ON c.shelf_id = sh.id
		ORDER BY f.id, r.id, sh.id, c.id`)
	if err != nil {
		return location.Tree{}, fmt.Errorf("tree: %w", err)
	}
	defer rows.Close()

	var tree location.Tree
	floorIdx := map[int64]int{}
	rackIdx := map[int64]int{}
	shelfIdx := map[int64]int{}

	for rows.Next() {
		var f location.Floor
		var r location.Rack
		var sh location.Shelf
		var c location.Cell
		if err := rows.Scan(&f.ID, &f.Code, &f.Name,
			&r.ID, &r.FloorID, &r.Code,
			&sh.ID, &sh.RackID, &sh.Code,
			&c.ID, &c.ShelfID, &c.Code); err != nil {
			return location.Tree{}, fmt.Errorf("scan tree row: %w", err)
		}

		fi, ok := floorIdx[f.ID]
		if !ok {
			tree.Floors = append(tree.Floors, location.FloorNode{Floor: f})
			fi = len(tree.Floors) - 1
			floorIdx[f.ID] = fi
		}
		ri, ok := rackIdx[r.ID]
		if !ok {
			tree.Floors[fi].Racks = append(tree.Floors[fi].Racks, location.RackNode{Rack: r})
			ri = len(tree.Floors[fi].Racks) - 1
			rackIdx[r.ID] = ri
		}
		si, ok := shelfIdx[sh.ID]
		if !ok {
			tree.Floors[fi].Racks[ri].Shelves = append(tree.Floors[fi].Racks[ri].Shelves, location.ShelfNode{Shelf: sh})
			si = len(tree.Floors[fi].Racks[ri].Shelves) - 1
			shelfIdx[sh.ID] = si
		}
		tree.Floors[fi].Racks[ri].Shelves[si].Cells = append(
			tree.Floors[fi].Racks[ri].Shelves[si].Cells,
			location.CellNode{Cell: c})
	}
	if err := rows.Err(); err != nil {
		return location.Tree{}, fmt.Errorf("iterate tree rows: %w", err)
	}
	return tree, nil
}

// ResolveCell находит ячейку по цепочке кодов.
func (s *LocationStore) ResolveCell(ctx context.Context, floorCode, rackCode, shelfCode, cellCode string) (location.Cell, error) {
	const q = `
		SELECT c.id, c.shelf_id, c.code
		FROM cells c
			JOIN shelves sh ON sh.id = c.shelf_id
			JOIN racks r ON r.id = sh.rack_id
			JOIN floors f ON f.id = r.floor_id
		WHERE f.code = $1 AND r.code = $2 AND sh.code = $3 AND c.code = $4`

	var cell location.Cell
	if err := s.pool.QueryRow(ctx, q, floorCode, rackCode, shelfCode, cellCode).
		Scan(&cell.ID, &cell.ShelfID, &cell.Code); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return location.Cell{}, location.ErrNotFound
		}
		return location.Cell{}, fmt.Errorf("resolve cell: %w", err)
	}
	return cell, nil
}

// CellAddress собирает полный адрес ячейки по её ID.
func (s *LocationStore) CellAddress(ctx context.Context, cellID int64) (location.Address, error) {
	const q = `
		SELECT f.code, r.code, sh.code, c.code
		FROM cells c
			JOIN shelves sh ON sh.id = c.shelf_id
			JOIN racks r ON r.id = sh.rack_id
			JOIN floors f ON f.id = r.floor_id
		WHERE c.id = $1`

	var a location.Address
	if err := s.pool.QueryRow(ctx, q, cellID).
		Scan(&a.FloorCode, &a.RackCode, &a.ShelfCode, &a.CellCode); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return location.Address{}, location.ErrNotFound
		}
		return location.Address{}, fmt.Errorf("cell address: %w", err)
	}
	return a, nil
}
