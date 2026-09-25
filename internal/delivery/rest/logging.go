package rest

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// routePattern возвращает шаблон маршрута (например /api/v1/cells/{cellID}/products),
// чтобы метрики и логи не плодили серии на каждый конкретный id.
func routePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
		return rc.RoutePattern()
	}
	return r.URL.Path
}

// RequestLogger пишет структурированный access-лог с request-id и шаблоном пути.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		attrs := []any{
			"request_id", chimiddleware.GetReqID(r.Context()),
			"method", r.Method,
			"path", routePattern(r),
			"status", rec.status,
			"duration_ms", float64(time.Since(start).Microseconds()) / 1000.0,
			"bytes", rec.bytes,
		}
		switch {
		case rec.status >= 500:
			slog.Error("http request", attrs...)
		case rec.status >= 400:
			slog.Warn("http request", attrs...)
		default:
			slog.Info("http request", attrs...)
		}
	})
}

// Recoverer перехватывает паники и возвращает JSON-ответ вместо текстового 500.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"request_id", chimiddleware.GetReqID(r.Context()),
					"error", err,
					"stack", string(debug.Stack()),
				)
				writeErr(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
