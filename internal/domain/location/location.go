package location

// Floor — этаж склада.
type Floor struct {
	ID   int64
	Code string
	Name string
}

// Rack — стеллаж на этаже.
type Rack struct {
	ID      int64
	FloorID int64
	Code    string
}

// Shelf — полка на стеллаже.
type Shelf struct {
	ID     int64
	RackID int64
	Code   string
}

// Cell — ячейка на полке. Атомарный адрес хранения.
type Cell struct {
	ID      int64
	ShelfID int64
	Code    string
}

// Address — полный адрес ячейки: этаж → стеллаж → полка → ячейка.
type Address struct {
	FloorCode string
	RackCode  string
	ShelfCode string
	CellCode  string
}

// String возвращает читаемое представление адреса, например: 2.A.03.14
func (a Address) String() string {
	return a.FloorCode + "." + a.RackCode + "." + a.ShelfCode + "." + a.CellCode
}
