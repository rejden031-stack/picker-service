package config

import (
	"os"
	"strconv"
	"strings"
)

// Config — настройки сервиса из переменных окружения.
type Config struct {
	HTTPAddr        string
	DatabaseDSN     string
	JWTSecret       string
	RedisEnabled    bool
	RedisAddr       string
	RedisPassword   string
	RedisTLS        bool
	LogLevel        string
	LoginRatePerSec int
	LoginBurst      int
	TrustedProxies  []string
	MetricsUser     string
	MetricsPassword string
	AdminUsername   string
	AdminPassword   string
}

func Load() Config {
	redisAddr := envOr("REDIS_ADDR", "localhost:6379")
	redisTLS := strings.HasPrefix(redisAddr, "redis+tls://")
	if redisTLS {
		redisAddr = strings.TrimPrefix(redisAddr, "redis+tls://")
	}
	return Config{
		HTTPAddr:        envOr("HTTP_ADDR", ":8000"),
		DatabaseDSN:     envOr("DATABASE_DSN", "postgres://picker:picker@localhost:5433/picker?sslmode=disable"),
		JWTSecret:       envOr("JWT_SECRET", "dev-secret-change-me"),
		RedisEnabled:    envBool("REDIS_ENABLED"),
		RedisAddr:       redisAddr,
		RedisPassword:   envOr("REDIS_PASSWORD", ""),
		RedisTLS:        redisTLS,
		LogLevel:        envOr("LOG_LEVEL", "info"),
		LoginRatePerSec: envInt("LOGIN_RATE_PER_SEC", 3),
		LoginBurst:      envInt("LOGIN_RATE_BURST", 6),
		TrustedProxies:  envList("TRUSTED_PROXIES"),
		MetricsUser:     envOr("METRICS_USER", ""),
		MetricsPassword: envOr("METRICS_PASSWORD", ""),
		AdminUsername:   envOr("ADMIN_USERNAME", ""),
		AdminPassword:   envOr("ADMIN_PASSWORD", ""),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string) bool {
	return os.Getenv(key) == "true" || os.Getenv(key) == "1"
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envList(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
