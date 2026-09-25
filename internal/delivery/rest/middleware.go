package rest

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"picker-service/internal/domain/auth"
)

type contextKey string

const (
	userKey contextKey = "user"
	ipKey   contextKey = "ip"
)

// LoginLimiter — порт для rate-limit на /login.
// In-memory реализация живёт здесь; в проде подставляется распределённая на Redis.
type LoginLimiter interface {
	Allow(ctx context.Context, key string) bool
}

// loginLimitMiddleware оборачивает Login: считает попытки по реальному IP клиента
// и кладёт его в контекст (для журнала аудита).
func loginLimitMiddleware(l LoginLimiter, resolve IPResolver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := resolve(r)
		if !l.Allow(r.Context(), ip) {
			writeErr(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		ctx := context.WithValue(r.Context(), ipKey, ip)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// limiter — потокобезопасный token bucket по ключу (обычно IP).
type limiter struct {
	mu      sync.Mutex
	rate    float64 // токенов в секунду
	burst   float64
	state   map[string]*bucket
	maxKeys int
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newLimiter(rps, burst int) *limiter {
	if rps <= 0 {
		rps = 3
	}
	if burst <= 0 {
		burst = rps
	}
	return &limiter{
		rate:    float64(rps),
		burst:   float64(burst),
		state:   make(map[string]*bucket),
		maxKeys: 100_000,
	}
}

func (l *limiter) Allow(_ context.Context, key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.state[key]
	if !ok {
		// защита от неограниченного роста памяти: при переполнении очищаем всё
		// (в проде — распределённый лимитер на Redis, память не его проблема)
		if len(l.state) >= l.maxKeys {
			l.state = make(map[string]*bucket)
		}
		b = &bucket{tokens: l.burst, last: now}
		l.state[key] = b
	}

	b.tokens += now.Sub(b.last).Seconds() * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Auth проверяет пропуск и кладёт пользователя в контекст запроса.
func (h *Handler) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authz := r.Header.Get("Authorization")
		if !strings.HasPrefix(authz, "Bearer ") {
			writeErr(w, http.StatusUnauthorized, "missing token")
			return
		}

		u, err := h.tokenParser.Parse(r.Context(), strings.TrimPrefix(authz, "Bearer "))
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "invalid token")
			return
		}

		ctx := context.WithValue(r.Context(), userKey, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireSenior пропускает только роль senior.
func (h *Handler) RequireSenior(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := r.Context().Value(userKey).(auth.User)
		if !ok || u.Role != auth.RoleSenior {
			writeErr(w, http.StatusForbidden, "senior role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin пропускает только роль admin.
func (h *Handler) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := r.Context().Value(userKey).(auth.User)
		if !ok || u.Role != auth.RoleAdmin {
			writeErr(w, http.StatusForbidden, "admin role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// userFrom достаёт аутентифицированного пользователя из контекста запроса.
func userFrom(r *http.Request) auth.User {
	u, _ := r.Context().Value(userKey).(auth.User)
	return u
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write response", "error", err)
	}
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
