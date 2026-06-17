# Chapter 10 · Go essentials, by example

> Goal: by the end of this chapter you can write idiomatic Go for a backend service — even outside this project. We cover what makes Go different, the standard library features you'll lean on every day (context, errors, slog), interface patterns, dependency injection without a framework, and testing.

Read this if you've used another language (Python, JS, Java) but are new to Go. If you already know Go, skim for the patterns specific to this codebase.

## The Go philosophy in 5 bullets

1. **Small surface, big stdlib.** The language is small (no generics until 1.18, no exceptions, no inheritance, no decorators). The standard library is huge and good.
2. **Composition over inheritance.** Embed structs to reuse fields; embed interfaces to satisfy them by piece.
3. **Errors are values.** Functions return `(result, error)`. No `try/catch`. You check the error every call. This sounds tedious; it makes failure paths visible.
4. **Concurrency primitives in the language.** Goroutines + channels + `select`. You don't import a framework for it.
5. **Build to a single binary.** No runtime. No node_modules-equivalent at runtime. Static compilation, fast startup, easy deploy.

Together, these make Go ideal for backend services: predictable performance, fast deploys, easy ops. The tradeoff is verbosity (you'll write more characters than equivalent Python). The codebase you're reading is ~30k lines of Go that does what Python would do in maybe 12k — but the Go version starts in 50ms instead of 800ms and uses 60MB of RAM instead of 250MB.

## Project layout conventions

Go has a strong convention (not enforced by the toolchain):

```
yourproject/
├── cmd/<binary>/main.go     ← each binary gets its own subdir
├── internal/                ← code only this project can import
├── pkg/                     ← public libraries this project exposes
├── go.mod                   ← module name + dep list
└── go.sum                   ← checksum of every dep version
```

GOGG adapts this for a monorepo:

```
gogg/
├── apps/{api,worker,web}/   ← each binary's tree (cmd/, internal/)
├── packages/                ← shared libraries (used by multiple binaries)
│   ├── domain/              ← shared types (Champion, Tier, Region)
│   ├── sqlc/                ← SQL + generated query bindings
│   └── riotapi/             ← Riot API client
├── internal/                ← LEGACY single-binary code
├── go.work                  ← workspace combining all modules
└── go.mod                   ← root module (gogg)
```

💡 **`internal/` is special.** Any package under `<module>/internal/` can only be imported by code in `<module>`. It's how you mark "implementation detail, don't depend on this from outside." Most projects abuse this aggressively to keep their public API small. You'll see it in this codebase: `apps/api/internal/...` is invisible to `apps/worker/...`.

💡 **`pkg/` and `packages/` vs `internal/`.** Public-by-intent code goes in `pkg/` (single-module convention) or `packages/` (workspace convention). GOGG uses `packages/` because the workspace setup expects each shared module at the top level.

Look at the workspace file:

```bash
cat go.work
```

It lists each module that lives in this repo. Tooling (`go build`, `go test`, IDEs) treats them as one logical thing while keeping module-level boundaries.

## Structs + methods

Go has no classes. Everything is a struct + functions that act on it. Methods are functions with a "receiver":

```go
type Issuer struct {
    secret []byte
    iss    string
}

// Pointer receiver — can mutate the struct + avoids copying.
func (i *Issuer) IssuePair(userID string) (Pair, error) {
    // ...
}
```

Two flavors of receiver:

- **Pointer receiver** `func (x *Foo) Bar()`: caller's `Foo` can be mutated; no copy on call.
- **Value receiver** `func (x Foo) Bar()`: caller's `Foo` is copied; safe for concurrent reads but you can't mutate the original.

Rule of thumb: use a pointer receiver unless the type is truly immutable + small (e.g. `time.Time`, custom int types). Consistency matters — pick one per type.

### Constructors

Go has no `new` operator for structs (well, `new` exists but is rarely idiomatic). Instead:

```go
func NewIssuer(secret []byte, iss string) (*Issuer, error) {
    if len(secret) < 32 {
        return nil, errors.New("secret must be at least 32 bytes")
    }
    return &Issuer{secret: secret, iss: iss}, nil
}
```

Notice: returns `(*Issuer, error)`. Constructors that can fail use this pattern. Constructors that can't fail return just `*Issuer`.

🛠️ **Exercise**: grep the codebase:

```bash
grep -rn 'func New[A-Z][A-Za-z]*(' apps/api/internal/ | head -20
```

You'll see ~30 `NewX` constructors. That's how nearly every dependency gets created.

## Interfaces — the most important concept

Go's interfaces are **structural** (duck-typed). A type satisfies an interface if it has the right methods, without declaring "implements." That's huge.

### Define small interfaces near the consumer

```go
// In apps/api/internal/transport/rest/v1/v1.go
type CatalogService interface {
    ListVersionsWithData(ctx context.Context) ([]string, error)
    ListRegionsWithData(ctx context.Context) ([]string, error)
}

func Routes(catalog CatalogService, rkn RankingsService) chi.Router {
    // ...
}
```

The interface is defined **where it's used** (the transport package), not in the package that implements it. This is the Go inverse of Java's "define interfaces near the implementer."

Why? Because the transport package only knows what it needs. The `catalog` package's `*Service` happens to have those methods plus a dozen others (caching, logging, etc.). The transport doesn't care.

This is called "accept interfaces, return concretes":

```go
// Constructor returns the concrete type.
func New(queries *sqlcgen.Queries, cache cache.Cache) *Service {
    return &Service{queries: queries, cache: cache}
}

// Consumer accepts an interface.
func versionsHandler(svc CatalogService) http.HandlerFunc {
    // ...
}
```

The caller wires them up:

```go
catalogSvc := catalog.New(queries, cacheClient)   // returns *catalog.Service
routes := v1.Routes(catalogSvc, rankingsSvc)      // accepts CatalogService
```

The `*catalog.Service` is assigned to `CatalogService` automatically because it has the right methods.

🛠️ **Exercise**: grep for interface declarations near the call site:

```bash
grep -rn 'type [A-Z][A-Za-z]* interface' apps/api/internal/transport/ | head
```

You'll see interfaces *named for the role* (`CatalogService`, `RankingsService`) declared in the file that uses them. That's the convention.

### Empty interface `any`

`interface{}` (alias `any` since Go 1.18) is the "I don't know the type" escape hatch. It's used for JSON marshaling, generic containers (pre-1.18), and Temporal SDK signatures. Avoid it in your own APIs unless necessary — you lose type safety.

### Type assertions + switches

```go
var iface any = someValue

// Single assertion (panics if wrong type)
str := iface.(string)

// Safe assertion with ok
str, ok := iface.(string)
if ok { /* use str */ }

// Type switch
switch v := iface.(type) {
case string:
    fmt.Println("a string:", v)
case int:
    fmt.Println("an int:", v)
default:
    fmt.Println("something else")
}
```

You'll see type switches in error handling (`errors.As`) and in places like `apps/api/internal/transport/graphql/error_presenter.go`.

## Context — the implicit "request scope"

Every long-ish operation in Go takes a `context.Context` as the first parameter. It's a tree-shaped object carrying:

- **Cancellation** — when `ctx.Done()` channel fires, callees should stop.
- **Deadline** — `ctx.Deadline()` is "stop by this absolute time."
- **Values** — `ctx.Value(key)` for request-scoped data (request ID, current user).

```go
func (s *Service) ListVersionsWithData(ctx context.Context) ([]string, error) {
    return s.queries.ListVersionsWithData(ctx)
}
```

Every layer passes ctx down. When the user closes their browser, chi cancels the request context, the cancellation propagates all the way to `pgx`, which kills the in-flight SQL query. No work wasted.

### Create your own context

```go
// Add a timeout — child context inherits from parent, then adds its own deadline.
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()    // ALWAYS defer cancel to release resources

// Add a value — read-only, for the duration of this request.
ctx = context.WithValue(ctx, contextKeyUserID, user.ID)
```

### Rules

- ✅ Pass ctx as the first parameter to any function that does I/O.
- ✅ `defer cancel()` whenever you create a context with `WithCancel/WithTimeout/WithDeadline`.
- ❌ Don't store ctx in a struct (it's per-call, not per-instance).
- ❌ Don't pass `context.Background()` deep into call chains. The HTTP handler has a context; use it.
- ❌ Don't pass `nil` as a context. Use `context.TODO()` if you genuinely don't have one yet.

