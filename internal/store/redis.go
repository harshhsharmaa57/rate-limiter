package store

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
)

// slidingWindowScript is the Lua script that atomically implements sliding window rate limiting.
// YOUR TASK: fill in the Lua script body.
// The script receives:
//
//	KEYS[1] — the Redis key (e.g., "rl:free-tier:user:harsh123")
//	ARGV[1] — current timestamp in milliseconds (as a string, use tonumber())
//	ARGV[2] — window size in milliseconds
//	ARGV[3] — the limit (max requests)
//	ARGV[4] — a unique request ID string
//
// The script must:
//  1. ZREMRANGEBYSCORE KEYS[1] 0 (ARGV[1] - ARGV[2])  ← remove old entries
//  2. ZCARD KEYS[1]                                      ← count current entries
//  3. If count < limit:
//     ZADD KEYS[1] ARGV[1] ARGV[4]     ← record this request with timestamp as score
//     PEXPIRE KEYS[1] ARGV[2]           ← expire the key after the window
//     return {1, count + 1}             ← {allowed=true, new count}
//  4. Else:
//     return {0, count}                 ← {allowed=false, current count}
//
// IMPORTANT: In Lua, you must convert string args to numbers with tonumber()
// IMPORTANT: To call Redis: redis.call('COMMAND', arg1, arg2, ...)
const slidingWindowScript = `
-- Everything in here runs as ONE atomic operation on Redis
local key    = KEYS[1]           -- the sorted set key
local now    = tonumber(ARGV[1]) -- current timestamp in ms
local window = tonumber(ARGV[2]) -- window size in ms
local limit  = tonumber(ARGV[3]) -- max requests allowed
local req_id = ARGV[4]           -- unique ID for this request

-- Step 1: remove entries older than the window
redis.call('ZREMRANGEBYSCORE', key, 0, now - window)

-- Step 2: count current entries in the window
local count = tonumber(redis.call('ZCARD', key))

-- Step 3: decide
if count < limit then
    redis.call('ZADD', key, now, req_id)  -- record this request
    redis.call('PEXPIRE', key, window)     -- auto-delete the key after the window
    return {1, count + 1}                  -- allowed, new count
end

return {0, count}  -- rejected, current count
`

// RedisStore implements Store using Redis with a sliding window algorithm.
type RedisStore struct {
	client *redis.Client
	script *redis.Script // pre-compiled Lua script (faster than sending the string each time)
}

// NewRedisStore creates a RedisStore connected to the given address.
func NewRedisStore(addr string) *RedisStore {
	client := redis.NewClient(&redis.Options{
		Addr:         addr, // e.g., "localhost:6379"
		PoolSize:     20,   // max connections in pool
		MinIdleConns: 5,    // keep 5 connections always open
		DialTimeout:  5 * time.Second,
		ReadTimeout:  2 * time.Second,
	})

	return &RedisStore{
		client: client,
		script: redis.NewScript(slidingWindowScript),
		// redis.NewScript computes the SHA1 of the script
		// On first Run(), it sends the script to Redis and caches it by SHA
		// Subsequent runs just send the SHA — faster
	}
}

// Increment implements Store.Increment using the sliding window Lua script.
// YOUR TASK: implement this.
//
// Steps:
//  1. Get current time: now := time.Now().UnixMilli()
//  2. Generate unique request ID: reqID := fmt.Sprintf("%d-%d", now, rand.Int63())
//  3. Run the script:
//     result, err := rs.script.Run(ctx, rs.client,
//     []string{key},                             ← KEYS
//     now, window.Milliseconds(), limit, reqID,  ← ARGV[1..4]
//     ).Slice()
//  4. If err != nil: return false, 0, fmt.Errorf("redis script: %w", err)
//  5. The result is []interface{} with two elements:
//     result[0].(int64) == 1 means allowed
//     result[1].(int64) is the count
//  6. Return (allowed, count, nil)
func (rs *RedisStore) Increment(ctx context.Context, key string, window time.Duration, limit int64) (bool, int64, error) {
	// YOUR CODE HERE
	now := time.Now().UnixMilli()
	reqID := fmt.Sprintf("%d-%d", now, rand.Int63())
	result, err := rs.script.Run(ctx, rs.client,
		[]string{key},
		now, window.Milliseconds(), limit, reqID,
	).Slice()

	if err !=nil {
		return false, 0, fmt.Errorf("redis script: %w", err)
	}

	allowed := result[0].(int64) == 1
	count := result[1].(int64)
	return allowed, count, nil
}

// Reset implements Store.Reset by deleting the key from Redis.
// YOUR TASK: implement this.
// Use: rs.client.Del(ctx, key).Err()
func (rs *RedisStore) Reset(ctx context.Context, key string) error {
	// YOUR CODE HERE
	err := rs.client.Del(ctx, key).Err()

	if err != nil {
		return fmt.Errorf("Error deleting req: %w", err)
	}

	return nil
}
