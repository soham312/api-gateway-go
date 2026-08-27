# Handoff: API Gateway Go

Status snapshot as of 2026-08-27. Written for whoever (including future-you)
picks this repo back up. Nothing described here has been committed — every
change below is sitting uncommitted in the working tree (`git status` /
`git diff` shows all of it) so it can be reviewed before committing.

## What this project is

A Layer 7 API gateway in Go (`github.com/soham312/api-gateway-go`):
reverse proxy with longest-prefix routing, weighted round-robin and
least-connections load balancing, an active health checker with a 3-state
circuit breaker (closed/open/half-open), retry-with-exponential-backoff,
JWT auth, CORS, per-IP rate limiting, and zero-downtime config hot-reload
via fsnotify.

Layout:
- `cmd/gateway/main.go` — wiring/entrypoint
- `internal/router` — longest-prefix route matching
- `internal/balancer` — round-robin (weighted) and least-connections
- `internal/health` — `Backend` (circuit breaker state) + `Poller` (active health checks)
- `internal/proxy` — reverse proxy director + retry/backoff transport
- `internal/middleware` — JWT auth, CORS, rate limiting, request logging
- `internal/config` — JSON config load + fsnotify watch
- `test.js` — k6 load test script (never run against this repo; k6 not installed here)

## Current verified state

```
gofmt -l .        -> clean
go build ./...    -> OK
go vet ./...      -> OK
go test ./... -race -cover -> all packages pass, 47 tests, 0 races
```

Coverage by package: `balancer` 100%, `router` 100%, `middleware` 85.4%,
`proxy` 85.5%, `health` 61.2%, `config` 31% (pre-existing test, untouched),
`cmd/gateway` 0% (just `main()` wiring — not unit-testable as written).

Smoke-tested the actual binary (not just `go test`) against a local mock
backend: routing, exact burst-based rate limiting (5 allowed then 429),
and `SIGTERM` → graceful drain → clean exit all confirmed working live.

## ⚠️ Incident: a `git reset` wiped this work once already

Partway through committing this work in small pieces, a `git reset`
(moving `HEAD` back to `origin/main`) discarded two already-made commits
and, combined with untracked files being removed, wiped every other
uncommitted change on disk (all fixes, all new test files, this doc). It
was rebuilt from conversation history and reapplied. **Lesson for next
time: don't run `git reset --hard` / `git clean` while there is
uncommitted work you still want — check `git status` and stash first, or
just commit everything as it's finished before doing any history surgery.**
If you're reading this and the fixes below aren't actually present in the
files, that's a sign it happened again — check `git reflog` before
assuming the work never existed.

## What was done this session, and why

Before this session: only `internal/config` had tests; every other package
was at 0% coverage. Everything below was found by writing real tests
against the existing code, not by inspection alone.

### Tests added (new files, all passing under `-race`)
- `internal/balancer/balancer_test.go` — weighted distribution, empty list, unhealthy-skip for both balancers
- `internal/router/router_test.go` — longest-prefix match, strip-prefix (incl. exact-match edge case), no-match, live route update
- `internal/health/health_test.go` — circuit breaker transitions (closed→open, half-open→open, half-open→closed, failure-count reset)
- `internal/middleware/auth_test.go` — missing/malformed/invalid/expired/wrong-secret tokens, **plus a forged `alg:none` token** (see security fix below)
- `internal/middleware/cors_test.go` — wildcard origin, disallowed origin, OPTIONS short-circuit
- `internal/middleware/ratelimit_test.go` — burst/block behavior, per-IP isolation, **plus spoofed-header bypass attempt** (see security fix below)
- `internal/proxy/proxy_test.go` — forwarding, retry-then-succeed, **plus fail-fast timing assertions** (see bug fix below)

### Bugs / security issues found and fixed