The classic bug in the legacy stack (and one of the things Phase B fixed): `internal/server/versions.go` had `context.Background()` hardcoded inside a request handler. Cancellation didn't propagate; SQL queries kept running after the client gave up. Phase B threaded `r.Context()` from the handler all the way to `sqlc`.

🛠️ **Exercise**: grep for `context.Background()` outside `main.go` and tests:

```bash
grep -rn 'context.Background()' apps/ packages/ | grep -v '_test.go' | grep -v 'main.go'
```

If something shows up that isn't a clear top-of-binary entry point, that's a context-propagation bug.

## Error handling

Go errors are values. The `error` interface:

```go
type error interface {
    Error() string
}
```

Anything with an `Error() string` method satisfies it. You return errors and check them:

```go
versions, err := s.queries.ListVersionsWithData(ctx)
if err != nil {
    return nil, err
}
```

This is the most-typed pattern in Go. Embrace it.

### Wrap errors with context

Plain forwarding loses information. Add context with `fmt.Errorf("%w", err)`:

```go
versions, err := s.queries.ListVersionsWithData(ctx)
if err != nil {
    return nil, fmt.Errorf("list versions: %w", err)
}
```

The `%w` verb wraps the underlying error so it stays unwrappable.

### errors.Is + errors.As

To check if an error is (or wraps) a specific value:

