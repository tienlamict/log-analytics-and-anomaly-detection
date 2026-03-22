# Phase 4: REST API - Research

**Researched:** 2026-03-22
**Domain:** Go HTTP API (chi v5, go-elasticsearch v9 TypedClient search, httptest)
**Confidence:** HIGH

---

## Summary

Phase 4 adds a chi-based HTTP server in `internal/api` that exposes four data-query endpoints (logs list, log by ID, anomalies list, anomaly by ID), plus `/health`, `/ready`, and `/metrics` endpoints. The project already has all domain types and store interfaces defined (`LogStore`, `AnomalyStore`, `LogQuery`, `AnomalyQuery`) and stub implementations in `internal/elasticsearch` — this phase fills in `SearchLogs`, `GetLog`, `SearchAnomalies`, and `GetAnomaly`. The chi router, its middleware, and `errgroup` for lifecycle management are the only new dependencies.

The main architectural split is clean: `internal/api` handles HTTP routing, input parsing, and response marshaling; `internal/elasticsearch` implements the store interfaces against the TypedClient. The API server runs in its own goroutine under the existing `errgroup` context from `cmd/server/main.go`. Health-check logic wires directly to the ES client's Ping call and a readiness flag set once the pipeline is up.

`golang.org/x/sync` (v0.17.0) is already a transitive dependency and provides `errgroup.WithContext`. Only `github.com/go-chi/chi/v5` needs to be added to `go.mod`.

**Primary recommendation:** Use `client.Search().Index("logs-*").Request(req).Do(ctx)` with a `*search.Request` struct containing a `types.BoolQuery` for all filter logic; unmarshal each `hit.Source_` via `json.Unmarshal`. For single-document lookup, use `client.Get("index", id).Do(ctx)` and check `response.Found` — the client returns `Found: false` (not an error) for missing documents since v8.8.2.

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| API-01 | `GET /api/v1/logs` — query by service, level, time range with pagination | ES TypedClient Search on `logs-*`; bool/term/range query pattern; from/size pagination |
| API-02 | `GET /api/v1/logs/{id}` — single log entry by ID | ES TypedClient Get; `response.Found` for 404 |
| API-03 | `GET /api/v1/anomalies` — query by type, service, severity, time range with pagination | Same query pattern on `anomalies` index |
| API-04 | `GET /api/v1/anomalies/{id}` — single anomaly by ID | Same Get pattern |
| API-05 | `GET /health` and `GET /ready` health endpoints | chi route; ES Ping for health; atomic bool for readiness |
| API-06 | `GET /metrics` Prometheus metrics endpoint | `promhttp.Handler()` already in main; relocate/share under chi router |
</phase_requirements>

---

## Standard Stack

### Core — New Dependencies

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/go-chi/chi/v5` | v5.2.5 | HTTP router | Project decision (STATE.md); 100% net/http compatible, no framework lock-in |

### Core — Already in go.mod

| Library | Version | Purpose | Status |
|---------|---------|---------|--------|
| `github.com/elastic/go-elasticsearch/v9` | v9.3.1 | ES TypedClient search/get | Already present (indirect) |
| `github.com/prometheus/client_golang` | v1.23.2 | `promhttp.Handler()` | Already present |
| `go.uber.org/zap` | v1.27.0 | Structured logging | Already present |
| `golang.org/x/sync` | v0.17.0 | `errgroup.WithContext` | Already transitive dep |

### Supporting — Already in go.mod

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/stretchr/testify` | v1.11.1 | Test assertions | All handler unit tests |
| `go.uber.org/goleak` | v1.3.0 | Goroutine leak detection | `TestMain` wrapper on handler tests |

**Installation (new dependency only):**
```bash
go get github.com/go-chi/chi/v5@v5.2.5
```

