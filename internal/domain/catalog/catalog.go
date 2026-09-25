package catalog

import "picker-service/internal/domain/location"

// Product — товар склада.
type Product struct {
	ID      int64  `json:"id"`
	SKU     string `json:"sku"`
	Name    string `json:"name"`
	Barcode string `json:"barcode"`
}

// Placement — прописка товара в конкретной ячейке.
type Placement struct {
	ID        int64 `json:"id"`
	ProductID int64 `json:"product_id"`
	CellID    int64 `json:"cell_id"`
	Qty       int   `json:"qty"`
	IsPrimary bool  `json:"is_primary"`
}

// Candidate — «где ещё может лежать товар»: ячейка + её полный адрес.
type Candidate struct {
	CellID    int64
	Address   location.Address
	Qty       int
	IsPrimary bool
}