```go
if errors.Is(err, pgx.ErrNoRows) {
    // Treat as "not found"
    return nil, ErrNotFound
}
```

To extract a wrapped error of a specific type:

```go
var apiErr *riot.APIError
if errors.As(err, &apiErr) {
    if apiErr.StatusCode == 429 {
        // Rate limited
    }
}
```

### Sentinel errors

Define package-level errors as sentinels:

```go
var ErrNotFound = errors.New("not found")
```

Callers use `errors.Is(err, pkg.ErrNotFound)`.

### Don't panic in libraries

`panic` is for "the program is in an unrecoverable state" — like main.go's startup phase failing. Library code returns errors. The web framework will catch panics via `chi.Recoverer` middleware so a panic doesn't take down the process — but you should never *rely* on that.

🛠️ **Exercise**: open `apps/api/cmd/api/main.go` and find the `run() error` function. The pattern: `main` is a thin wrapper that calls `run()`, prints any error, and exits with a nonzero code. The actual work is in `run()`, where errors return up the stack instead of `log.Fatal`. This separation makes the entry point testable.

```bash
grep -A 5 'func main' apps/api/cmd/api/main.go
```

## Structured logging with slog

`log/slog` is the standard structured logger (since Go 1.21):

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
logger.Info("user_signed_in", "user_id", userID, "provider", "discord")
```

The first arg is the **event name** (a stable identifier like `user_signed_in`, not a sentence). The rest are key/value pairs.

Why "event name + attributes" instead of "f-string with vars"? Because logs become queryable: `slog | grep event=user_signed_in | jq .provider`. Cardinality of event names is bounded; that's what makes log search fast.

GOGG uses snake_case event names by convention. Search:

```bash
grep -rEo 'slog\.[A-Z][a-z]*\(\s*\".+?\"' apps/ | head -20
```

You'll see `http_server_started`, `secrets_decrypt_failed`, etc. — all snake_case, all noun-form.

### Add context to a logger

```go
reqLogger := logger.With("request_id", reqID, "path", r.URL.Path)
reqLogger.Info("handling")
```

`With(...)` returns a new logger with those attributes baked in. Every subsequent log line gets them. This is how the request-scoped logger gets the request ID into every log line for a request.

## Goroutines + channels

You won't write many goroutines in this codebase — Temporal handles concurrency in the worker, and HTTP handlers already run in their own goroutine per request. But the basics:

```go
go someFunc()           // start a goroutine
```

```go
ch := make(chan int, 10)
go func() {
    ch <- 42
}()
val := <-ch             // blocks until value available
```

```go
select {
case val := <-ch:
    fmt.Println(val)
case <-ctx.Done():
    return ctx.Err()
case <-time.After(5*time.Second):
    return errors.New("timeout")
}
```

### sync.atomic

The Phase C race-fix lesson:

```go
var calls atomic.Int64
calls.Add(1)
n := calls.Load()
```

Use atomics for counters that are touched by multiple goroutines (and that's the only thing they're touched by). For more complex shared state, use `sync.Mutex`. For producer/consumer, use channels.

🛠️ **Exercise**: read `apps/worker/internal/workflow/crawl/workflow_test.go` — find the `atomic.Int64` usage in the pipeline test. The plain `int++` version raced in CI. The fix is one of the smallest possible changes that shows up in `git log`.

## pgx + connection pools

`pgx` is the Postgres driver Go projects use (the stdlib `database/sql` is slower + less type-aware). sqlc generates code that uses `pgx/v5` directly.

```go
pool, err := pgxpool.New(ctx, dsn)
if err != nil { return err }
defer pool.Close()
```

The pool is a pool of connections — when you `Acquire` one, use it for a query, release it back. sqlc's generated code does this for you.

Connection lifecycle:

- Pool min/max idle configurable
- A connection that errors out is auto-evicted
- Cancellation from `ctx` aborts the underlying query

🛠️ **Exercise**: look at `apps/api/cmd/api/main.go`'s pool construction:

```bash
grep -A 5 'pgxpool.NewWithConfig\|pgxpool.New' apps/api/cmd/api/main.go
```

That's the single source of every Postgres connection used by the API binary.

## Dependency injection without a framework

Spring (Java), nestjs (Node), FastAPI (Python) — they all have DI containers. Go projects almost never use one. The pattern is **explicit constructor wiring in main**:

```go
// in main / run()
logger := buildLogger(cfg)
pool, err := buildPool(ctx, cfg)
if err != nil { return err }
defer pool.Close()
redisClient, err := buildRedis(ctx, cfg)
if err != nil { return err }
queries := sqlcgen.New(pool)
cacheClient := cache.New(redisClient, logger)
catalogSvc := catalog.New(queries, cacheClient, logger)
rankingsSvc := rankings.New(queries, cacheClient, logger)
authIssuer := auth.NewIssuer(cfg.JWTSecret, cfg.Issuer)
// ...
handler := buildRouter(cfg, logger, catalogSvc, rankingsSvc, authIssuer)
```

Every dependency is explicit. There's no magic. Reading `main.go` tells you exactly what depends on what.

The downside: `main` gets long. The upside: refactoring is mechanical, the dependency graph is always visible, and there's no autowiring debugging session.

This is the **most important pattern** for backend Go projects to grok. If you read `cmd/api/main.go` start to finish, you've read the dependency graph of the entire service.

## Testing

Go tests live next to the code in `*_test.go` files:

```go
// apps/api/internal/auth/jwt_test.go