**Version verification (confirmed 2026-03-22):**
- chi v5.2.5 — published 2026-02-05 (Go module proxy confirmed)
- go-elasticsearch v9.3.1 — already pinned in go.mod
- golang.org/x/sync v0.17.0 — already transitive, exposes errgroup

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| chi v5 | stdlib `http.ServeMux` (Go 1.22) | ServeMux now supports method+path patterns; chi adds sub-routing, middleware composition, URL params — chi is locked in by STATE.md |
| errgroup | manual goroutine + WaitGroup | errgroup propagates first error and cancels context automatically; strictly better for multi-server lifecycle |

---

## Architecture Patterns

### Recommended Project Structure

```
internal/
├── api/
│   ├── server.go          # Server struct, NewServer, chi router init, middleware, ListenAndServe
│   ├── health.go          # /health, /ready handlers; readiness flag (atomic.Bool)
│   ├── logs.go            # GET /api/v1/logs, GET /api/v1/logs/{id}
│   ├── anomalies.go       # GET /api/v1/anomalies, GET /api/v1/anomalies/{id}
│   └── middleware.go      # Custom Zap request-logging middleware (chi-compatible)
├── elasticsearch/
│   ├── log_indexer.go     # (existing) — add SearchLogs + GetLog implementations
│   └── anomaly_indexer.go # (existing) — add SearchAnomalies + GetAnomaly implementations
cmd/
└── server/
    └── main.go            # Wire Server into errgroup alongside existing pipeline
```

### Pattern 1: Chi Router Initialisation with Middleware

**What:** Construct a chi.Router with ordered middleware, mount routes, wrap in net/http.Server.
**When to use:** Always — this is the single entry point for the API server.

```go
// Source: pkg.go.dev/github.com/go-chi/chi/v5 (verified 2026-03-22)
import (
    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "time"
)

r := chi.NewRouter()
r.Use(middleware.Recoverer)
r.Use(middleware.RequestID)
r.Use(middleware.Timeout(30 * time.Second))
r.Use(zapRequestLogger(logger))          // custom middleware — see middleware.go
r.Get("/health", s.handleHealth)
r.Get("/ready", s.handleReady)
r.Get("/metrics", promhttp.Handler().ServeHTTP)
r.Route("/api/v1", func(r chi.Router) {
    r.Get("/logs", s.handleListLogs)
    r.Get("/logs/{id}", s.handleGetLog)
    r.Get("/anomalies", s.handleListAnomalies)
    r.Get("/anomalies/{id}", s.handleGetAnomaly)
})
```

**Middleware order matters:** `Recoverer` first so panics are caught before RequestID and Timeout run; Logger (custom Zap) placed after Recoverer to log recovered-panic responses correctly. The built-in `middleware.Logger` writes to stdout; the custom Zap middleware is preferred for consistency with the rest of the project.

### Pattern 2: Custom Zap Request-Logging Middleware

**What:** Wrap `http.Handler` to log method, path, status, duration using the project's `*zap.Logger`.
**When to use:** Replace `middleware.Logger` (which writes to os.Stdout) with this for structured JSON logs.

```go
// Pattern — no external library needed; uses chi's middleware.WrapResponseWriter
func zapRequestLogger(log *zap.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
            start := time.Now()
            defer func() {
                log.Info("http request",
                    zap.String("method", r.Method),
                    zap.String("path", r.URL.Path),
                    zap.Int("status", ww.Status()),
                    zap.Duration("duration", time.Since(start)),
                    zap.String("request_id", middleware.GetReqID(r.Context())),
                )
            }()
            next.ServeHTTP(ww, r)
        })
    }
}
```

### Pattern 3: URL Parameter Extraction

**What:** Extract `{id}` from path using `chi.URLParam`.
**When to use:** All `/{id}` routes (logs/{id}, anomalies/{id}).

```go
// Source: pkg.go.dev/github.com/go-chi/chi/v5 (verified 2026-03-22)
func (s *Server) handleGetLog(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    // id is empty string if not found (safe; never panics)
}
```

### Pattern 4: ES TypedClient Search with Bool/Term/Range

**What:** Build a `*search.Request` with `types.BoolQuery` filter clauses for keyword terms and time ranges.
**When to use:** `SearchLogs` and `SearchAnomalies` implementations in `internal/elasticsearch`.

