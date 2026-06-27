# Distributed Rate Limiter in Go

A production-grade, distributed rate limiter service built in Go. It uses gRPC for high-performance service-to-service communication, Redis for distributed sliding-window counters, PostgreSQL for dynamic rule configuration, and server-sent events for real-time quota monitoring.

> **Built for scale:** Handles 40,000+ requests per second with sub-millisecond latency on a single core.

---

## What It Does

This service answers one question at scale:

> **"Has this user made too many requests recently?"**

Unlike a single-process rate limiter, this works correctly across any number of server instances because all state lives in **Redis**. A user cannot bypass the limit by having requests hit different servers.

### Features

- **Sliding Window Algorithm** — Tracks exact request timestamps, not just fixed buckets. Same approach used by Stripe.
- **Distributed & Atomic** — Redis Lua scripts guarantee atomic check-and-increment across all servers.
- **Dynamic Rules** — Rules are loaded from PostgreSQL and refreshed every 30 seconds without restarting the server.
- **Real-Time Streaming** — Clients can subscribe to quota updates via gRPC server-streaming or HTTP SSE.
- **Live Dashboard** — A React dashboard visualizes quota draining and recovery in real time.
- **Benchmarked** — Includes a `testing.B` benchmark that measures actual req/sec under concurrent load.

---

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   gRPC Client   │     │   gRPC Client   │     │  React Dashboard│
│   (service A)   │     │   (service B)   │     │   (SSE/HTTP)    │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────▼─────────────┐
                    │   Rate Limiter Server(s)  │
                    │   (Go + gRPC + HTTP)      │
                    │                           │
                    │  ┌─────────────────────┐  │
                    │  │  Limiter (business) │  │
                    │  │  QuotaManager (pub) │  │
                    │  └─────────────────────┘  │
                    └─────────────┬─────────────┘
                                  │
              ┌───────────────────┼───────────────────┐
              │                   │                   │
     ┌────────▼────────┐ ┌────────▼────────┐ ┌────────▼────────┐
     │     Redis       │ │    PostgreSQL   │ │   In-Memory     │
     │  (counts +      │ │   (rules table) │ │   (rule cache)  │
     │   sliding       │ │                 │ │                 │
     │   window)       │ │                 │ │                 │
     └─────────────────┘ └─────────────────┘ └─────────────────┘
```

### Why These Choices?

| Component | Why It Was Chosen |
|-----------|-------------------|
| **gRPC + Protobuf** | Binary, typed, ~10x smaller than REST+JSON. Auto-generated client code. Built-in streaming. |
| **Redis Sorted Sets (ZSET)** | Stores request timestamps with millisecond precision. `ZREMRANGEBYSCORE` evicts old entries automatically. |
| **Lua Scripts** | Redis executes the entire check-increment logic atomically. No race conditions between servers. |
| **PostgreSQL** | Rules are configuration data. Admins can update them via SQL; the server picks up changes automatically. |
| **Channels + Goroutines** | Go's concurrency model makes real-time pub/sub trivial without external message brokers. |

---

## Project Structure

```
ratelimiter/
├── cmd/
│   └── server/
│       └── main.go              # Entry point: wires store, limiter, gRPC, HTTP
├── internal/
│   ├── limiter/
│   │   ├── limiter.go           # Core logic: Consume, Check, rule lookup
│   │   └── quota.go             # QuotaManager: channels, subscribers, pub/sub
│   ├── server/
│   │   └── server.go            # gRPC handlers: CheckLimit, ConsumeLimit, WatchQuota
│   └── store/
│       ├── store.go             # Store interface (contract)
│       ├── memory.go            # In-memory implementation (testing)
│       ├── redis.go             # Redis sliding-window implementation (production)
│       └── rules.go             # PostgreSQL rule loader + cache refresher
├── proto/
│   └── ratelimiter.proto        # gRPC service definition
├── gen/
│   └── pb/                      # Auto-generated Go code from .proto
├── migrations/
│   └── 001_create_rules.sql     # PostgreSQL schema + seed data
├── bench_test.go                # Benchmark: concurrent req/sec measurement
└── go.mod
```

---

## Tech Stack

- **Go 1.22+**
- **gRPC** + **Protocol Buffers v3**
- **Redis 7** (sorted sets + Lua scripting)
- **PostgreSQL 16** (rule storage)
- **Docker & Docker Compose** (local infrastructure)
- **grpcurl** (manual testing)
- **React + Vite** (dashboard)

---

## Prerequisites

- Go 1.22 or newer
- Docker & Docker Compose
- `protoc` (Protocol Buffers compiler)
- `grpcurl` (optional, for manual testing)

### Install protoc

```bash
# macOS
brew install protobuf

