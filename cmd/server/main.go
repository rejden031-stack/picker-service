package main

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"picker-service/internal/adapters/authimpl"
	adaptercache "picker-service/internal/adapters/cache"
	"picker-service/internal/adapters/postgres"
	"picker-service/internal/adapters/ratelimit"
	auditapp "picker-service/internal/application/audit"
	authapp "picker-service/internal/application/auth"
	catalogapp "picker-service/internal/application/catalog"
	"picker-service/internal/application/locations"
	cacheport "picker-service/internal/application/ports/cache"
	"picker-service/internal/application/useradmin"
	"picker-service/internal/delivery/rest"
	"picker-service/internal/domain/auth"
	"picker-service/internal/infra/config"
)

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func main() {
	cfg := config.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLogLevel(cfg.LogLevel)})))

	if cfg.JWTSecret == "dev-secret-change-me" {
		slog.Warn("используется дефолтный JWT_SECRET — для прода задайте свой")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseDSN)
	if err != nil {
		slog.Error("connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	for i := 0; i < 15; i++ {
		if err := pool.Ping(ctx); err == nil {
			break
		}
		slog.Info("waiting for postgres...")
		time.Sleep(2 * time.Second)
	}

	var cachePort cacheport.Port = adaptercache.Noop{}
	var redisClient *redis.Client
	if cfg.RedisEnabled {
		redisOptions := &redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword}
		if cfg.RedisTLS {
			redisOptions.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		redisClient = redis.NewClient(redisOptions)
		cachePort = adaptercache.NewRedis(redisClient)
		slog.Info("redis cache enabled", "addr", cfg.RedisAddr, "tls", cfg.RedisTLS)
	}

	locationStore := postgres.NewLocationStore(pool)
	catalogStore := postgres.NewCatalogStore(pool)
	userStore := postgres.NewUserStore(pool)
	auditStore := postgres.NewAuditStore(pool)

	hasher := authimpl.NewPasswordHasher()
	tokenIssuer := authimpl.NewTokenIssuer(cfg.JWTSecret)

	login := authapp.NewLogin(userStore, hasher, tokenIssuer, auditStore)
	tree := locations.NewTree(locationStore)
	auditWriter := auditapp.NewWriter(auditStore, 4096)
	defer auditWriter.Close()
	listProducts := catalogapp.NewListCellProducts(catalogStore, cachePort, auditWriter)
	findCandidates := catalogapp.NewFindCandidates(catalogStore, cachePort, auditWriter)
	userAdmin := useradmin.NewUserAdmin(userStore, auditStore, hasher)
	changePassword := authapp.NewChangePassword(userStore, hasher, auditStore)

	bootstrapAdmin(ctx, cfg, userStore, hasher, auditStore)

	metrics := rest.NewMetrics(pool)

	handler := rest.NewHandler(login, tree, listProducts, findCandidates, tokenIssuer, userAdmin, changePassword)
	handler.ReadinessCheck = func(c context.Context) error {
		ctx, cancel := context.WithTimeout(c, 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			return err
		}
		if redisClient != nil {
			return redisClient.Ping(ctx).Err()
		}
		return nil
	}

	routerOpts := []rest.RouterOption{
		rest.WithLoginRateLimit(cfg.LoginRatePerSec, cfg.LoginBurst),
		rest.WithTrustedProxies(cfg.TrustedProxies),
	}
	if cfg.RedisEnabled {
		// распределённый лимитер: общий для всех реплик
		routerOpts = append(routerOpts, rest.WithLoginLimiter(
			ratelimit.New(redisClient, cfg.LoginRatePerSec, cfg.LoginBurst),
		))
	}
	if cfg.MetricsUser != "" {
		routerOpts = append(routerOpts, rest.WithMetricsBasicAuth(cfg.MetricsUser, cfg.MetricsPassword))
	}

	router := rest.NewRouter(handler, metrics, routerOpts...)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
}

// bootstrapAdmin создаёт первого администратора из env, если его ещё нет.
func bootstrapAdmin(ctx context.Context, cfg config.Config,
	users *postgres.UserStore, hasher *authimpl.PasswordHasher, auditStore *postgres.AuditStore) {
	if cfg.AdminUsername == "" || cfg.AdminPassword == "" {
		slog.Info("ADMIN_USERNAME/ADMIN_PASSWORD не заданы — стартовый админ не создаётся")
		return
	}

	creds, err := users.UserByUsername(ctx, cfg.AdminUsername)
	switch {
	case err == nil:
		if creds.User.Role != auth.RoleAdmin {
			if err := users.SetRole(ctx, creds.User.ID, auth.RoleAdmin); err != nil {
				slog.Error("bootstrap admin: set role", "error", err)
				os.Exit(1)
			}
			slog.Info("админ повышен", "username", cfg.AdminUsername)
		}
		if creds.Blocked {
			if err := users.SetBlocked(ctx, creds.User.ID, false); err != nil {
				slog.Error("bootstrap admin: unblock", "error", err)
				os.Exit(1)
			}
		}
	case errors.Is(err, auth.ErrNotFound):
		actor := auth.User{ID: 0, Username: "bootstrap"}
		u, err := useradmin.NewUserAdmin(users, auditStore, hasher).
			CreateUser(ctx, actor, cfg.AdminUsername, cfg.AdminPassword, auth.RoleAdmin)
		if err != nil {
			slog.Error("bootstrap admin: create", "error", err)
			os.Exit(1)
		}
		slog.Info("стартовый админ создан", "username", u.Username, "id", u.ID)
	default:
		slog.Error("bootstrap admin: lookup", "error", err)
		os.Exit(1)
	}
}
