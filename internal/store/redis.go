package store

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
)

const slidingWindowScript = `
local key    = KEYS[1]
local now    = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit  = tonumber(ARGV[3])
local req_id = ARGV[4]

local cutoff = now - window
redis.call('ZREMRANGEBYSCORE', key, 0, cutoff)

local count = tonumber(redis.call('ZCARD', key))

if count < limit then
    redis.call('ZADD', key, now, req_id)
    redis.call('PEXPIRE', key, window)
    return {1, count + 1}
end

return {0, count}
`

// RedisStore implements Store using Redis with a sliding window algorithm.
type RedisStore struct {
	client *redis.Client
	script *redis.Script
}

// NewRedisStore creates a RedisStore connected to the given address.
func NewRedisStore(addr string) *RedisStore {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		PoolSize:     20,
		MinIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  2 * time.Second,
	})

	return &RedisStore{
		client: client,
		script: redis.NewScript(slidingWindowScript),
	}
}

// Increment implements Store.Increment using the sliding window Lua script.
func (rs *RedisStore) Increment(ctx context.Context, key string, window time.Duration, limit int64) (bool, int64, error) {
	now := time.Now().UnixMilli()
	reqID := fmt.Sprintf("%d-%d", now, rand.Int63())

	result, err := rs.script.Run(ctx, rs.client,
		[]string{key},
		now, window.Milliseconds(), limit, reqID,
	).Slice()

	if err != nil {
		return false, 0, fmt.Errorf("redis script: %w", err)
	}

	allowed := result[0].(int64) == 1
	count := result[1].(int64)

	return allowed, count, nil
}

// Reset implements Store.Reset by deleting the key from Redis.
func (rs *RedisStore) Reset(ctx context.Context, key string) error {
	return rs.client.Del(ctx, key).Err()
}
