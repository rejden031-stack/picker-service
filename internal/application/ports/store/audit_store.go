package store

import (
	"context"

	"picker-service/internal/domain/audit"
)

// AuditStore — «разъём» для журнала аудита.
type AuditStore interface {
	// Append сохраняет событие синхронно (важно для логина и админа).
	Append(ctx context.Context, e audit.Entry) error
	// List возвращает последние события (сначала новые).
	List(ctx context.Context, limit, offset int) ([]audit.Entry, error)
}