1. **JWT algorithm-confusion vulnerability** — `internal/middleware/auth.go`.
   The original `jwt.Parse` keyfunc returned the secret unconditionally,
   never checking `token.Method`. A forged token with `alg: none` (or any
   non-HMAC algorithm) would pass verification. Fixed by checking
   `token.Method.(*jwt.SigningMethodHMAC)` inside the keyfunc and adding
   `jwt.WithValidMethods([]string{"HS256","HS384","HS512"})`.
   `TestJWTAuth_RejectsNoneAlgorithm` forges exactly this attack and
   proves it's now rejected.

2. **Rate limiter trusted spoofable headers** — `internal/middleware/ratelimit.go`,
   `internal/config/config.go`. `getIP` read `X-Forwarded-For`/`X-Real-IP`
   unconditionally, so any client could set a different header value per
   request and get a fresh rate-limit bucket every time — the limiter was
   trivially bypassable unless the gateway sat behind a proxy that
   overwrote those headers. Fixed by adding a `trust_proxy_headers` config
   flag (`middleware.rate_limit.trust_proxy_headers` in config.json,
   defaults to `false`) that gates whether those headers are honored at
   all. `getIP` and `NewRateLimiter` both took a new `trustProxy bool`
   parameter — **this is a breaking signature change**, already updated
   at the one call site in `main.go`. `TestRateLimiter_UntrustedProxyCannotBypassViaSpoofedHeader`
   proves the bypass no longer works with the default (`false`) setting.

3. **Retry-storm on requests that never reached a backend** —
   `internal/proxy/proxy.go`. When no route matched, or a route matched
   but every backend was circuit-broken, `GatewayTransport.RoundTrip`
   still ran through the full `maxRetries` + exponential-backoff loop
   before giving up — turning what should be an instant 502 into a
   multi-second one. Fixed by tracking whether a backend was actually
   selected (`routed := ok && backend != nil`) and forcing `maxRetries = 0`
   when it wasn't, applied *after* the config overrides so config can't
   silently re-enable retries here. Confirmed via timing assertions in
   `TestProxy_NoRouteMatch_FailsFast` / `TestProxy_AllBackendsDown_FailsFast`
   (was >1000ms, now <1ms).

4. **Silent auth-disable footgun** — `cmd/gateway/main.go`. An empty
   `middleware.jwt.secret` in config disabled JWT auth on every route with
   zero signal to whoever deployed it. Now logs a loud warning at startup:
   `⚠️  JWT authentication is DISABLED: no middleware.jwt.secret configured.`

5. **No graceful shutdown** — `cmd/gateway/main.go`. `ListenAndServe`/`ListenAndServeTLS`
   ran directly with no signal handling, so a deploy or restart would drop
   in-flight connections. Added `SIGINT`/`SIGTERM` handling via
   `signal.Notify`, with `srv.Shutdown(ctx)` on a 15s drain timeout.
   Verified live (see smoke test above).

6. **Dead code removed** — `internal/proxy/backend.go` defined a `Backend`
   struct (`SetStatus`/`IsAlive`) that nothing in the codebase referenced;
   `internal/health.Backend` is what's actually used everywhere. Deleted.

7. **Formatting** — `gofmt -w .` applied across the repo (import ordering,
   trailing whitespace). Pure formatting, no logic changes.

### Config changes

New field, off by default, backward compatible:
```json
"rate_limit": {
  ...,
  "trust_proxy_headers": false
}
```
Added to both `config.json` and `testdata/config.json` for documentation
visibility (JSON zero-value would default to `false` even without it).

## Known gaps / things NOT done

- **`cmd/gateway/main.go` has 0% test coverage.** It's wiring/composition,
  not logic — reasonable to leave untested, but worth knowing if someone
  asks "what's untested and why."
- **No containerized multi-service demo.** A `Dockerfile` exists (per git
  log) but there's no `docker-compose.yml` wiring the gateway to real
  backend containers for a one-command demo.

