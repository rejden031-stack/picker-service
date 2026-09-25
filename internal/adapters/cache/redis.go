package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis — кэш в Redis: разделяется между всеми подами сервиса.
type Redis struct {
	client *redis.Client
}

func NewRedis(client *redis.Client) *Redis {
	return &Redis{client: client}
}

func (r *Redis) Get(ctx context.Context, key string) (string, bool) {
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return "", false
	}
	return val, true
}

func (r *Redis) Set(ctx context.Context, key, value string, ttl time.Duration) {
	r.client.Set(ctx, key, value, ttl)
}

func (r *Redis) Delete(ctx context.Context, key string) {
	r.client.Del(ctx, key)
}

func (r *Redis) DeletePrefix(ctx context.Context, prefix string) {
	iter := r.client.Scan(ctx, 0, prefix+"*", 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if len(keys) > 0 {
		r.client.Del(ctx, keys...)
	}
}
