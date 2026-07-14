## gogg-api 启动流程

### 1.初始化 Config
```go
// ---- /api/cmd/main.go ----
cfg, err := config.Load()
if err != nil {
    return fmt.Errorf("load config: %w", err)
}

// ---- /api/internal/config/config.go ----
// Config is the full runtime configuration for gogg-api.
type Config struct {
	API      APIConfig      `mapstructure:"api"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Auth     AuthConfig     `mapstructure:"auth"`
	OAuth    OAuthConfig    `mapstructure:"oauth"`
}
```

### 2. 初始化 Logger
```go
// ---- /api/cmd/main.go ----
logger := newLogger(cfg.Logging)   // slog.Logger
slog.SetDefault(logger)
logger.Info("starting",
    "version", version, "commit", commit, "build_date", buildDate,
    "port", cfg.API.Port, "log_level", cfg.Logging.Level,
)
```

### 3. 初始化 rootCtx
```go
// ---- /api/cmd/main.go ----
// 用于监听系统信号，优雅退出
// SIGINT: Ctrl+C 中断信号
// SIGTERM: 终止信号,由 kill 命令发送
rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()
```

### 4. 初始化 Database
```go
// ---- /api/cmd/main.go ----
// context.WithTimeout 用于设置连接数据库的超时时间，避免连接数据库时阻塞过久
dbCtx, cancel := context.WithTimeout(rootCtx, 10*time.Second)
defer cancel()
pool, err := connectDB(dbCtx, cfg.Database)   // *pgxpool.Pool
if err != nil {
    return fmt.Errorf("connect db: %w", err)
}
defer pool.Close()
logger.Info("db_connected", "max_open_conns", cfg.Database.MaxOpenConns)

// connectDB
// 按照config.Config.Database初始化*pgxpool.Config
// 然后用*pgxpool.Config创建*pgxpool.Pool (dbCtx)
// 然后Ping数据库，确保连接成功 （dbCtx）
```

### 5. 初始化 Redis
```go
// ---- /api/cmd/main.go ----
	var redisClient *cache.Redis
	if cfg.Redis.URL != "" {
		redisClient, err = cache.NewRedis(cfg.Redis.URL)
		if err != nil {
			return fmt.Errorf("init redis: %w", err)
		}
		defer func() { _ = redisClient.Close() }()
		pingCtx, pingCancel := context.WithTimeout(rootCtx, 3*time.Second)
		if err := redisClient.Ping(pingCtx); err != nil {
			pingCancel()
			return fmt.Errorf("ping redis: %w", err)
		}
		pingCancel()
		logger.Info("redis_connected")
	}

// ---- /api/internal/cache/redis.go ----
// Cache.Redis 是一个封装了 go-redis/v9 的客户端，用于与 Redis 服务器进行通信
// 它还使用 singleflight.Group 来防止缓存击穿，把同一个 key 的多个请求合并成一个请求
// 封装了GetJson和SetJson方法，方便存取JSON数据
type Redis struct {
	client *redis.Client   // redis.Client 是 go-redis/v9 的客户端，用于与 Redis 服务器进行通信
	sf     singleflight.Group // singleflight.Group 用于防止缓存击穿，把同一个 key 的多个请求合并成一个请求
}
```

# 6. Build Router(handler) and Server
```go
handler := buildRouter(cfg, logger, pool, redisClient)
srv := &http.Server{
    Addr:              ":" + strconv.Itoa(cfg.API.Port),
    Handler:           handler,
    ReadTimeout:       cfg.API.ReadTimeout,
    ReadHeaderTimeout: 5 * time.Second,
    WriteTimeout:      cfg.API.WriteTimeout,
    IdleTimeout:       cfg.API.IdleTimeout,
    BaseContext:       func(_ net.Listener) context.Context { return rootCtx },
}

