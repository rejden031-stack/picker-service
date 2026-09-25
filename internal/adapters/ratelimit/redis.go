// Package ratelimit — распределённый rate-limit на Redis, общий для всех инстансов.
package ratelimit

import (
	"context"
	"log/slog"
	"math"

	"github.com/redis/go-redis/v9"
)

// bucketScript — атомарный token bucket: refill по времени Redis (TIME),
// чтобы не полагаться на часы пода. Ключи живут максимум windowTTL —
// после простоя bucket "забывается", что эквивалентно полному ведру.
var bucketScript = redis.NewScript(`
local key = KEYS[1]
local now = redis.call('TIME')
local nowF = tonumber(now[1]) + tonumber(now[2]) / 1000000
local rate = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])
local prev = redis.call('GET', key)
local tokens, last
if prev then
  local data = cjson.decode(prev)
  tokens = data[1]
  last = data[2]
else
  tokens = burst
  last = nowF
end
tokens = math.min(burst, tokens + (nowF - last) * rate)
local ok = 0
if tokens >= 1 then
  tokens = tokens - 1
  ok = 1
end
redis.call('SET', key, cjson.encode({tokens, nowF}), 'EX', ttl)
return ok
`)

// Redis — token bucket в Redis: значения и арифметика общие для всех подов.
type Redis struct {
	client redis.UniversalClient
	rate   float64
	burst  float64
	window int64
	script *redis.Script
}

// New создаёт лимитер: rps токенов в секунду, максимум burst подряд.
func New(client redis.UniversalClient, rps, burst int) *Redis {
	if rps <= 0 {
		rps = 3
	}
	if burst <= 0 {
		burst = rps
	}
	// TTL = время на полный refill из нуля + запас; простой делает ключ
	// "полным", так что его потеря безопасна.
	ttl := int64(math.Ceil(float64(burst)/float64(rps))) + 60
	return &Redis{
		client: client,
		rate:   float64(rps),
		burst:  float64(burst),
		window: ttl,
		script: bucketScript,
	}
}

func (l *Redis) Allow(ctx context.Context, key string) bool {
	res, err := l.script.Run(ctx, l.client, []string{"rl:login:" + key},
		l.rate, l.burst, l.window).Int()
	if err != nil {
		// Redis недоступен — не валим login: fail-closed отдал бы 429 всем,
		// fail-open пропустит брутфорс. Выбираем fail-open с тревогой в лог.
		slog.Warn("rate limiter: redis unavailable, falling back to allow",
			"error", err, "key", key)
		return true
	}
	return res == 1
}
