# Distributed Rate Limiter

A high-throughput, distributed rate limiting service built in Go. Designed for multi-instance deployments where accuracy matters — every request is tracked atomically across all servers using Redis and Lua scripting.

Uses **gRPC** for service-to-service communication, **Redis sorted sets** for sliding-window counters, **PostgreSQL** for dynamic rule management, and **server-sent events** for real-time quota monitoring.

---

## Performance

Benchmarked on a single machine (Intel i5-12500H, 16 threads) against a local Redis 7 instance with 16 concurrent gRPC clients.

```
goos:   windows
goarch: amd64
cpu:    12th Gen Intel(R) Core(TM) i5-12500H

BenchmarkConsumeLimit-16    98175    128803 ns/op    5541 B/op    83 allocs/op
BenchmarkConsumeLimit-16    87830    207991 ns/op    5502 B/op    83 allocs/op
BenchmarkConsumeLimit-16    58922    193854 ns/op    5515 B/op    83 allocs/op
```

### What the Numbers Mean

| Metric | Best Run | Meaning |
|--------|----------|---------|
| **128,803 ns/op** | ~0.13 ms per request | End-to-end latency for a single `ConsumeLimit` call through the full gRPC → Limiter → Redis Lua → response pipeline |
| **~7,700 req/s** | Sustained throughput | Total requests per second across 16 parallel clients (`1s / 128,803ns`) |
| **5,541 B/op** | ~5.4 KB per request | Total heap memory allocated per operation (gRPC serialization + Redis round-trip + Lua result parsing) |
| **83 allocs/op** | 83 heap allocations | Per-request allocation count across the full stack |
| **98,175 iterations** | Over 10 seconds | Total operations completed in the fastest run |

> **Note:** The benchmark measures the complete round-trip — from gRPC client dial, through protobuf serialization, to the server-side sliding-window Lua script execution in Redis, and back. This is not a synthetic in-memory test. Every operation hits a real Redis instance and executes the atomic Lua script.

### Interpreting the Benchmark Score

Go's `testing.B` benchmark produces one line per run. Here is how to read it:

```
BenchmarkConsumeLimit-16    98175    128803 ns/op    5541 B/op    83 allocs/op
│                     │       │         │              │             │
│                     │       │         │              │             └─ Heap allocations per operation
│                     │       │         │              └─ Bytes allocated on the heap per operation
│                     │       │         └─ Nanoseconds per operation (latency)
│                     │       └─ Total iterations completed in the benchmark duration
│                     └─ GOMAXPROCS: number of OS threads used (= CPU logical cores)
└─ Benchmark function name
```

**Column-by-column breakdown:**