// go internal package
type Handler interface {
	ServeHTTP(ResponseWriter, *Request)
}
```

# 7. 开始监听 HTTP 请求
```go
// ---- /api/cmd/main.go ----
errCh := make(chan error, 1)
go func() {  // 单独的 goroutine 来监听 HTTP 请求，避免阻塞主线程
	logger.Info("listening", "addr", srv.Addr)
	errCh <- srv.ListenAndServe()
}()
```

# 8. 等待系统信号或者 HTTP 监听错误
```go
// ---- /api/cmd/main.go ----
select {
case <-rootCtx.Done():		// SIGINT or SIGTERM
	logger.Info("shutdown_signal_received")
case err := <-errCh:		// ListenAndServe() 返回错误
	if !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}
}
```

# 9. 优雅退出 HTTP Server
```go
shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.API.ShutdownGrace)
defer cancel()
if err := srv.Shutdown(shutdownCtx); err != nil {
	logger.Error("graceful_shutdown_failed", "err", err)
	return fmt.Errorf("shutdown: %w", err)
}
logger.Info("stopped_cleanly")
return nil
```

## Go 语言 Server， Handler，Router，Middleware 之间的关系

### 1. Server
Server来自"net/http"包，表示一个HTTP服务器。它包含了监听地址、处理请求的Handler、读写超时时间等配置。
Server主要负责监听HTTP请求，管理网络连接，并将请求分发给Handler进行处理。
Server主要忙于下面的事情：
- 监听与端口绑定 (Listening)
- 建立与维护连接 (Connection Management)
	- 当有客户端连接进来时，Server 负责处理 TCP 三次握手，为每个客户端建立起网络连接
	- 并发处理：每来一个新连接，Server 内部就会自动启动一个新的 goroutine（Go 协程）去处理它。这使得 Go 的 HTTP 服务天然具备极高的并发处理能力。
	- 连接复用：Server 支持 HTTP/1.1 的 Keep-Alive 和 HTTP/2 的多路复用，这意味着同一个 TCP 连接可以被多个请求复用，从而减少了连接建立和关闭的开销。
- 严格的时间控制 (Timeouts)
	- ReadTimeout：限制从客户端读取请求的时间，防止慢速客户端拖慢服务器。
	- ReadHeaderTimeout：限制读取请求头的时间，防止慢速客户端拖慢服务器。
	- WriteTimeout：限制向客户端写入响应的时间，防止慢速客户端拖慢服务器。
	- IdleTimeout：限制空闲连接的最大时间，防止资源
- 协议解析与对象转换 (Parsing)
	- Server 会解析 HTTP 请求，将原始的字节流转换为 http.Request 对象，方便 Handler 使用。
	- 同样，Server 会将 http.ResponseWriter 对象传递给 Handler，Handler 可以通过它来构建和发送 HTTP 响应。
```go
handler := buildRouter(cfg, logger, pool, redisClient)
srv := &http.Server{
	Addr:              ":" + strconv.Itoa(cfg.API.Port),
	Handler:           handler,
	ReadTimeout:       cfg.API.ReadTimeout,
	ReadHeaderTimeout: 5 * time.Second,
	WriteTimeout:      cfg.API.WriteTimeout,
	IdleTimeout:       cfg.API.IdleTimeout,
	BaseContext:       func(_ net.Listener) context.Context { return rootCtx },
}
```

2. Handler
Handler是一个`http.Handler`接口，只要实现了 ServeHTTP(w, r) 方法的对象都可以是 Handler。Server 收到网络请求后，会将请求封装成 http.Request 对象，并调用 Handler 的 ServeHTTP 方法来处理请求。
```go
type Handler interface {
	ServeHTTP(ResponseWriter, *Request)
}

type ResponseWriter interface {
	Header() Header
	Write([]byte) (int, error)
	WriteHeader(statusCode int)
}
```
关于此处ResponseWriter和Request的说明：
- ResponseWriter：是一个接口，提供了向客户端发送HTTP响应的方法。Handler通过它来写入响应头和响应体。
- Request：是一个结构体，封装了HTTP请求的所有信息，包括请求方法、URL、头部信息、查询参数、请求体等。Handler通过它来读取客户端发送的请求数据。

### 3. Router
Router是一个特殊的Handler，它负责根据请求的URL路径和HTTP方法，将请求分发给对应的处理函数（HandlerFunc）。Router通常会维护一个路由表，记录每个URL路径和HTTP方法对应的处理函数。当Server收到请求时，会将请求交给Router，Router根据路由表找到对应的处理函数，并调用它来处理请求。
这里的Router是一个`*chi.Mux`对象，它实现了`http.Handler`接口，因此可以直接作为Server的Handler使用。
```go
r := chi.NewRouter()
```

## Prometheus 介绍
```go
type Metric interface {
	Desc() *Desc
	Write(*dto.Metric) error

}

type Desc struct {
	fqName string
	help string
	constLabelPairs []*dto.LabelPair
	variableLabels *compiledLabels
	id uint64
	dimHash uint64
	err error
}

