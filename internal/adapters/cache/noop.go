package cache

import (
	"context"
	"time"
)

// Noop — кэш-заглушка: клиент с этим кэшем всегда ходит в хранилище.
// Используется, когда кэш выключен.
type Noop struct{}

func (Noop) Get(_ context.Context, _ string) (string, bool)             { return "", false }
func (Noop) Set(_ context.Context, _ string, _ string, _ time.Duration) {}
func (Noop) Delete(_ context.Context, _ string)                         {}
func (Noop) DeletePrefix(_ context.Context, _ string)                   {}