```go
// Source: elastic.co/search-labs/blog/perform-text-queries (verified 2026-03-22)
//         pkg.go.dev/github.com/elastic/go-elasticsearch/v9/typedapi/core/search
import (
    "github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
    "github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

filters := []types.Query{}
if q.Service != "" {
    filters = append(filters, types.Query{
        Term: map[string]types.TermQuery{"service": {Value: q.Service}},
    })
}
if q.Level != "" {
    filters = append(filters, types.Query{
        Term: map[string]types.TermQuery{"level": {Value: q.Level}},
    })
}
if !q.From.IsZero() || !q.To.IsZero() {
    rq := types.RangeQuery{}
    // types.DateRangeQuery embeds in RangeQuery via interface; use types.DateRangeQuery
    drq := types.DateRangeQuery{}
    if !q.From.IsZero() { v := q.From.Format(time.RFC3339); drq.Gte = &v }
    if !q.To.IsZero()   { v := q.To.Format(time.RFC3339);   drq.Lte = &v }
    _ = rq // use DateRangeQuery directly
    filters = append(filters, types.Query{
        Range: map[string]types.RangeQuery{"@timestamp": drq},
    })
}

from := (q.Page - 1) * q.Size
size := q.Size

req := &search.Request{
    Query: &types.Query{Bool: &types.BoolQuery{Filter: filters}},
    From:  &from,
    Size:  &size,
}
res, err := client.Search().Index("logs-*").Request(req).Do(ctx)
if err != nil { return nil, 0, fmt.Errorf("es search: %w", err) }

total := res.Hits.Total.Value
var entries []domain.LogEntry
for _, hit := range res.Hits.Hits {
    var entry domain.LogEntry
    if err := json.Unmarshal(hit.Source_, &entry); err != nil {
        logger.Warn("unmarshal log hit", zap.Error(err), zap.String("id", hit.Id_))
        continue
    }
    entry.ID = hit.Id_
    entries = append(entries, entry)
}
return entries, total, nil
```

**Note on RangeQuery types:** The ES TypedClient v9 uses `types.RangeQuery` as an interface type; the concrete implementation for date fields is `types.DateRangeQuery`. The map key must match the ES field name — use `@timestamp` for logs (matching the index template in `setup.go`) and `detected_at` for anomalies.

### Pattern 5: ES TypedClient Get by ID

**What:** Retrieve a single document; check `response.Found` for 404.
**When to use:** `GetLog` and `GetAnomaly` implementations.

```go
// Source: github.com/elastic/go-elasticsearch/issues/678 (fixed v8.8.2+, v9 inherits fix)
//         pkg.go.dev/github.com/elastic/go-elasticsearch/v9/typedapi/core/get
res, err := client.Get("logs-*", id).Do(ctx)
// Note: wildcard index not supported by Get — must use the concrete index name or _all
// For logs, use "_all" or query by doc ID across indices using search with _id term query
if err != nil { return domain.LogEntry{}, fmt.Errorf("es get: %w", err) }
if !res.Found {
    return domain.LogEntry{}, domain.ErrNotFound  // sentinel for 404
}
var entry domain.LogEntry
if err := json.Unmarshal(res.Source_, &entry); err != nil {
    return domain.LogEntry{}, fmt.Errorf("unmarshal: %w", err)
}
entry.ID = res.Id_
return entry, nil
```

**Critical caveat:** The ES `Get` API requires a concrete index name — it does NOT support wildcard patterns like `logs-*`. For `GetLog`, query using `Search().Index("logs-*")` with a `types.TermQuery` on the `_id` meta-field, or use `"_all"` as the index. The safest approach for `GetLog` is a search with `IDs` query type:

```go
req := &search.Request{
    Query: &types.Query{Ids: &types.IdsQuery{Values: []string{id}}},
}
res, err := client.Search().Index("logs-*").Request(req).Do(ctx)
```

### Pattern 6: API Server Lifecycle under errgroup

**What:** Run the HTTP server goroutine under the shared `errgroup` from main.
**When to use:** `cmd/server/main.go` wiring.

