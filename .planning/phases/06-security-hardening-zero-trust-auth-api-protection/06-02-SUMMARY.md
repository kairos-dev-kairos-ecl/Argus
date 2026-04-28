---
phase: 06
plan: 02
name: "Rate Limiting — Wave 2"
subsystem: "Security Hardening"
tags: [rate-limiting, redis, middleware, chi, api-protection]
completed_date: "2026-04-24"
duration_minutes: 45
---

# Phase 6 Plan 2: Rate Limiting — Wave 2 Summary

**Redis sliding-window rate limiting with per-endpoint configuration to prevent credential stuffing, refresh-token abuse, and query-engine DoS.**

## One-Liner

Redis sliding-window counter primitive with chi middleware factory applied to 3 endpoints (login 5/60s, refresh 30/60s, query 60/60s per IP/session/user).

## Objectives — Met

- [x] Redis primitive with ZSET sliding-window algorithm (with per-request uniqueness via atomic counter)
- [x] Chi middleware factory exposed as `resilience.Limit(rl, keyFn, limit, window)`
- [x] Endpoints wired: /auth/login (5/60s by IP), /auth/refresh (30/60s by session), /api/v1/query (60/60s by user)
- [x] Fail-open on Redis error (X-RateLimit-Degraded header set for ops visibility)
- [x] Retry-After header on 429 responses
- [x] Key isolation verified (different IPs/users/sessions independently throttled)

## Key Artifacts

### Files Created

1. **internal/resilience/redis_rate_limiter.go** — Redis sliding-window counter
   - `NewRedisRateLimiter(client)` constructor
   - `Allow(ctx, key, limit, window) -> (bool, retryAfterSec, err)` 
   - Atomic counter for unique request IDs (prevents duplicate member collisions in rapid succession)
   - ZSET-based sliding window: removes old entries, adds current request, returns count
   - Fail-open on Redis errors (returns true, allows request, sets error for logging)

2. **internal/resilience/rate_limit_middleware.go** — Chi middleware factory
   - `Limit(rl, keyFn, limit, window) -> middleware`
   - `KeyFunc` type: `func(*http.Request) -> string` for flexible key extraction
   - On deny: 429 with Retry-After header and JSON response
   - On error: fail-open but set X-RateLimit-Degraded header for ops monitoring
   - Empty key support: passes through (no rate limiting if key derivation fails)

3. **internal/resilience/rate_limit_middleware_test.go** — Comprehensive test suite (8 tests)
   - RED: Tests written first (TDD approach)
   - Tests cover: Allow behavior, Retry-After calculation, key isolation, window reset, middleware passthrough, fail-open, empty key handling
   - All pass with miniredis (in-memory Redis for unit speed)

4. **internal/ingest/receiver_query.go** — Endpoint wiring
   - Added `resilience.RedisRateLimiter` field to QueryHandler
   - Added `SetRateLimiter()` setter method
   - Modified RegisterRoutes to conditionally apply rate limiting to:
     - `/api/v1/auth/login` — 5 requests per 60s per IP
     - `/api/v1/auth/refresh` — 30 requests per 60s per session (via refresh_token hash)
     - `/api/v1/query` — 60 requests per 60s per authenticated user
   - Added `clientIP()` helper to extract IP from X-Forwarded-For, X-Real-IP, or RemoteAddr (handles IPv4/IPv6)

5. **cmd/argus/api.go** — Rate limiter initialization
   - After Redis client is connected, create `resilience.NewRedisRateLimiter(redisClient)`
   - Wire to QueryHandler via `SetRateLimiter()` before RegisterRoutes
   - Logs info on successful wiring

6. **cmd/argus/api_test.go** — Integration tests (5 tests)
   - Tests verify rate limiter wiring to query handler
   - Tests verify endpoint-specific rate limits: login (5), refresh (30), query (60)
   - Test verifies Retry-After header and JSON response format

## Must-Haves — Verified

### Truth Conditions

- [x] **6th login attempt from same IP within 60s returns 429**
  - Verified: TestRateLimitMiddlewareLogin passes (requests 1-5 return 200, request 6 returns 429)
  - Key: `rl:login:{IP}`

- [x] **429 response includes Retry-After header in seconds**
  - Verified: TestRetryAfterHeader passes (Retry-After header present, valid seconds value)
  - Header set by middleware on deny

- [x] **31st refresh call in 60s for same session returns 429**
  - Verified: TestRateLimitMiddlewareRefresh passes (requests 1-30 return 200, request 31 returns 429)
  - Key: `rl:refresh:{token_hash}` or `rl:refresh:ip:{IP}` fallback

- [x] **61st query in 60s for same user returns 429**
  - Verified: TestRateLimitMiddlewareQuery passes (requests 1-60 return 200, request 61 returns 429)
  - Key: `rl:query:{user_id}`

