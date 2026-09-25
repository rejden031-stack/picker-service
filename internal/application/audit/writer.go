// Package audit — прикладной слой журнала аудита.
package audit

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"picker-service/internal/application/ports/store"
	"picker-service/internal/domain/audit"
)

// Poster — отправить событие без ожидания результата.
// Для логина и администрирования события пишутся синхронно (долговечность важнее),
// для просмотров каталога — через асинхронный Writer.
type Poster interface {
	Post(e audit.Entry)
}

// Writer — асинхронная запись аудита: канал + воркер.
// Не блокирует запросы, при переполнении — дропает с предупреждением (бэктпрессур).
type Writer struct {
	store store.AuditStore
	ch    chan audit.Entry
	wg    sync.WaitGroup
	once  sync.Once
}

func NewWriter(s store.AuditStore, buf int) *Writer {
	if buf <= 0 {
		buf = 4096
	}
	w := &Writer{store: s, ch: make(chan audit.Entry, buf)}
	w.wg.Add(1)
	go w.run()
	return w
}

func (w *Writer) run() {
	defer w.wg.Done()
	for e := range w.ch {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := w.store.Append(ctx, e); err != nil {
			slog.Warn("audit append", "error", err, "action", e.Action)
		}
		cancel()
	}
}

// Post кладёт событие в очередь; не ждёт записи.
func (w *Writer) Post(e audit.Entry) {
	select {
	case w.ch <- e:
	default:
		slog.Warn("audit buffer full, event dropped", "action", e.Action)
	}
}

// Close останавливает воркера и доосвобождает буфер.
func (w *Writer) Close() {
	w.once.Do(func() {
		close(w.ch)
		w.wg.Wait()
	})
}

var _ Poster = (*Writer)(nil)