```go
// Source: pkg.go.dev/golang.org/x/sync/errgroup (verified — already transitive dep v0.17.0)
eg, ctx := errgroup.WithContext(rootCtx)

eg.Go(func() error {
    if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        return fmt.Errorf("api server: %w", err)
    }
    return nil
})

eg.Go(func() error {
    <-ctx.Done()
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    return apiServer.Shutdown(shutdownCtx)
})

if err := eg.Wait(); err != nil {
    logger.Error("server exited with error", zap.Error(err))
}
```

### Pattern 7: Pagination Envelope Response

**What:** JSON envelope `{"data": [...], "total": N, "page": P, "size": S}`.
**When to use:** All four list endpoints.

```go
type PagedResponse struct {
    Data  any   `json:"data"`
    Total int64 `json:"total"`
    Page  int   `json:"page"`
    Size  int   `json:"size"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(v)
}
```

### Pattern 8: Query Parameter Parsing with Validation

**What:** Parse `?page=`, `?size=`, `?from=`, `?to=` from URL query; apply defaults and caps.
**When to use:** `handleListLogs` and `handleListAnomalies`.

```go
func parseIntParam(r *http.Request, name string, def, min, max int) (int, error) {
    raw := r.URL.Query().Get(name)
    if raw == "" { return def, nil }
    v, err := strconv.Atoi(raw)
    if err != nil { return 0, fmt.Errorf("%s: not an integer", name) }
    if v < min { v = min }
    if v > max { v = max }
    return v, nil
}

func parseTimeParam(r *http.Request, name string) (time.Time, error) {
    raw := r.URL.Query().Get(name)
    if raw == "" { return time.Time{}, nil }
    t, err := time.Parse(time.RFC3339, raw)
    if err != nil { return time.Time{}, fmt.Errorf("%s: must be RFC 3339", name) }
    return t, nil
}
```

### Pattern 9: Health and Readiness Handlers

**What:** `/health` pings ES; `/ready` checks an `atomic.Bool` set after pipeline init.
**When to use:** Plan 04-01.

```go
// readinessFlag set to true in main.go after pipeline goroutines start
var ready atomic.Bool

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
    defer cancel()
    _, err := s.esClient.Ping().Do(ctx)
    if err != nil {
        http.Error(w, `{"status":"unhealthy"}`, http.StatusServiceUnavailable)
        return
    }
    writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
    if !s.ready.Load() {
        http.Error(w, `{"status":"not ready"}`, http.StatusServiceUnavailable)
        return
    }
    writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
```

**Note:** `client.Ping()` on `elasticsearch.TypedClient` returns `(*PingResponse, error)`. A non-nil error means ES is unreachable or returned an error status. This is consistent with the project's existing ES client usage pattern.

### Pattern 10: Handler Unit Testing with httptest

**What:** Test handlers in isolation using mock store implementations and `httptest.NewRecorder`.
**When to use:** Plan 04-04 — all four query handlers.

```go
// Source: stdlib net/http/httptest (no import needed beyond standard library)
func TestHandleListLogs_ValidQuery(t *testing.T) {
    mockStore := &mockLogStore{
        entries: []domain.LogEntry{{ID: "abc", Service: "api", Level: "error"}},
        total:   1,
    }
    s := NewServer(mockStore, nil, nil, zap.NewNop())
    req := httptest.NewRequest(http.MethodGet, "/api/v1/logs?service=api&level=error", nil)
    rec := httptest.NewRecorder()
    s.router.ServeHTTP(rec, req)
    assert.Equal(t, http.StatusOK, rec.Code)
    // parse and assert response body
}

// Mock interface implementation (in _test file or testhelpers)
type mockLogStore struct {
    entries []domain.LogEntry
    total   int64
    err     error
}