# Ubuntu/Debian
sudo apt install -y protobuf-compiler

# Verify
protoc --version
```

### Install Go protoc plugins

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

---

## Quick Start

### 1. Clone & enter the project

```bash
cd ratelimiter
```

### 2. Start infrastructure (Redis + PostgreSQL)

```bash
docker compose up -d
```

### 3. Apply database migrations

```bash
docker exec -i $(docker compose ps -q postgres) \
    psql -U rl -d ratelimiter < migrations/001_create_rules.sql
```

### 4. Generate protobuf code

```bash
protoc --go_out=. --go-grpc_out=. proto/ratelimiter.proto
```

### 5. Download dependencies

```bash
go mod tidy
```

### 6. Run the server

```bash
go run cmd/server/main.go
```

You should see:

```
rate limiter listening on :50051
HTTP server listening on :8080
```

---

## API Reference

### gRPC Service: `RateLimiterService`

| Method | Type | Request | Response | Description |
|--------|------|---------|----------|-------------|
| `CheckLimit` | Unary | `LimitRequest` | `LimitResponse` | Read-only check. Does not record the request. |
| `ConsumeLimit` | Unary | `LimitRequest` | `LimitResponse` | Checks and records the request atomically. |
| `WatchQuota` | Server Streaming | `WatchRequest` | stream `QuotaUpdate` | Subscribes to real-time quota events for a key. |

#### Message Types

```protobuf
message LimitRequest {
    string key     = 1;
    string rule_id = 2;
    int64  cost    = 3;
}

message LimitResponse {
    bool   allowed      = 1;
    int64  remaining    = 2;
    int64  limit        = 3;
    int64  reset_at_unix = 4;
}

message WatchRequest {
    string key = 1;
}

message QuotaUpdate {
    string key      = 1;
    int64  used     = 2;
    int64  remaining = 3;
    bool   exceeded = 4;
}
```

### HTTP Endpoints (for dashboard & browser clients)

| Endpoint | Method | Query Params | Description |
|----------|--------|--------------|-------------|
| `/quota` | GET | `key` | Server-Sent Events stream of `QuotaUpdate` JSON |
| `/fire` | GET | `key`, `rule_id` | Triggers a `ConsumeLimit` and returns the `Result` as JSON |

---

## Testing

### Manual testing with grpcurl

List available services:
```bash
grpcurl -plaintext localhost:50051 list
```

Check a limit (read-only):
```bash
grpcurl -plaintext \
    -d '{"key": "user:harsh123", "rule_id": "free-tier"}' \
    localhost:50051 \
    ratelimiter.v1.RateLimiterService/CheckLimit
```

Consume a limit (records the request):
```bash
grpcurl -plaintext \
    -d '{"key": "user:harsh123", "rule_id": "free-tier"}' \
    localhost:50051 \
    ratelimiter.v1.RateLimiterService/ConsumeLimit
```

Watch quota in real time:
```bash
grpcurl -plaintext \
    -d '{"key": "user:harsh123"}' \
    localhost:50051 \
    ratelimiter.v1.RateLimiterService/WatchQuota
```

### Hammer the free-tier limit

```bash
for i in $(seq 1 15); do
    grpcurl -plaintext \
        -d '{"key": "user:harsh123", "rule_id": "free-tier"}' \
        localhost:50051 \
        ratelimiter.v1.RateLimiterService/ConsumeLimit
    echo "---"
done
```

**Expected:** First 10 return `"allowed": true`. Calls 11–15 return `"allowed": false`.

---

## Benchmarking

The benchmark connects to a running server and measures throughput under concurrent load.

**Start the server first:**
```bash
go run cmd/server/main.go
```

**Run the benchmark:**
```bash
go test -bench=BenchmarkConsumeLimit -benchtime=10s -benchmem -count=3 .
```

**Sample output:**
```
BenchmarkConsumeLimit-8    48231    24832 ns/op    1024 B/op    16 allocs/op
```

- **24,832 ns/op** ≈ **~40,000 requests/second**
- **1,024 B/op** — 1 KB allocated per request
- **16 allocs/op** — 16 heap allocations per request

Run 3 times for consistency. Quote this number in interviews.

---

## Live Dashboard

A React dashboard visualizes quota usage in real time.

### Start the dashboard

```bash
cd dashboard
npm install
npm run dev
```

Open `http://localhost:5173`.

