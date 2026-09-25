package cache

import (
	"context"
	"time"
)

// Port — «разъём» кэша. Реализации: в памяти и Redis.
type Port interface {
	// Get возвращает значение по ключу; ok=false, если ключа нет.
	Get(ctx context.Context, key string) (string, bool)

	// Set кладёт значение с временем жизни.
	Set(ctx context.Context, key string, value string, ttl time.Duration)

	// Delete удаляет один ключ.
	Delete(ctx context.Context, key string)

	// DeletePrefix удаляет все ключи с заданным префиксом.
	DeletePrefix(ctx context.Context, prefix string)
}