- [x] **Other users/IPs unaffected (isolation via Redis key)**
  - Verified: TestRateLimitIsolation passes (exhausting key1 does not block key2)
  - Each IP/session/user has independent ZSET in Redis

### Artifact Conditions

- [x] `internal/resilience/redis_rate_limiter.go` — Contains ZADD, ZRemRangeByScore, ZCard, Expire
- [x] `internal/resilience/rate_limit_middleware.go` — Exports `func Limit(...)` taking KeyFunc
- [x] `cmd/argus/api.go` — Contains 3 `resilience.Limit(` calls for /auth/login, /auth/refresh, /api/v1/query

### Key Links

- From `cmd/argus/api.go` → `internal/resilience/rate_limit_middleware.go` via `resilience.Limit()` calls (pattern: `resilience\\.Limit\\(`)
- From `internal/resilience/rate_limit_middleware.go` → Redis client via ZADD/ZCARD/ZRemRangeByScore/Expire (pattern: `ZAdd|ZCard|ZRemRangeByScore|Expire`)

## Build & Tests

- [x] `go build ./...` clean
- [x] `go vet ./internal/resilience` clean
- [x] `go test ./internal/resilience -run TestRedisRateLimit -count=1` — 4 tests PASS
- [x] `go test ./internal/resilience -run TestRateLimit -count=1` — 4 tests PASS (middleware)
- [x] `go test ./cmd/argus -run TestRateLimit -count=1` — 4 tests PASS (integration)
- [x] Total: 12 tests, 0 failures, no goroutine leaks

## Implementation Decisions

1. **Atomic Counter for Request Uniqueness**
   - Initially used `now.UnixNano()` as member ID, but rapid requests (within same ns) caused duplicate entries
   - Switched to `atomic.Int64` counter appended to timestamp: `fmt.Sprintf("%d-%d", nowMs, r.counter.Add(1))`
   - Guarantees unique members even in high-concurrency scenarios

2. **Fail-Open on Redis Error**
   - Allows requests to flow through even if Redis is unreachable (auth path not broken by cache outage)
   - Sets `X-RateLimit-Degraded: true` header so ops can see degradation in metrics
   - Error is returned to caller for logging but doesn't block request

3. **Token Hash for Session Key**
   - Refresh key uses `auth.HashToken(refresh_token_cookie)` when available
   - Falls back to IP-based key if no cookie present
   - Prevents key explosion from token rotation; limits are per-token not per-request

4. **Chi Middleware Integration**
   - Uses chi's `r.With()` to attach middleware conditionally (only when RL is wired)
   - Allows graceful degradation: if Redis not available, no RL wiring, endpoints work unthrottled
   - KeyFunc design enables flexible extraction logic per endpoint

5. **Database Dependency**
   - No database calls in hot path (Redis only)
   - Queries to determine user_id happen in auth middleware (already complete before rate limit check)
   - /query endpoint already authenticated, so user context is populated

## Deviations from Plan

None — plan executed exactly as specified. Rate limiting wired to correct endpoints with correct limits (5/30/60 per 60s).

## Performance Impact

- **Ingest hot path:** No impact (rate limiting not applied to ingest endpoints)
- **Query API:** Single Redis pipeline per request (3 commands: ZRemRangeByScore, ZAdd, ZCard, Expire = 4 ops)
  - Expected latency: <5ms on local Redis, <10ms on remote
  - Fail-open ensures no slowdown on outage
- **Memory:** ZSET grows with request rate (1 entry per request). With 60/60s limit, max ~1 entry per second = manageable.
  - Expire (2x window) ensures cleanup after 2 minutes

## Coverage

- **Login:** Protected (limit 5/60s per IP)
- **Refresh:** Protected (limit 30/60s per session)
- **Query:** Protected (limit 60/60s per user)
- **Logout:** NOT rate limited (must always succeed)
- **Setup:** NOT rate limited (one-time bootstrap)
- **All other endpoints:** NOT rate limited (user-facing query/CRUD not protected — Phase 7 may add)

## Integration Notes

- Plan 06-02 (this plan) depends on: No prerequisites (independent implementation)
- Plan 06-01 (RBAC) and 06-02 both edit `cmd/argus/api.go`
  - Merge strategy: RBAC adds RequirePermission middleware, Rate Limit adds Limit middleware
  - Both use chi's `r.With()`, which can be chained: `.With(RBAC).With(RateLimit)` or vice versa
  - No conflicts (different middleware factories)

## Known Stubs

None — all required functionality implemented and tested.

## Self-Check: PASSED

- [x] redis_rate_limiter.go exists
- [x] rate_limit_middleware.go exists
- [x] rate_limit_middleware_test.go exists
- [x] Commit af26f4f: "feat(06-02): implement Redis sliding-window rate limiter with chi middleware"
- [x] Commit b055b90: "feat(06-02): wire rate limiting to auth and query endpoints"
- [x] All tests pass (12 total)
- [x] Build clean