type Collector interface {
	Describe(chan<- *Desc)
	Collect(chan<- Metric)
}


type Registry struct {
	mtx                   sync.RWMutex
	collectorsByID        map[uint64]Collector // ID is a hash of the descIDs.
	descIDs               map[uint64]struct{}
	dimHashesByName       map[string]uint64
	uncheckedCollectors   []Collector
	pedanticChecksEnabled bool
}
```

### 1. Registry
Registry是Prometheus的核心组件之一，它负责管理和注册各种指标（metrics）。Registry维护了一个收集器（Collector）的列表，每个收集器负责收集特定类型的指标数据。

### 2. Collector
Collector会维护一个或多个指标（Metric），并实现了Describe和Collect方法。Describe方法用于描述指标的元数据，而Collect方法用于收集指标的实际数据。

### 3. Metric
Metric是Prometheus中最基本的概念，表示一个具体的指标数据点。每个Metric都有一个描述（Desc），用于说明指标的名称、帮助信息和标签等。Metric可以是计数器（Counter）、仪表盘（Gauge）、直方图（Histogram）或摘要（Summary）等类型。

### 4. Prometheus的工作流程
1. 应用程序在运行时创建一个Registry对象，并注册各种Collector。
2. 当Prometheus服务器向应用程序发送抓取请求时，Registry会调用每个Collector的Collect方法，收集所有注册的指标数据。
3. 收集到的指标数据会被打包成Prometheus的格式，并返回给Prometheus服务器进行存储和分析。
4. Prometheus服务器可以根据这些指标数据生成图表、告警规则和仪表盘，帮助开发者监控应用程序的性能和健康状态。



## BuildRuoter 流程

### 1. 初始化 chi.Router
```go
// ---- /api/internal/api/main.go ----
r := chi.NewRouter()
```

### 2. 初始化JWT Issuer
```go
// ---- /api/internal/api/main.go ----
var issuer *auth.Issuer
if cfg.Auth.JWTSecret != "" {
	var err error
	issuer, err = auth.NewIssuer(cfg.Auth.JWTSecret, cfg.Auth.AccessTTL, cfg.Auth.RefreshTTL, cfg.Auth.Issuer)
	if err != nil {
		// Misconfigured secret at startup is fatal — log loudly and
		// return a router that 503s everything so the failure is
		// visible to k8s readiness probes (which Page on consistent
		// 5xx).
		logger.Error("auth_issuer_init_failed", "err", err)
		return errorRouter(err)
	}
	logger.Info("auth_enabled", "issuer", cfg.Auth.Issuer, "access_ttl", cfg.Auth.AccessTTL, "refresh_ttl", cfg.Auth.RefreshTTL)
} else {
	logger.Warn("auth_disabled_no_jwt_secret")
}

// ---- /api/internal/auth/jwt.go ----
type Issuer struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
}
```

### 3. 注册Prometheus的Collectors
```go
// Prometheus registry: process collectors + our HTTP histograms.
reg := prometheus.NewRegistry()
reg.MustRegister(
	collectors.NewGoCollector(),
	collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
)
metrics := middleware.NewMetrics(reg)

// --

// -- apps/api/internal/transport/middleware/metrics.go --
type MetricsRegistry struct {
	requestDuration *prometheus.HistogramVec
	requestsTotal   *prometheus.CounterVec
	inFlight        prometheus.Gauge
}

