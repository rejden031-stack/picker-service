package ratelimit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// TestRedisAllow — живой тест на настоящий Redis (skip без TEST_REDIS_ADDR).
func TestRedisAllow(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set TEST_REDIS_ADDR to run redis tests")
	}

	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: addr})
	defer client.Close()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("redis unavailable: %v", err)
	}

	token := "testbucket:" + time.Now().Format("150405.000000000")
	_ = client.Del(ctx, "rl:login:"+token).Err()
	t.Cleanup(func() { _ = client.Del(ctx, "rl:login:"+token).Err() })

	l := New(client, 1, 2)

	// burst=2: два подряд проходят, третий — нет.
	if !l.Allow(ctx, token) {
		t.Fatal("1st should pass (burst)")
	}
	if !l.Allow(ctx, token) {
		t.Fatal("2nd should pass (burst)")
	}
	if l.Allow(ctx, token) {
		t.Fatal("3rd must be rejected (burst exhausted)")
	}

	// rps=1: после ~1.2с снова разрешено.
	time.Sleep(1200 * time.Millisecond)
	if !l.Allow(ctx, token) {
		t.Fatal("after refill should pass")
	}
}