func (m *mockLogStore) IndexLog(_ context.Context, _ domain.LogEntry) error { return m.err }
func (m *mockLogStore) SearchLogs(_ context.Context, _ domain.LogQuery) ([]domain.LogEntry, int64, error) {
    return m.entries, m.total, m.err
}
func (m *mockLogStore) GetLog(_ context.Context, id string) (domain.LogEntry, error) {
    if m.err != nil { return domain.LogEntry{}, m.err }
    for _, e := range m.entries {
        if e.ID == id { return e, nil }
    }
    return domain.LogEntry{}, domain.ErrNotFound
}
```

### Anti-Patterns to Avoid

- **Using chi.URLParam before registering route:** Returns empty string silently. Always define the route pattern with `{id}` before calling `URLParam`.
- **Ignoring Timeout context in handlers:** `middleware.Timeout` cancels `r.Context()` — all ES calls must use `r.Context()` not `context.Background()` to respect the deadline.
- **Writing header after body:** Call `w.WriteHeader(status)` before `json.NewEncoder(w).Encode(v)` — once `w.Write` is called, the header is sent automatically as 200 and `WriteHeader` is a no-op.
- **Panic from `middleware.Recoverer` without recovery response:** Place `Recoverer` FIRST in the Use chain so it wraps all subsequent middleware and handlers.
- **Using `client.Get("logs-*", id)` for log retrieval:** ES Get API does not support wildcard index patterns; use Search with an IDs query instead.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| URL path parameter extraction | Custom string parsing / regex | `chi.URLParam(r, "id")` | chi injects params into request context automatically |
| Panic recovery middleware | Recover() + write 500 | `middleware.Recoverer` | Handles stack trace logging, avoids double-write bugs |
| Request ID generation | uuid.New() in middleware | `middleware.RequestID` | Thread-safe, injects into context, propagates to response header |
| Response writer wrapping to capture status code | Custom `ResponseWriter` wrapper | `middleware.NewWrapResponseWriter` | Already in chi; captures status, bytes written, hijacker support |
| Pagination defaults and clamping | Inline if-else chains | Helper parse functions (see Pattern 8) | Centralized; easy to test; prevents size > 1000 from reaching ES |

**Key insight:** The ES TypedClient's bulk indexer (`esutil`) is for writes; search queries use a completely different API path (`client.Search()`, `client.Get()`). Do not attempt to use `esutil` for reads.

---

## Common Pitfalls

### Pitfall 1: ES Get with Wildcard Index
**What goes wrong:** `client.Get("logs-*", id).Do(ctx)` returns an error (index not found or malformed request) rather than a document.
**Why it happens:** ES `_get` API requires a concrete index name; wildcards are not supported.
**How to avoid:** Use `client.Search().Index("logs-*").Request(&search.Request{Query: &types.Query{Ids: &types.IdsQuery{Values: []string{id}}}}).Do(ctx)` and check `len(hits) == 0` for 404.
**Warning signs:** HTTP 400 or 404 from ES even when the document exists.

### Pitfall 2: Missing `ErrNotFound` Sentinel
**What goes wrong:** `GetLog` / `GetAnomaly` return a generic `errors.New("not found")` — the handler can't distinguish a missing document from an ES transport error, so it returns 500 instead of 404.
**Why it happens:** No sentinel error defined in the domain package.
**How to avoid:** Define `var ErrNotFound = errors.New("not found")` in `internal/domain` (or a dedicated errors file); use `errors.Is(err, domain.ErrNotFound)` in handlers to write 404.
**Warning signs:** Integration tests expecting 404 get 500.

### Pitfall 3: Timeout Context Not Propagated to ES
**What goes wrong:** Handler hangs past `middleware.Timeout` deadline because ES calls use `context.Background()`.
**Why it happens:** Copying the ES call pattern from setup.go (which uses background context) into handler code.
**How to avoid:** Always pass `r.Context()` into every ES call inside an HTTP handler.
**Warning signs:** Load tests show goroutine leaks; requests pile up after slow ES responses.

### Pitfall 4: Double Header Write
**What goes wrong:** First `writeJSON` call with 200, then a deferred error write tries to call `http.Error` — client sees 200 with garbled body.
**Why it happens:** Error check happens after the response is partially written.
**How to avoid:** Check all errors before writing any response. Structure handlers as: parse → query → write result OR write error (never both).
**Warning signs:** JSON decode failures on the client for error cases.

### Pitfall 5: Pagination Off-by-One in ES `from`
**What goes wrong:** Page 1, size 10 should return documents 0-9. If `from = page * size` instead of `(page-1) * size`, page 1 returns documents 10-19.
**Why it happens:** ES `from` is a zero-based offset; page numbers are 1-based from the API.
**How to avoid:** `from = (page - 1) * size`; default page=1 maps to `from=0`.
**Warning signs:** First page of results is empty or offset by one page.

### Pitfall 6: chi.Timeout Middleware vs. Server.ReadTimeout
**What goes wrong:** `http.Server.ReadTimeout` and `middleware.Timeout` both configured to 30s — double-counting; or ReadTimeout fires before chi Timeout cancels context properly.
**Why it happens:** Two independent timeout mechanisms.
**How to avoid:** Set `http.Server` timeouts conservatively (e.g., 60s) and rely on `middleware.Timeout` for per-handler deadlines. Do not set both to the same value.
**Warning signs:** 504 responses with no handler log entries (server-level timeout fired first).

### Pitfall 7: Prometheus `/metrics` Port Conflict
**What goes wrong:** `main.go` already runs a `http.ServeMux` with `/metrics` on port 2112; the new chi API server also mounts `/metrics` — two servers fight for the same port or the operator hits the wrong port.
**Why it happens:** Prometheus metrics endpoint currently lives in a separate server in `main.go` (Phase 1 stub).
**How to avoid:** Consolidate: remove the old `http.Server` from main.go and mount `promhttp.Handler()` on the chi router under the API port. Update `MetricsConfig.Port` to be the API port, or keep two servers on different ports with explicit config. The plan says mount at `GET /metrics` on the chi router — so the old server should be removed.
**Warning signs:** `bind: address already in use` on startup, or metrics at different port than health.

### Pitfall 8: `AnomalyQuery.Type` maps to `rule_id` in ES
**What goes wrong:** `?type=error_rate` filters on a `type` field that doesn't exist in the ES mapping; no results returned.
**Why it happens:** `AnomalyQuery.Type` semantically means "rule type" / `rule_id` in the domain, but the ES mapping uses `rule_id` (see `setup.go` anomalies template).
**How to avoid:** Map `AnomalyQuery.Type` to a term filter on `rule_id` field in ES, not `type`.
**Warning signs:** `?type=error_rate` returns 0 results even with matching anomalies.

---

## Code Examples

### Complete SearchLogs Implementation Sketch

```go
// Source: elastic.co typed API docs + pkg.go.dev verified patterns (2026-03-22)
func (li *LogIndexer) SearchLogs(ctx context.Context, q domain.LogQuery) ([]domain.LogEntry, int64, error) {
    var filters []types.Query
    if q.Service != "" {
        filters = append(filters, types.Query{
            Term: map[string]types.TermQuery{"service": {Value: q.Service}},
        })
    }
    if q.Level != "" {
        filters = append(filters, types.Query{
            Term: map[string]types.TermQuery{"level": {Value: q.Level}},
        })
    }
    if !q.From.IsZero() || !q.To.IsZero() {
        drq := types.DateRangeQuery{}
        if !q.From.IsZero() { v := q.From.UTC().Format(time.RFC3339); drq.Gte = &v }
        if !q.To.IsZero()   { v := q.To.UTC().Format(time.RFC3339);   drq.Lte = &v }
        filters = append(filters, types.Query{
            Range: map[string]types.RangeQuery{"@timestamp": drq},
        })
    }

    page := q.Page
    if page < 1 { page = 1 }
    size := q.Size
    if size < 1 { size = 20 }
    if size > 1000 { size = 1000 }
    from := (page - 1) * size

    boolQ := &types.BoolQuery{}
    if len(filters) > 0 {
        boolQ.Filter = filters
    }

    req := &search.Request{
        Query: &types.Query{Bool: boolQ},
        From:  &from,
        Size:  &size,
    }

    res, err := li.client.Search().Index("logs-*").Request(req).Do(ctx)
    if err != nil {
        return nil, 0, fmt.Errorf("es search logs: %w", err)
    }

    total := res.Hits.Total.Value
    entries := make([]domain.LogEntry, 0, len(res.Hits.Hits))
    for _, hit := range res.Hits.Hits {
        var e domain.LogEntry
        if err := json.Unmarshal(hit.Source_, &e); err != nil {
            li.logger.Warn("unmarshal log hit", zap.Error(err))
            continue
        }
        e.ID = hit.Id_
        entries = append(entries, e)
    }
    return entries, total, nil
}
```

### Structured Error Response

```go
type ErrorResponse struct {
    Error  string `json:"error"`
    Status int    `json:"status"`
}