func NewMetrics(reg prometheus.Registerer) *MetricsRegistry {
	m := &MetricsRegistry{
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "gogg",
			Subsystem: "api",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latency by method, route, status.",
			// Buckets tuned for an aggregation API: most queries are
			// <50ms cached, <500ms uncached. The long tail catches
			// pathological queries before they're invisible.
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		}, []string{"method", "route", "status"}),

		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "gogg",
			Subsystem: "api",
			Name:      "http_requests_total",
			Help:      "Total HTTP requests by method, route, status.",
		}, []string{"method", "route", "status"}),

		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "gogg",
			Subsystem: "api",
			Name:      "http_requests_in_flight",
			Help:      "In-flight HTTP requests right now.",
		}),
	}
	reg.MustRegister(m.requestDuration, m.requestsTotal, m.inFlight)
	return m
}
```
reg是一个Prometheus的Registry对象，用于注册和管理各种指标（metrics）。在这里，我们注册了两个默认的收集器：
- GoCollector：收集Go运行时的指标，如goroutine数量、内存使用情况等。（Go 内部指标）
- ProcessCollector：收集当前进程的指标，如CPU使用率、文件描述符数量等。 （进程指标）
这两个Collector是自动收集的，每次访问/metrics时，Prometheus会调用它们的Collect方法，收集最新的指标数据。
同时，我们还创建了一个自定义的MetricsRegistry，这里面包含了三个Collector：
- requestDuration：一个HistogramVec，用于记录HTTP请求的延迟时间，按方法、路由和状态码进行分类。(e.g. Name: http_request_duration_seconds, Method: GET, Route: /api/v1/users, Status: 200, Value: 0.123s)
- requestsTotal：一个CounterVec，用于统计HTTP请求的总数，按方法、路由和状态码进行分类。(e.g. Name: http_requests_total, Method: GET, Route: /api/v1/users, Status: 200, Value: 42)
- inFlight：一个Gauge，用于记录当前正在处理的HTTP请求数量。(e.g. Name: http_requests_in_flight, Value: 5)
这三个Collector也需要注册在Prometheus的Registry中，但是它们的Metric需要手动更新，在每次HTTP请求开始和结束时，分别调用MetricsRegistry的相关方法来更新它们的值。
```go

```

### 4. 注册中间件
```go
// ---- /api/internal/api/main.go ----
r.Use(middleware.Recover)
r.Use(middleware.RequestID)
r.Use(middleware.Logger(logger))
r.Use(metrics.Middleware)
r.Use(middleware.CORS(cfg.API.AllowedOrigins))
if issuer != nil {
	r.Use(middleware.Auth(issuer))
}
```
r.Use()方法用于注册中间件，它会将中间件函数添加到Router的中间件链中。中间件是一个函数，它接受一个http.Handler作为参数，并返回一个新的http.Handler。中间件可以在请求到达最终的处理函数之前，对请求进行预处理，或者在响应返回给客户端之前，对响应进行后处理。
```go
func (mx *Mux) Use(middlewares ...func(http.Handler) http.Handler) {
	if mx.handler != nil {
		panic("chi: all middlewares must be defined before routes on a mux")
	}
	mx.middlewares = append(mx.middlewares, middlewares...)
}
```

### 5. 注册ops路由 (healthz，/readyz, /metrics)
ops endpoints是一些用于运维和监控的HTTP接口，它们通常用于检查服务的健康状态、就绪状态以及暴露指标数据。常见的ops endpoints包括：
- /healthz：用于检查服务是否存活，返回200 OK表示服务正常运行
- /readyz：检查服务是否准备好处理请求，返回200 OK表示服务可以处理请求
- /metrics：用于暴露Prometheus指标，返回服务的各种性能指标
"out of band, k8s-facing" role 是指这些ops endpoints是独立于业务逻辑的，它们主要面向Kubernetes等容器编排平台，用于监控和管理服务的健康状态。Kubernetes会定期访问这些接口，以判断服务是否需要重启或扩展。
```go
	r.Get("/healthz", rest.LivenessHandler())

	pingers := []rest.NamedPinger{
		{Name: "db", Pinger: rest.PoolPinger{Pool: pool}},
	}
	if redisClient != nil {
		pingers = append(pingers, rest.NamedPinger{Name: "redis", Pinger: redisClient})
	}
	r.Get("/readyz", rest.ReadinessHandler(pingers...))

	r.Method(http.MethodGet, "/metrics", rest.MetricsHandler(reg))
```
注册路由就是把Method + pattern和对应的HandlerFunc绑定在一起，当请求的Method和pattern匹配时，就会调用对应的HandlerFunc来处理请求。
这里的Method是http Method，例如GET, POST, PUT, DELETE等；pattern是URL路径，例如/healthz, /readyz, /metrics等；HandlerFunc是一个函数，接受http.ResponseWriter和*http.Request作为参数，用于处理请求并返回响应。
这里的第二个参数叫pattern而不是URL，是因为它可以包含动态参数，例如/user/{id}，其中{id}就是一个动态参数，可以通过chi.URLParam(r, "id")来获取它的值。
这里一共注册了三个路由：
- /healthz：用于检查服务是否存活，返回200 OK表示服务正常运行
- /readyz：检查数据库和Redis是否可用，返回200 OK表示服务可以处理请求
- /metrics：用于暴露Prometheus指标，返回服务的各种性能指标
这里的rest.MetricsHandler(reg)是一个HandlerFunc，它会调用Prometheus的Registry对象reg的Gather方法，收集所有注册的指标数据，并将它们以Prometheus的文本格式返回给客户端。

### 6. 创建数据库查询层
```go
// ---- /api/internal/api/main.go ----
queries := sqlcgen.New(pool)
```
这里的sqlcgen.New(pool)是一个工厂函数，它会根据数据库连接池pool创建一个新的查询层对象queries。这个对象封装了所有的SQL查询方法，方便在Handler中调用。比如（假设有一个查询方法GetUserByID）：
```go
user, err := queries.GetUserByID(ctx, id)
```

### 7. 创建业务Service
```go
// ---- /api/internal/api/main.go ----
catalogSvc := catalog.New(queries)
baseRankings := rankings.New(queries, versionResolverAdapter{queries: queries})
var rankingsSvc v1.RankingsService = baseRankings