func TestNewIssuer_RejectsShortSecret(t *testing.T) {
    if _, err := NewIssuer("short", time.Minute, time.Hour, "test"); err == nil {
        t.Fatal("expected error for short secret")
    }
}
```

Run with `go test ./...`. Parallel by default at the package level; opt-in within (`t.Parallel()`).

### Table-driven tests

The idiomatic pattern for testing variations:

```go
func TestParse(t *testing.T) {
    cases := []struct {
        name    string
        input   string
        want    int
        wantErr bool
    }{
        {"empty", "", 0, true},
        {"single", "5", 5, false},
        {"negative", "-3", -3, false},
    }
    for _, c := range cases {
        c := c
        t.Run(c.name, func(t *testing.T) {
            t.Parallel()
            got, err := Parse(c.input)
            if (err != nil) != c.wantErr {
                t.Fatalf("err=%v wantErr=%v", err, c.wantErr)
            }
            if got != c.want {
                t.Errorf("got %d want %d", got, c.want)
            }
        })
    }
}
```

You see this everywhere in the codebase. The `c := c` line is the loop-variable capture workaround pre-Go 1.22 (the `for` loop's variable is reused; capturing it without that line means every parallel `t.Run` references the same `c`).

### testify

For richer assertions, the codebase uses `testify`:

```go
require.NoError(t, err)
require.Equal(t, int64(3), counter.Load())
require.GreaterOrEqual(t, calls.Load(), int64(1))
```

`require.X` halts the test on failure; `assert.X` continues. Use `require` for setup invariants, `assert` for per-case checks.

### Mocking

`testify/mock` provides a mock framework. The Phase C workflow test uses it:

```go
var acts *crawl.Activities
env.OnActivity(acts.CreateRun, mock.Anything, mock.Anything).Return(
    crawl.CreateRunOutput{RunID: 42}, nil)