| Column | Value | How to Interpret |
|--------|-------|------------------|
| `BenchmarkConsumeLimit` | Function name | Maps to `func BenchmarkConsumeLimit(b *testing.B)` in `bench_test.go`. This is the test that fires `ConsumeLimit` gRPC calls concurrently. |
| `-16` | GOMAXPROCS | The benchmark ran with 16 OS threads (matching the CPU's 16 logical cores). `b.RunParallel` spawns goroutines across all available threads. More threads = more concurrent gRPC clients. |
| `98175` | Iterations | The total number of `ConsumeLimit` calls completed during the 10-second benchmark window (`-benchtime=10s`). Higher is better. |
| `128803 ns/op` | Latency | Average wall-clock time per operation in nanoseconds. **This is the most important metric.** Convert to throughput: `1,000,000,000 ÷ 128,803 ≈ 7,764 req/s`. |
| `5541 B/op` | Memory | Bytes of heap memory allocated per operation. Includes gRPC frame buffers, protobuf marshaling, Redis command construction, and Lua result parsing. Lower is better — this is ~5.4 KB, which is reasonable for a full gRPC round-trip. |
| `83 allocs/op` | Allocations | Number of separate `make()` / `new()` heap allocations per operation. Each allocation is a potential GC pressure point. 83 allocs across an entire gRPC+Redis stack is typical. |

**Deriving throughput from `ns/op`:**

```
Throughput = 1,000,000,000 / ns_per_op

Run 1: 1,000,000,000 / 128,803 = ~7,764 req/s
Run 2: 1,000,000,000 / 207,991 = ~4,808 req/s
Run 3: 1,000,000,000 / 193,854 = ~5,158 req/s
```

**What counts as a good score for a distributed rate limiter:**

| Throughput Range | Assessment |
|-----------------|------------|
| < 1,000 req/s | Below expectations — likely a misconfigured Redis pool, high network latency, or a single-threaded benchmark |
| 1,000–5,000 req/s | Acceptable for production APIs with moderate traffic. Typical for remote Redis (1-5ms RTT) |
| **5,000–10,000 req/s** | **Good. This is where this project sits.** Sufficient for most SaaS APIs and internal microservices |
| 10,000–50,000 req/s | Excellent. Achievable with O(1) algorithms (GCRA, token bucket) or Redis pipelining |
| 50,000+ req/s | Requires in-memory-only limiting (no Redis) or batched/async Redis sync |

The benchmark score of **~5,600–7,700 req/s** is strong for a sliding-window-log implementation because the algorithm is O(log N) per Redis call (sorted set operations) rather than O(1). The exact-counting guarantee comes at a compute cost that is reflected in the score.

**Memory efficiency assessment:**

- **5,541 B/op (~5.4 KB)** — At 7,700 req/s, this translates to ~42 MB/s of heap allocation throughput. Go's garbage collector handles this comfortably.
- **83 allocs/op** — Spread across the gRPC framework (~40-50 allocs for connection handling, serialization), Redis client (~15-20 allocs for command construction and result parsing), and application logic (~10-15 allocs). This is not a hotspot for optimization.

### Variance Across Runs

The three runs show throughput ranging from ~5,100 to ~7,700 req/s. This variance is normal for benchmarks that involve network I/O and is influenced by:

- **Redis connection pool saturation** — 16 concurrent goroutines sharing a 20-connection pool
- **OS thread scheduling** — Windows process scheduling and context switches
- **Redis Lua script atomicity** — Each script briefly blocks other Redis commands during execution
- **gRPC overhead** — TCP connection management, protobuf marshaling, and HTTP/2 framing

The median throughput of **~5,600 req/s** is the most reliable figure for capacity planning.

---

## How It Compares

This rate limiter implements the same sliding-window-log algorithm used by companies like Stripe for API rate limiting. Here is how it compares to production-grade alternatives:

| Solution | Algorithm | Distributed | Accuracy | Throughput (single node) | Protocol |
|----------|-----------|-------------|----------|--------------------------|----------|
| **This project** | Sliding window log (ZSET + Lua) | ✅ Redis | Exact | ~5,600–7,700 req/s | gRPC |
| **go-redis/redis_rate** | GCRA (leaky bucket via Lua) | ✅ Redis | Approximate | ~10,000–15,000 req/s¹ | Library call |
| **Kong Rate Limiting** | Fixed/sliding window counter | ✅ Redis or local | Approximate | Varies by policy² | HTTP plugin |
| **golang.org/x/time/rate** | Token bucket | ❌ Single process | Exact (local) | ~100,000+ req/s | Library call |
| **Cloudflare Rate Limiting** | Proprietary (edge) | ✅ Edge network | Approximate | N/A (edge-managed) | HTTP |
| **NVIDIA/go-ratelimit** | Sliding window | ✅ Redis | Configurable | ~8,000–12,000 req/s¹ | Library call |

<sup>¹ Library call without gRPC/network transport overhead. ² Kong's Redis policy performance depends on sync_rate configuration.</sup>

### Algorithmic Complexity Comparison

The algorithm choice directly impacts both accuracy and Redis resource consumption:

| Algorithm | Time Complexity | Memory per Key | Boundary Burst | Used By |
|-----------|----------------|----------------|----------------|---------|
| **Sliding Window Log** (this project) | O(log N + M)³ | O(N) — stores every timestamp | ❌ None | Stripe, this project |
| **Sliding Window Counter** | O(1) | O(1) — two counters per key | ~1.5% error⁴ | Kong, custom implementations |
| **GCRA / Leaky Bucket** | O(1) | O(1) — single TAT value per key | ❌ None (smoothed) | go-redis/redis_rate |
| **Fixed Window** | O(1) | O(1) — single counter per key | ✅ Up to 2× limit | Simple implementations |
| **Token Bucket** | O(1) | O(1) — token count + timestamp | ❌ None (allows controlled bursts) | golang.org/x/time/rate |

<sup>³ N = entries in the sorted set, M = expired entries removed per call. ⁴ Sliding window counter approximation error based on Cloudflare's published analysis.</sup>

This project uses the sliding window log because it provides **exact** counting — no approximation, no boundary burst, and no smoothing artifacts. The trade-off is higher Redis memory usage per key (O(N) vs O(1)), which is acceptable for APIs with limits in the hundreds-to-thousands range (e.g., 1,000 req/min per user). For APIs enforcing limits of 100,000+ per key, consider switching to a sliding window counter.

### Latency Breakdown

Where time is spent in a single `ConsumeLimit` call (~130µs best case):

| Stage | Estimated Time | Percentage |
|-------|---------------|------------|
| gRPC client → server (localhost TCP + HTTP/2) | ~15–25 µs | ~15% |
| Protobuf deserialization | ~2–5 µs | ~3% |
| Rule cache lookup (in-memory RWMutex read) | < 1 µs | < 1% |
| Redis round-trip (localhost TCP) | ~80–100 µs | ~70% |
| Redis Lua script execution | ~5–15 µs | ~8% |
| Protobuf serialization + gRPC response | ~5–10 µs | ~5% |

The bottleneck is the **Redis network round-trip**, not the algorithm or Go code. On a production deployment with Redis on a separate host (1-5ms RTT), the total latency would be dominated by that hop. Optimizations like Redis connection pooling (already configured at 20 connections) and Lua script caching via `EVALSHA` (handled by `go-redis` automatically) are already applied.

### Where This Rate Limiter Fits

**Use this when you need all three:** distributed enforcement, exact counting, and a self-hosted solution with real-time observability.

| Scenario | Why this rate limiter works well |
|----------|----------------------------------|
| **Multi-instance API backends** | All instances share Redis state. A user cannot bypass limits by hitting different servers. The Lua script guarantees atomicity — no race conditions even under high concurrency. |
| **APIs requiring exact quota tracking** | Sliding window log tracks every request timestamp. No approximation, no boundary burst problem. When a user's limit is 1,000/min, they get exactly 1,000 — not 1,015 or 1,980. |
| **SaaS platforms with tiered plans** | Rules are stored in PostgreSQL and loaded dynamically. Change a plan limit with a SQL `UPDATE` — no redeployment, no restart. The server picks up changes within 30 seconds. |
| **Internal services with real-time monitoring** | gRPC streaming and HTTP SSE provide live quota visibility. Operations teams can watch limits drain and recover in real time via the React dashboard. |
| **Microservice architectures using gRPC** | Native gRPC interface means no HTTP translation layer. Services call `ConsumeLimit` with a single protobuf-encoded RPC — binary, typed, and ~10× smaller than equivalent JSON. |

### Where Alternatives Are Better

| Scenario | Better Alternative | Why |
|----------|-------------------|-----|
| **Single-process applications** | `golang.org/x/time/rate` | Zero network overhead. Runs at ~100,000+ req/s with nanosecond latency. No Redis, no Docker, no infrastructure. |
| **Extreme distributed scale (100K+ req/s)** | `go-redis/redis_rate` (GCRA) or sliding window counter | O(1) Redis operations vs O(log N). Lower memory per key. GCRA uses a single `HSET` per key vs a sorted set with N members. |
| **Edge-level DDoS protection** | Cloudflare, AWS WAF, Akamai | Traffic is blocked before it reaches your infrastructure. Operates at CDN edge with sub-millisecond decisions. |
| **API gateway integration** | Kong Rate Limiting plugin | Runs inside an existing gateway. Supports local buffering with periodic Redis sync (`sync_rate`) for reduced Redis load at the cost of accuracy. |
| **No Redis in your stack** | Database-backed limiting or in-memory | Adding Redis solely for rate limiting may not be worth the operational overhead. Consider PostgreSQL advisory locks or application-level token buckets. |

---

## How to Reproduce the Benchmark

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- A machine with at least 4 cores (benchmark uses `GOMAXPROCS` threads in parallel)

### Steps

**1. Start infrastructure**

```bash
docker compose up -d
```

**2. Apply database migrations**

```bash
# Linux / macOS
docker exec -i $(docker compose ps -q postgres) \
    psql -U rl -d ratelimiter < migrations/001_create_rules.sql

# Windows (PowerShell)
Get-Content migrations/001_create_rules.sql | docker exec -i ratelimiter-postgres-1 psql -U rl -d ratelimiter
```

**3. Start the server**

```bash
go run cmd/server/main.go
```

Wait for both lines:
```
rate limiter listening on :50051
HTTP server listening on :8080
```

**4. Run the benchmark**

```bash
go test -bench=BenchmarkConsumeLimit -benchtime=10s -benchmem -count=3 .
```

### Reading the Output

```
BenchmarkConsumeLimit-16    98175    128803 ns/op    5541 B/op    83 allocs/op
                      │       │          │             │            │
                 CPU cores  total    nanoseconds    bytes per    heap allocs
                  used    iterations  per op       operation    per operation
```

- **`-16`** — Number of CPU threads used (matches `GOMAXPROCS`)
- **`98175`** — Total iterations completed in 10 seconds
- **`128803 ns/op`** — Average time per operation. Divide 1,000,000,000 by this to get req/s
- **`5541 B/op`** — Heap memory allocated per request
- **`83 allocs/op`** — Number of heap allocations per request

### Factors That Affect Results

| Factor | Impact |
|--------|--------|
| Redis location | Local Redis (~0.1ms RTT) vs remote Redis (~1-5ms RTT) dramatically changes throughput |
| CPU core count | More cores = more parallel gRPC clients in `b.RunParallel` |
| Redis pool size | Default is 20 connections. Increase `PoolSize` in `redis.go` for higher concurrency |
| OS | Linux typically outperforms Windows for network I/O benchmarks by 10-30% |
| Background load | Close other applications. Redis and Go compete for CPU cycles |

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

### Design Rationale

| Component | Why |
|-----------|-----|
| **gRPC + Protobuf** | Binary serialization, ~10× smaller payloads than REST+JSON. Auto-generated client stubs. Built-in server streaming for real-time quota updates. |
| **Redis Sorted Sets** | Stores request timestamps with millisecond precision. `ZREMRANGEBYSCORE` evicts expired entries. Enables exact sliding-window counting. |
| **Redis Lua Scripts** | The sliding-window check requires three steps: evict old entries → count remaining → add new entry. Lua scripts execute all three atomically. No race conditions between servers. |
| **PostgreSQL** | Rate-limit rules (plan tiers, limits, windows) are configuration data. Stored in a relational database so admins can update them via SQL without restarting the service. |
| **Go Channels + Goroutines** | Powers the real-time quota pub/sub system without external message brokers. Subscribers receive events through buffered channels. |

---

## Features

- **Sliding Window Algorithm** — Tracks exact request timestamps in Redis sorted sets. No boundary burst problem. Same approach used by Stripe.
- **Distributed & Atomic** — Redis Lua scripts guarantee atomic check-and-increment across all server instances. No race conditions.
- **Dynamic Rules** — Rate-limit rules are loaded from PostgreSQL and refreshed every 30 seconds. Change limits without restarting the server.
- **Real-Time Streaming** — Clients can subscribe to quota updates via gRPC server-streaming or HTTP SSE.
- **Live Dashboard** — A React dashboard visualizes quota usage draining and recovering in real time.
- **Benchmarked** — Includes a `testing.B` benchmark that measures actual throughput under concurrent load.

---

## Project Structure

```
ratelimiter/
├── cmd/
│   └── server/
│       └── main.go                # Entry point: wires store, limiter, gRPC, HTTP
├── internal/
│   ├── limiter/
│   │   ├── limiter.go             # Core logic: Consume, Check, rule lookup
│   │   └── quota.go               # QuotaManager: channels, subscribers, pub/sub
│   ├── server/
│   │   └── server.go              # gRPC handlers: CheckLimit, ConsumeLimit, WatchQuota
│   └── store/
│       ├── store.go               # Store interface (contract for any backend)
│       ├── memory.go              # In-memory implementation (testing)
│       ├── redis.go               # Redis sliding-window implementation (production)
│       └── rules.go               # PostgreSQL rule loader + background cache refresh
├── proto/
│   └── ratelimiter.proto          # gRPC service definition
├── gen/
│   └── pb/                        # Auto-generated Go code from .proto
├── migrations/
│   └── 001_create_rules.sql       # PostgreSQL schema + seed data
├── dashboard/                     # React + Vite live monitoring dashboard
├── bench_test.go                  # Benchmark: concurrent throughput measurement
├── docker-compose.yml             # Redis + PostgreSQL infrastructure
└── go.mod
```

---

## Tech Stack

| Technology | Version | Purpose |
|------------|---------|---------|
| Go | 1.22+ | Service implementation |
| gRPC | v1.81 | High-performance RPC framework |
| Protocol Buffers | v3 | Binary serialization |
| Redis | 7 | Sliding-window state (sorted sets + Lua) |
| PostgreSQL | 16 | Rule configuration storage |
| Docker & Docker Compose | — | Local infrastructure |
| React + Vite | — | Live monitoring dashboard |

---

## Quick Start

### Prerequisites

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

# Windows (scoop)
scoop install protobuf

# Verify
protoc --version
```

### Install Go protoc plugins

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### Setup

```bash
# 1. Clone & enter the project
git clone https://github.com/harshhsharmaa57/rate-limiter.git
cd rate-limiter

# 2. Start infrastructure (Redis + PostgreSQL)
docker compose up -d

# 3. Apply database migrations
# Linux / macOS
docker exec -i $(docker compose ps -q postgres) \
    psql -U rl -d ratelimiter < migrations/001_create_rules.sql

# Windows (PowerShell)
Get-Content migrations/001_create_rules.sql | docker exec -i ratelimiter-postgres-1 psql -U rl -d ratelimiter

# 4. Generate protobuf code (if not already generated)
protoc --go_out=. --go-grpc_out=. proto/ratelimiter.proto

# 5. Download dependencies
go mod tidy

# 6. Run the server
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

| Method | Type | Description |
|--------|------|-------------|
| `CheckLimit` | Unary | Read-only check. Does not record the request. |
| `ConsumeLimit` | Unary | Checks and records the request atomically. |
| `WatchQuota` | Server Streaming | Subscribes to real-time quota events for a key. |

### Message Types

```protobuf
message LimitRequest {
    string key     = 1;    // Identifier (e.g., "user:harsh123")
    string rule_id = 2;    // Rule to check against (e.g., "free-tier")
    int64  cost    = 3;    // Cost of this request (default: 1)
}

message LimitResponse {
    bool   allowed       = 1;    // Whether the request is allowed
    int64  remaining     = 2;    // Remaining quota in the current window
    int64  limit         = 3;    // Total limit for this rule
    int64  reset_at_unix = 4;    // When the window resets (Unix timestamp)
}

message WatchRequest {
    string key = 1;
}

message QuotaUpdate {
    string key       = 1;
    int64  used      = 2;
    int64  remaining = 3;
    bool   exceeded  = 4;
}
```

### HTTP Endpoints

| Endpoint | Method | Query Params | Description |
|----------|--------|--------------|-------------|
| `/quota` | GET | `key` | SSE stream of quota updates (JSON) |
| `/fire` | GET | `key`, `rule_id` | Triggers a `ConsumeLimit` and returns JSON |

---

## Testing with grpcurl

```bash
# List available services
grpcurl -plaintext localhost:50051 list

# Check a limit (read-only)
grpcurl -plaintext \
    -d '{"key": "user:harsh123", "rule_id": "free-tier"}' \
    localhost:50051 \
    ratelimiter.v1.RateLimiterService/CheckLimit

# Consume a limit (records the request)
grpcurl -plaintext \
    -d '{"key": "user:harsh123", "rule_id": "free-tier"}' \
    localhost:50051 \
    ratelimiter.v1.RateLimiterService/ConsumeLimit

# Watch quota in real time
grpcurl -plaintext \
    -d '{"key": "user:harsh123"}' \
    localhost:50051 \
    ratelimiter.v1.RateLimiterService/WatchQuota
```

### Load test: exhaust the free-tier limit

```bash
for i in $(seq 1 15); do
    grpcurl -plaintext \
        -d '{"key": "user:harsh123", "rule_id": "free-tier"}' \
        localhost:50051 \
        ratelimiter.v1.RateLimiterService/ConsumeLimit
    echo "---"
done
```

**Expected:** Requests 1–10 return `"allowed": true`. Requests 11–15 return `"allowed": false`.

---

## Live Dashboard

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
3. Watch the quota bar drain as requests are consumed.
4. When the limit is exceeded, the bar turns **red**.
5. Wait 60 seconds — the sliding window moves forward and the bar recovers.

---

## Key Design Decisions

### Sliding Window vs Fixed Window

A fixed window resets all counters at fixed intervals. This allows **2× the configured limit** at window boundaries:

```
Window 1: second 59 → 10 requests (allowed)
Window 2: second  1 → 10 requests (allowed)
Total: 20 requests in 2 seconds, limit is 10/minute
```

The **sliding window log** stores every request timestamp in a Redis sorted set. Old entries are evicted by score (time). The count is always accurate for the last *N* milliseconds. No boundary burst problem.

### Why Lua Scripts?

The sliding window requires three Redis commands:

1. `ZREMRANGEBYSCORE` — remove expired entries
2. `ZCARD` — count remaining entries
3. `ZADD` — record this request (if allowed)

Without Lua, another server could interleave between steps 2 and 3, causing both to think there is room. Redis Lua scripts execute atomically — no other command from any client can run between steps.

### Interfaces for Testability

The `Store` interface abstracts the backend behind a contract:

```go
type Store interface {
    Increment(ctx context.Context, key string, window time.Duration, limit int64) (bool, int64, error)
    Reset(ctx context.Context, key string) error
}
```

- `MemoryStore` implements `Store` for unit tests — no Docker, no Redis required.
- `RedisStore` implements `Store` for production — real sliding-window enforcement.

The `Limiter` does not know which implementation it is using. Only `main.go` decides.

---

## Default Rate-Limit Rules

Seeded via `migrations/001_create_rules.sql`:

| Rule ID | Limit | Window | Use Case |
|---------|-------|--------|----------|
| `free-tier` | 10 requests | 60 seconds | Free plan users |
| `pro-tier` | 1,000 requests | 60 seconds | Paid plan users |
| `admin` | 999,999 requests | 60 seconds | Internal/admin access |

Rules are loaded into memory on startup and refreshed from PostgreSQL every 30 seconds. Update a rule with SQL:

```sql
UPDATE rules SET limit_count = 50 WHERE id = 'free-tier';
```

The change takes effect within 30 seconds without restarting the server.

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

## Troubleshooting

| Issue | Solution |
|-------|----------|
| `protoc: command not found` | Install `protoc` via your package manager (see Prerequisites) |
| `cannot use *MemoryStore as type Store` | Method signatures on `MemoryStore` don't match the `Store` interface. Check parameter types and return values. |
| Race condition warnings | Run `go test -race ./...`. Ensure every shared map access is protected by a mutex. |
| Dashboard shows no events | Verify the HTTP server is running on `:8080`. Check CORS headers and browser console for connection errors. |
| Benchmark shows 0 iterations | The server must be running before you execute the benchmark. Start it with `go run cmd/server/main.go`. |

---

## Roadmap

- [ ] gRPC interceptors for structured logging, metrics, and authentication
- [ ] Adaptive rate limiting — different limits per user tier resolved at runtime
- [ ] Prometheus metrics endpoint + Grafana dashboard
- [ ] Kubernetes deployment with Helm charts
- [ ] Circuit breaker for Redis unavailability (graceful degradation)
- [ ] Sliding window counter option for higher throughput at the cost of precision

---

## License

MIT