// --- apps/api/internal/service/catalog/service.go ---

// 这是sqlc生成的Qerier接口的子集，定义了catalog service需要的数据库查询方法
// 和原始的Querier接口相比，减少了不必要的方法，降低了耦合度
type Querier interface {
	ListVersionsWithData(ctx context.Context) ([]string, error)
	ListRegionsWithData(ctx context.Context) ([]string, error)
}

type Service struct {
	q Querier
}

func New(q Querier) *Service {
	return &Service{q: q}
}

// --- apps/api/internal/service/rankings/service.go ---

// 这是sqlc生成的Qerier接口的子集，定义了rankings service需要的数据库查询方法
type Querier interface {
	ListOverallRankings(ctx context.Context, arg sqlcgen.ListOverallRankingsParams) ([]sqlcgen.ListOverallRankingsRow, error)
	ListRankingsByPosition(ctx context.Context, arg sqlcgen.ListRankingsByPositionParams) ([]sqlcgen.ListRankingsByPositionRow, error)
}

// 用于获取最新版本的接口，方便在Service中调用
type VersionResolver interface {
	GetLatestVersion(ctx context.Context) (string, error)
}

// Service is the rankings use case. Construct with New().
type Service struct {
	q        Querier
	versions VersionResolver
}

func New(q Querier, versions VersionResolver) *Service {
	return &Service{q: q, versions: versions}
}
```
这里的catalogSvc和baseRankings都是业务Service对象，它们封装了具体的业务逻辑，调用数据库查询层queries来获取数据，并进行处理和转换，最终返回给Handler。比如：


### 8. 包装rankings 缓存（可选）
```go
// ---- /api/internal/api/main.go ----
if redisClient != nil {
	const rankingsTTL = 5 * time.Minute
	rankingsSvc = rankings.NewCached(baseRankings, redisClient, rankingsTTL)
	logger.Info("rankings_cache_enabled", "ttl", rankingsTTL)
}

// --- apps/api/internal/service/rankings/cached.go ---
type CachedService struct {
	inner *Service
	cache cache.Cache
	ttl   time.Duration
}
```
没有Redis
```go
rankingsSvc = baseRankings
```
有Redis
```go
rankingsSvc = rankings.NewCached(baseRankings, redisClient, rankingsTTL)
```

### 9. 挂载REST API路由
```go
// ---- /api/internal/api/main.go ----
r.Mount("/api/v1", v1.Routes(catalogSvc, rankingsSvc))
```


### 10. 挂载 GraphQL
```go
gqlRoot := &resolver.Resolver{Catalog: catalogSvc, Rankings: rankingsSvc}
r.Handle("/graphql", gqlserver.NewHandler(gqlRoot))
if cfg.API.GraphQLPlayground {
	r.Handle("/graphql/playground", gqlserver.NewPlaygroundHandler("/graphql"))
	logger.Info("graphql_playground_enabled", "path", "/graphql/playground")
}
```

### 11. 挂载认证相关路由
```go
// ---- /api/internal/api/main.go ----
// OAuth + /auth endpoints land only when a jwt secret is configured.
if issuer != nil {
	providers := configuredProviders(cfg.OAuth, logger)
	userService := usersvc.New(queries, issuer, providers...)
	authCfg := restauth.Config{
		CookieDomain: cfg.Auth.CookieDomain,
		CookieSecure: cfg.Auth.CookieSecure,
	}
	r.Mount("/", restauth.Routes(userService, authCfg))
	names := make([]string, 0, len(providers))
	for _, p := range providers {
		names = append(names, p.Name())
	}
	logger.Info("oauth_providers_registered", "providers", names)
}
```