Resolved in a follow-up session (2026-08-28): README's fabricated
benchmark table was removed; a minimal `/metrics` endpoint
(`internal/metrics`, Prometheus text format — total requests, requests
by status code, per-backend circuit state) was added and wired into
`main.go` via a small `http.ServeMux` so it bypasses the auth/rate-limit
chain; a GitHub Actions CI workflow (`.github/workflows/ci.yml`) now
runs gofmt/vet/build/test on every push and PR to `main`.

### k6 benchmark: actually run this time

`k6` was installed via `go install go.k6.io/k6@latest` (no Homebrew in
this environment) and `test.js` was actually run. Two things fell out of
that:

1. **`test.js` had a real bug**: it hardcoded `http://host.docker.internal:8080`,
   which only resolves when k6 runs inside Docker — but there's no
   `docker-compose.yml` in this repo, so anyone following the README's
   own `k6 run test.js` instruction would get a DNS failure. Fixed to
   default to `http://localhost:8080`, overridable via `-e BASE_URL=...`.
2. **First real run (against `python3 -m http.server` as the backend)
   showed a 13% failure rate.** Root-caused to two compounding factors:
   the Python dev server's own concurrency limits under 100 VUs, *and* a
   genuine gateway bug — `internal/proxy.GatewayTransport` was calling
   `http.DefaultTransport.RoundTrip`, whose default `MaxIdleConnsPerHost`
   is 2. That starves a reverse proxy fanning many concurrent client
   requests into a few backend hosts, forcing a fresh TCP handshake per
   request under load. Fixed by giving `GatewayTransport` its own
   `http.Transport` (`MaxIdleConnsPerHost: 100`, `MaxIdleConns: 200`,
   `IdleConnTimeout: 90s`) in `proxy.New()`.
   Re-testing against the *same* Python backend after the fix only
   dropped the failure rate to 9% — confirming the Python dev server
   itself was still the dominant bottleneck, not (only) the gateway.
   Swapped in a two-line Go stub backend (plain `net/http`, immediate
   `200 OK`) instead, and got a clean **341,925 requests, 0 failures,
   ~34,185 req/s, 2.88ms avg / 6.72ms p95 latency** (Apple M3, macOS
   26.0.1, go1.27.0, 100 VUs, 10s). That real, reproducible number is now
   in the README's Load Testing section, along with the methodology and
   a note about the Python-backend detour so nobody mistakes the earlier
   13%/9% numbers for a gateway defect if they go looking.

None of the benchmark scaffolding (stub backend, temp configs) is part
of the repo — it lived in `/tmp/gw_bench` for this session only and was
torn down after. The only repo changes from this: `test.js`'s `BASE_URL`
fix and the `internal/proxy/proxy.go` transport-pooling fix (with
existing tests still passing at 86.1% coverage for that package, up from
85.5% — no new test was written specifically for the pooling change
itself; it's exercised indirectly by the existing proxy tests).

Remaining open items: Docker Compose demo, a security case-study write-up
of the JWT bug. Still an open decision, not a to-do list to execute
unprompted.

## How to verify this state yourself

```bash
gofmt -l .                      # expect no output
go build ./...                  # expect success
go vet ./...                    # expect success
go test ./... -race -cover -v   # expect all PASS, 0 races
```

Manual smoke test (routing + rate limit + graceful shutdown), using a
throwaway local backend instead of the internet-dependent routes in the
committed `config.json`:
```bash
go build -o /tmp/gateway ./cmd/gateway
python3 -m http.server 9001 --directory /tmp &   # dummy backend
# write a temp config.json pointing a route at http://127.0.0.1:9001,
# cd into that temp dir, run /tmp/gateway from there, curl it, then
# `kill -TERM <pid>` and confirm the "shut down cleanly" log line.
```

## Committing this work

Before running anything destructive (`git reset`, `git clean`, `git
checkout --`), re-read the incident note above. Recommended path: commit
everything in small logical pieces (gofmt, then each fix + its tests
together, then the doc), verifying `go build && go vet && go test
./... -race` still passes after each commit, and don't run any reset/clean
commands until it's all safely committed.
