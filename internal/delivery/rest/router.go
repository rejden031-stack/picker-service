package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type routerConfig struct {
	loginRate    int
	loginBurst   int
	loginLimiter LoginLimiter
	ipResolver   IPResolver
	metricsUser  string
	metricsPass  string
}

// RouterOption — опция конфигурации роутера (DI без ломки сигнатуры).
type RouterOption func(*routerConfig)

// WithLoginRateLimit задаёт лимит попыток логина на один IP (rps, burst).
// Работает только если не передан явный лимитер через WithLoginLimiter.
func WithLoginRateLimit(rps, burst int) RouterOption {
	return func(c *routerConfig) {
		c.loginRate = rps
		c.loginBurst = burst
	}
}

// WithLoginLimiter подставляет распределённый лимитер (например, на Redis) —
// общий для всех инстансов. По умолчанию — in-memory per-инстанс.
func WithLoginLimiter(l LoginLimiter) RouterOption {
	return func(c *routerConfig) {
		c.loginLimiter = l
	}
}

// WithTrustedProxies задаёт сети (CIDR или IP) ingress/LB, чьим
// X-Forwarded-For / X-Real-IP можно доверять.
func WithTrustedProxies(trusted []string) RouterOption {
	return func(c *routerConfig) {
		c.ipResolver = NewIPResolver(trusted)
	}
}

// WithMetricsBasicAuth закрывает /metrics HTTP Basic Auth.
func WithMetricsBasicAuth(user, pass string) RouterOption {
	return func(c *routerConfig) {
		c.metricsUser = user
		c.metricsPass = pass
	}
}

// NewRouter собирает HTTP-приложение из сценариев.
func NewRouter(h *Handler, m *Metrics, opts ...RouterOption) http.Handler {
	cfg := routerConfig{loginRate: 3, loginBurst: 6}
	for _, o := range opts {
		o(&cfg)
	}

	resolve := cfg.ipResolver
	if resolve == nil {
		resolve = DefaultIPResolver()
	}

	lim := cfg.loginLimiter
	if lim == nil {
		lim = newLimiter(cfg.loginRate, cfg.loginBurst)
	}

	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(RequestLogger)
	r.Use(Recoverer)
	r.Use(m.Middleware)

	r.Handle("/metrics", basicAuth(cfg.metricsUser, cfg.metricsPass)(m.Handler()))
	r.Get("/healthz", h.Health)
	r.Get("/readyz", h.Ready)
	r.Post("/api/v1/auth/login", loginLimitMiddleware(lim, resolve, http.HandlerFunc(h.Login)).ServeHTTP)

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(h.Auth)
		r.Post("/auth/change-password", h.ChangePassword)
		r.Get("/locations/tree", h.Tree)
		r.Group(func(r chi.Router) {
			r.Use(h.RequireSenior)
			r.Get("/cells/{cellID}/products", h.CellProducts)
			r.Get("/products/{productID}/candidates", h.Candidates)
		})
		r.Group(func(r chi.Router) {
			r.Use(h.RequireAdmin)
			r.Get("/admin/users", h.ListUsers)
			r.Post("/admin/users", h.CreateUser)
			r.Patch("/admin/users/{userID}/role", h.SetUserRole)
			r.Patch("/admin/users/{userID}/blocked", h.SetUserBlocked)
			r.Post("/admin/users/{userID}/reset-password", h.ResetUserPassword)
			r.Get("/admin/audit", h.ListAudit)
		})
	})

	return r
}