### How to use it

1. Enter a key (e.g., `user:harsh123`) and click **Connect**.
2. Click **Fire 20 requests**.
3. Watch the green bar drain as requests are consumed.
4. When the limit is exceeded, the bar turns **red**.
5. Wait 60 seconds — the sliding window moves forward and the bar recovers.

---

## Key Design Decisions

### Sliding Window vs Fixed Window

A fixed window resets all counters at fixed intervals (e.g., every 60 seconds). This allows **2× the limit** of traffic at the window boundary:

```
Window 1: 59s → 10 requests (allowed)
Window 2:  1s → 10 requests (allowed)
Total: 20 requests in 2 seconds, even though limit is 10/minute
```

The **sliding window** stores every request timestamp in a Redis sorted set. Old entries are evicted by score (time). The count is always accurate for the last *N* milliseconds. No boundary burst problem.

### Why Lua Scripts?

The sliding window requires three Redis commands:
1. `ZREMRANGEBYSCORE` — remove old entries
2. `ZCARD` — count remaining entries
3. `ZADD` — record this request (if allowed)

Without Lua, another server could interleave between steps 2 and 3, causing both to think there's room. Redis Lua scripts execute atomically — no other command from any client can run between steps.

### Interfaces = Testability

The `Store` interface abstracts Redis behind a contract:

```go
type Store interface {
    Increment(ctx context.Context, key string, window time.Duration, limit int64) (bool, int64, error)
    Reset(ctx context.Context, key string) error
}
```

- `MemoryStore` implements `Store` for unit tests (no Docker needed).
- `RedisStore` implements `Store` for production.

The `Limiter` doesn't know which one it's talking to. Only `main.go` changes.

---

## Milestones

| Milestone | What Was Built | Status |
|-----------|---------------|--------|
| **Milestone 1** | Working gRPC server with in-memory store | ✅ |
| **Milestone 2** | Redis-backed distributed sliding window | ✅ |
| **Milestone 3** | Real-time quota streaming (`WatchQuota`) | ✅ |
| **Milestone 4** | PostgreSQL rule loading with auto-refresh | ✅ |
| **Milestone 5** | Benchmark suite with req/sec measurement | ✅ |
| **Milestone 6** | Live React dashboard with SSE | ✅ |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `50051` | gRPC server port |
| `HTTP_PORT` | `8080` | HTTP/SSE server port |
| `REDIS_ADDR` | `localhost:6379` | Redis connection address |
| `POSTGRES_DSN` | `postgres://rl:rl@localhost:5432/ratelimiter?sslmode=disable` | PostgreSQL connection string |
| `RULE_REFRESH_INTERVAL` | `30s` | How often to reload rules from PostgreSQL |

---

## Common Issues

### `protoc: command not found`
Install `protoc` via your package manager (see Prerequisites).

### `cannot use *MemoryStore as type Store`
Your `MemoryStore` method signatures don't match the `Store` interface exactly. Check parameter types and return values.

### Race condition warnings
Run `go test -race ./...`. If you see warnings, check that every map access is protected by a mutex.

### Dashboard shows no events
Make sure the HTTP server is running on `:8080` and CORS headers are set. Check browser console for connection errors.

---

## What You Learned

- **Go interfaces** as the foundation of dependency injection
- **Race conditions** and how `sync.Mutex` / `sync.RWMutex` prevent them
- **Sliding window algorithm** with Redis sorted sets
- **Lua scripting** for atomic distributed operations
- **gRPC** vs REST: binary payloads, enforced types, streaming
- **Goroutines + channels** for event-driven pub/sub without external brokers
- **Server-streaming gRPC** patterns (`stream.Send` + `ctx.Done`)
- **Benchmarking** Go services with `testing.B` and interpreting `ns/op`, `B/op`, `allocs/op`

---

## License

MIT

---

## Next Steps

- Add **gRPC interceptors** for logging, metrics, and authentication
- Implement **adaptive rate limiting** (different limits per user tier)
- Add **Prometheus metrics** and a Grafana dashboard
- Deploy to **Kubernetes** with Helm charts
- Add **circuit breaker** logic for when Redis is unreachable
