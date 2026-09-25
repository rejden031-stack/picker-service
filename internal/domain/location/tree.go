package location

// Tree — «весь склад» целиком: этажи → стеллажи → полки → ячейки.
// Нужен для форм выбора адреса у старшего.
type Tree struct {
	Floors []FloorNode
}

type FloorNode struct {
	Floor
	Racks []RackNode
}

type RackNode struct {
	Rack
	Shelves []ShelfNode
}

type ShelfNode struct {
	Shelf
	Cells []CellNode
}

type CellNode struct {
	Cell
}