func writeError(w http.ResponseWriter, status int, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(ErrorResponse{Error: msg, Status: status})
}
```

### Handler with Full Error Discrimination

```go
func (s *Server) handleGetLog(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if id == "" {
        writeError(w, http.StatusBadRequest, "missing id")
        return
    }
    entry, err := s.logStore.GetLog(r.Context(), id)
    if err != nil {
        if errors.Is(err, domain.ErrNotFound) {
            writeError(w, http.StatusNotFound, "log entry not found")
            return
        }
        s.logger.Error("get log failed", zap.Error(err), zap.String("id", id))
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }
    writeJSON(w, http.StatusOK, entry)
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| olivere/elastic (community client) | `go-elasticsearch/v9` TypedClient | ES v8+ | Official client; TypedClient gives compile-time checked types |
| Raw JSON body building for ES queries | `types.BoolQuery`, `types.TermQuery` structs | go-elasticsearch v8.5+ | No string/JSON assembly; IDE completion |
| gorilla/mux router | chi v5 | ~2017 (ecosystem shift) | No external dependencies; stdlib-compatible |
| manual WaitGroup for multi-server | `errgroup.WithContext` | golang.org/x/sync 0.1+ | Context propagation + error forwarding built in |
| `response.Status` for ES 404 | `response.Found == false` | go-elasticsearch v8.8.2+ | Clean boolean; no string status parsing needed |

**Deprecated/outdated:**
- `middleware.Logger` for Zap projects: Use custom Zap middleware wrapping `middleware.NewWrapResponseWriter` instead.
- Scroll API for pagination: Use `from`/`size` for <10,000 docs; use `search_after` + PIT for deeper pagination (v1 caps at 1000, so `from`/`size` is fine).

---

## Open Questions

1. **API server port**
   - What we know: `MetricsConfig.Port` defaults to 2112 in config.go; main.go already runs a metrics-only server there.
   - What's unclear: Should the chi API server share port 2112 (replacing the old server), or run on a new port (e.g., 8080)? The plan description says "Run API server in its own goroutine" and mount `/metrics` on chi — implying the old server is replaced.
   - Recommendation: Add `api.port` to config (default 8080); move `/metrics` to chi router; remove old `http.Server` from main.go. Update `Config` struct to add `APIConfig`.

2. **`ErrNotFound` sentinel location**
   - What we know: `internal/domain/interfaces.go` has no sentinel errors defined; `GetLog` / `GetAnomaly` stubs return `errors.New("not implemented")`.
   - What's unclear: Should the sentinel live in `internal/domain` or `internal/elasticsearch`?
   - Recommendation: Define `var ErrNotFound = errors.New("not found")` in `internal/domain` (domain package is the contract layer).

3. **`AnomalyQuery.Type` field semantics**
   - What we know: `domain.AnomalyQuery` has `Type string`; ES mapping has `rule_id` keyword field.
   - What's unclear: Does `Type` filter on `rule_id` or is there a separate `type` field?
   - Recommendation: Map `AnomalyQuery.Type` → ES `rule_id` term filter (consistent with `Anomaly.RuleID` field in domain types).

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | stdlib `testing` + `github.com/stretchr/testify` v1.11.1 |
| Config file | none (no separate config file needed for unit tests) |
| Quick run command | `go test -race ./internal/api/... ./internal/elasticsearch/...` |
| Full suite command | `go test -race ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| API-01 | `GET /api/v1/logs` filters and pagination | unit (httptest) | `go test -race ./internal/api/... -run TestHandleListLogs` | ❌ Wave 0 |
| API-02 | `GET /api/v1/logs/{id}` returns 200 or 404 | unit (httptest) | `go test -race ./internal/api/... -run TestHandleGetLog` | ❌ Wave 0 |
| API-03 | `GET /api/v1/anomalies` filters and pagination | unit (httptest) | `go test -race ./internal/api/... -run TestHandleListAnomalies` | ❌ Wave 0 |
| API-04 | `GET /api/v1/anomalies/{id}` returns 200 or 404 | unit (httptest) | `go test -race ./internal/api/... -run TestHandleGetAnomaly` | ❌ Wave 0 |
| API-05 | `/health` 200/503; `/ready` 200/503 | unit (httptest) | `go test -race ./internal/api/... -run TestHandleHealth` | ❌ Wave 0 |
| API-06 | `/metrics` returns Prometheus text | unit (httptest) | `go test -race ./internal/api/... -run TestHandleMetrics` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test -race ./internal/api/...`
- **Per wave merge:** `go test -race ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `internal/api/logs_test.go` — covers API-01, API-02 (httptest + mockLogStore)
- [ ] `internal/api/anomalies_test.go` — covers API-03, API-04 (httptest + mockAnomalyStore)
- [ ] `internal/api/health_test.go` — covers API-05 (httptest + mock ES ping)
- [ ] `internal/api/server_test.go` — TestMain with goleak.VerifyTestMain

*(chi v5 must be added to go.mod before test files compile)*

---

## Sources

### Primary (HIGH confidence)

- `pkg.go.dev/github.com/go-chi/chi/v5` — Router API, URLParam, middleware list, version v5.2.5 confirmed
- `pkg.go.dev/github.com/go-chi/chi/v5/middleware` — All middleware function signatures (Recoverer, RequestID, Timeout, Logger, NewWrapResponseWriter)
- `github.com/elastic/go-elasticsearch/issues/678` — TypedClient Get 404 resolution: `response.Found` is correct, fixed in v8.8.2+
- `elastic.co/search-labs/blog/perform-text-queries-with-the-elasticsearch-go-client` — `hit.Source_` unmarshal pattern, `types.Query` struct, `.From()/.Size()` chaining
- `pkg.go.dev/github.com/elastic/go-elasticsearch/v9/typedapi/core/search` — search.Request, From/Size, HitsMetadata structure
- `pkg.go.dev/github.com/elastic/go-elasticsearch/v9/typedapi/core/get` — Get API, Found field
- Go module proxy (`proxy.golang.org`) — chi v5.2.5 confirmed 2026-02-05
- Project codebase (`internal/domain/types.go`, `interfaces.go`, `elasticsearch/*.go`) — domain types, interface contracts, existing stub implementations

### Secondary (MEDIUM confidence)

- `elastic.co/docs/reference/elasticsearch/clients/go/using-the-api/searching` — search.Request structure with From/Size and BoolQuery filter pattern
- `pkg.go.dev/golang.org/x/sync/errgroup` — errgroup.WithContext pattern for HTTP server lifecycle

### Tertiary (LOW confidence)

- WebSearch community results on chi+Zap middleware — custom WrapResponseWriter middleware pattern (standard pattern, low risk but not from official chi docs)

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — chi v5.2.5 confirmed via Go proxy; all other deps already in go.mod
- Architecture: HIGH — domain interfaces and store stubs are already defined; ES TypedClient API verified
- Pitfalls: HIGH — ES Get wildcard limitation, ErrNotFound sentinel, pagination offset formula verified from official sources
- ES query construction: MEDIUM — BoolQuery/TermQuery/DateRangeQuery verified from official docs and blog; `DateRangeQuery` in Range map confirmed from TypedAPI type inspection

**Research date:** 2026-03-22
**Valid until:** 2026-04-22 (chi and go-elasticsearch are stable; 30-day window)
