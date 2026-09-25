package location

import "errors"

// ErrNotFound - ячейка/адрес не найдены в хранилище
var ErrNotFound = errors.New("location not found")