```

The `acts *crawl.Activities` (typed nil) is a trick: you can pass a `nil` of a struct pointer type as an activity reference because Temporal uses the *type* (method symbol) to find the activity, not the value.

### Integration tests + build tags

Tests that need a real database are tagged:

```go
//go:build integration
// +build integration

package phase3_test
```

Run with `go test -tags=integration ./...`. The `make test-int` target does this. CI runs them separately.

### Race detector

`go test -race` instruments memory access to detect races. CI uses it for the worker package. Locally:

```bash
go test -race ./apps/worker/internal/workflow/...
```

If your test counter is a plain `int` written from multiple goroutines, the race detector finds it. Use atomics.

## Patterns specific to this codebase

### `sqlc-skip` comments

When you must write raw pgx (dynamic SQL), leave:

```go
// sqlc-skip: WHERE clause is filter-built; can't statically describe in sqlc.
rows, err := tx.Query(ctx, query, args...)
```

That comment is the convention that prevents reviewers from assuming you forgot to run codegen.

### Functional options

A pattern for variable construction parameters:

```go
type RouterOption func(*routerConfig)

func WithMetrics(reg prometheus.Registerer) RouterOption {
    return func(c *routerConfig) { c.metrics = reg }
}

func NewRouter(opts ...RouterOption) *chi.Mux {
    cfg := defaultRouterConfig()
    for _, opt := range opts { opt(cfg) }
    // ...
}
```

You'll see this in middleware setup and Temporal worker options. The advantage over a giant struct: forward-compatibility (adding a field doesn't break callers).

### Don't return interfaces, accept them

```go
// BAD
func NewCatalog(queries *Queries) CatalogService

// GOOD
func NewCatalog(queries *Queries) *Catalog
```

Returning a concrete type lets callers see all methods + benefit from IDE go-to-def. The interface is for the consumer to declare what they need, not for the producer to constrain themselves.

## Tooling you should know

| Tool | What |
|---|---|
| `gofmt`/`goimports` | Auto-format. Editor integration mandatory. |
| `go vet` | Static analysis, catches common bugs. Run before commit. |
| `golangci-lint` | Aggregated linters. Run before push. CI gates on it. |
| `go mod tidy` | Sync `go.mod` + `go.sum` with the actual imports. |
| `go test -cover` | Coverage report. |
| `go test -race` | Race detector. |
| `pprof` | CPU + heap profiler. `import _ "net/http/pprof"` for live profiles. |
| `delve` (`dlv`) | Debugger. Use it sparingly; printf-debug is faster in Go. |

## Going further

The Go community's canonical resources:

- [Effective Go](https://go.dev/doc/effective_go) — official style guide. Read top-to-bottom once.
- [Go by Example](https://gobyexample.com/) — pattern catalog with runnable snippets.
- [Standard Library docs](https://pkg.go.dev/std) — the stdlib is your toolkit. Bookmark `context`, `errors`, `log/slog`, `net/http`, `encoding/json`, `time`.
- [Go Proverbs](https://go-proverbs.github.io/) — Rob Pike's 1-liners. Short. Re-read every few months.

For backend-Go in particular:

- [Mat Ryer — How I structure HTTP services](https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/) — close to what GOGG does.
- [Peter Bourgon — Standard Package Layout](https://peter.bourgon.org/go-best-practices-2016/) — older but timeless.
- [Dave Cheney — On Go](https://dave.cheney.net/category/golang) — opinions on Go practices, all worth reading.

## Up next

[Chapter 11 — Frontend essentials](./11-frontend-essentials.md) does the same thing for TypeScript + React + the modern web stack: how to be productive in `apps/web/` and in any similar codebase.
