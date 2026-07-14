# GOGG API 优化日志与实施手册

## 1. 文档目的

本文档用于持续记录 GOGG API 的性能、稳定性和可维护性优化过程。
所有优化都必须遵循以下闭环：

```text
定义目标 -> 建立基线 -> 定位瓶颈 -> 提出假设 -> 单点修改
        -> 重复测试 -> 对比结果 -> 灰度验证 -> 记录结论
```

禁止只凭感觉修改 SQL、增加缓存或扩大资源。每项修改必须有优化前数据、
优化后数据、正确性验证和回滚方案。

## 2. 优化范围

当前 API 表面包括：

| 类型 | 接口 | 主要风险 |
|---|---|---|
| GraphQL | `POST /graphql` | 查询复杂度、N+1、响应大小 |
| REST | `GET /api/v1/rankings/champions` | 聚合 SQL、缓存、超时 |
| REST | `GET /api/v1/versions` | 重复查询、缓存 |
| REST | `GET /api/v1/regions` | 重复查询、缓存 |
| Auth | `/oauth/*`、`/auth/*` | 限流、安全、外部依赖 |
| Operations | `/healthz`、`/readyz`、`/metrics` | 探针、指标基数、访问控制 |

优化优先级按用户影响、请求量、错误率和资源成本决定，不按代码修改难度决定。

## 3. 阶段总览

| 阶段 | 目标 | 核心产物 | 状态 |
|---|---|---|---|
| 0 | 明确契约和目标 | API 清单、SLO、测试数据说明 | 进行中 |
| 1 | 建立可重复基线 | k6 结果、Grafana Dashboard | 进行中：基础设施和实验归档已完成，待固定数据集 |
| 2 | 补齐可观测性 | HTTP、Redis、PostgreSQL 指标 | 已完成第一版 |
| 3 | 定位数据库瓶颈 | 执行计划、索引和 SQL 假设 | 待执行 |
| 4 | 优化缓存 | 命中率、失效策略、降级策略 | 待执行 |
| 5 | 加固 API | 参数校验、分页、限流、超时 | 待执行 |
| 6 | 控制 GraphQL 成本 | 深度、复杂度、N+1 报告 | 待执行 |
| 7 | 容量与故障测试 | 容量模型、故障演练报告 | 待执行 |
| 8 | CI 和发布门禁 | 性能回归、灰度、回滚标准 | 待执行 |

## 4. 阶段 0：定义 API 契约和 SLO

### 4.1 要做什么

1. 列出所有公开接口、调用方、认证要求和数据依赖。
2. 为每个接口定义典型请求和最坏合法请求。
3. 固定测试数据快照，记录表行数和关键字段分布。
4. 定义延迟、错误率、可用性和响应大小目标。
5. 明确哪些接口允许读取稍旧的数据。

### 4.2 第一版目标

| 接口场景 | p95 | p99 | 5xx | 备注 |
|---|---:|---:|---:|---|
| catalog | `<100ms` | `<250ms` | `<0.1%` | versions、regions |
| rankings 热缓存 | `<100ms` | `<250ms` | `<0.1%` | 当前本地结果明显低于目标 |
| rankings 冷缓存 | 先测量 | 先测量 | `<0.1%` | 优化后再确定 SLO |
| GraphQL 常用查询 | `<300ms` | `<750ms` | `<0.1%` | 必须限制查询成本 |

### 4.3 工具与产物

- GraphQL SDL、REST 路由和 OpenAPI 文档。
- PostgreSQL 表规模查询。
- `docs/api-optimization-log.md` 中的基线表。

### 4.4 完成标准

- 每个关键 API 都有可直接执行的代表性请求。
- SLO 不使用“尽量快”等不可验证描述。
- 测试数据来源和规模可被另一个工程师复现。

## 5. 阶段 1：建立性能基线

### 5.1 要做什么

1. 启动容器化观测 API、PostgreSQL、Redis、Prometheus 和 Grafana。
2. 分别执行热缓存、冷缓存、正常参数和最大合法参数测试。
3. 先执行一次不计入统计的环境预热，再执行至少三次正式测量。
4. 保存 k6 JSON，记录代码提交、镜像、数据规模和机器配置。
5. 使用中位数作为版本对比值，不使用最好的一次结果。

### 5.2 本地命令

```bash
make observability
make perf-warm
make perf-cold

make perf-warm PERF_SCENARIO=graphql PERF_VUS=50 PERF_DURATION=2m
```

结果按 `<experiment>/<variant>/<scenario>-<mode>/run-NN` 保存于
`tmp/performance/`，不会覆盖历史运行。该目录不提交 Git；正式结论填写到本文档
的实验记录，并引用实验 ID 和参与统计的 run ID。

### 5.3 必须收集的指标

- RPS 和并发数。
- p50、p95、p99、最大延迟。
- 4xx、5xx 和客户端失败率。
- 响应体大小。
- API CPU、内存、goroutine 和在途请求。
- Redis hit/miss 和内存。
- PostgreSQL 连接、查询时间、缓存读取和磁盘读取。

### 5.4 完成标准

- 相同提交和数据下，连续测试结果差异可解释。
- k6、Prometheus 和服务日志中的请求时间能够相互对应。
- 测试失败时能够区分客户端、API、Redis 和 PostgreSQL 问题。

## 6. 阶段 2：补齐可观测性

### 6.1 已完成内容

- Prometheus 采集 HTTP 请求数、延迟、在途请求和响应大小。
- Redis Exporter 提供 keyspace hit/miss、内存和操作指标。
- PostgreSQL Exporter 提供连接和数据库活动指标。
- `pg_stat_statements` 记录规范化 SQL 的调用次数和耗时。
- Grafana 自动加载 `Gogg API Overview` Dashboard。
- Prometheus 配置 API 5xx、高 p95、依赖下线和 Redis 低命中率告警。

### 6.2 后续增强

1. 为应用缓存增加业务级 `hit/miss/error` 指标。
2. 增加 pgx 连接池 acquire 次数和等待时间。
3. 接入 OpenTelemetry Trace，串联 HTTP、service、Redis 和 PostgreSQL。
4. GraphQL 指标按受控的 operation name 聚合。
5. 检查所有 Prometheus 标签，禁止原始 URL、用户 ID 和完整 GraphQL query。

### 6.3 完成标准

- 一个慢请求可以定位到具体依赖和 SQL。
- Dashboard 中不存在无限增长的高基数标签。
- 告警可以在本地通过受控故障触发并恢复。

## 7. 阶段 3：优化 PostgreSQL 查询

排名查询是当前第一优先级。冷查询已经观测到约 13 至 17 秒。

### 7.1 要做什么

1. 使用真实参数提取 `ListOverallRankings` 和 `ListRankingsByPosition` SQL。
2. 执行：

```sql
EXPLAIN (ANALYZE, BUFFERS, WAL, SETTINGS)
-- ranking query
```

3. 记录执行计划中的：
   - 顺序扫描和索引扫描。
   - 估算行数与实际行数偏差。
   - sort 是否落盘。
   - shared hit/read 和临时块。
   - 每个聚合、连接和 CTE 的耗时。
4. 一次只验证一个假设，例如增加一个索引或重写一个 CTE。
5. 同时测量 Worker 写入成本，防止读优化造成写入显著退化。

### 7.2 优化顺序

1. 更新统计信息并确认数据分布。
2. 检查现有索引是否被使用。
3. 验证复合或部分索引。
4. 减少重复扫描和过早产生的大结果集。
5. 评估预聚合表或物化视图。
6. 若实时聚合仍昂贵，将聚合移动到 Worker 数据发布阶段。

### 7.3 工具

- `EXPLAIN (ANALYZE, BUFFERS, WAL, SETTINGS)`。
- `pg_stat_statements`：`make db-slow-queries`。
- `pg_stat_user_indexes`、`pg_stat_user_tables`。
- `auto_explain`，仅在受控测试环境使用。

### 7.4 完成标准

- 冷查询耗时达到阶段 0 确定的目标。
- 执行计划在生产级数据量下稳定。
- 新索引的空间和 Worker 写入代价已量化。
- SQL 正确性和 REST/GraphQL 契约测试通过。

## 8. 阶段 4：优化缓存

### 8.1 要做什么

1. 增加应用级 cache hit、miss、error 和 loader duration 指标。
2. 验证缓存 key 包含所有影响结果的过滤条件和 schema 版本。
3. 将 TTL 配置化，并添加随机抖动避免同时过期。
4. Worker 发布新数据后主动失效相关排名缓存。
5. 保留 `singleflight`，验证并发 miss 只产生一次数据库加载。
6. Redis 故障时继续读 PostgreSQL，同时限制回源并发。
7. 评估 stale-while-revalidate，明确允许的数据陈旧时间。

### 8.2 必测场景

- 热缓存稳定负载。
- 单个 key 冷启动。
- 多个 key 同时过期。
- Redis 延迟升高和完全不可用。
- loader 失败，不写入错误结果。
- 大响应值对 Redis 内存和网络的影响。

### 8.3 完成标准

- 命中率达到目标且可解释。
- Redis 故障不会直接导致全部 API 失败。
- 缓存失效后不会形成数据库请求风暴。
- 新数据在约定时间内对用户可见。

## 9. 阶段 5：API 正确性与资源保护

### 9.1 要做什么

1. 非法数字、position、tier 和 version 返回明确的 `400`。
2. 禁止生产请求使用无限 `limit=-1`，设置响应条数上限。
3. 大数据列表采用 cursor pagination。
4. 设置请求体、Header、读取、写入和每请求处理超时。
5. 对 OAuth、刷新 token 和高成本查询实施差异化限流。
6. 增加最大并发保护，避免连接池被单一路由耗尽。
7. 统一 REST 错误结构和内部错误日志。

### 9.2 完成标准

- fuzz 和边界测试不能触发无限结果或 panic。
- 超时能取消下游 PostgreSQL/Redis 操作。
- 限流响应包含稳定状态码和必要的重试信息。
- 所有资源上限都有配置、默认值和测试。

## 10. 阶段 6：GraphQL 成本控制

### 10.1 要做什么

1. 设置请求体、查询深度、字段数量和复杂度上限。
2. 检查 resolver 的数据库调用次数，识别 N+1。
3. 必要时引入 DataLoader，并验证批处理上限。
4. 所有列表字段要求分页并限制最大 page size。
5. 生产环境关闭或限制 Playground 和 introspection。
6. 使用 operation name 或 persisted query 建立稳定指标维度。

### 10.2 完成标准

- 恶意深层或高 fan-out 查询在执行前被拒绝。
- 常用查询的数据库访问次数固定且可预测。
- GraphQL errors 与 HTTP transport errors 分别统计。

## 11. 阶段 7：容量测试与故障演练

### 11.1 容量测试

逐步提高负载，而不是直接使用最大并发：

```text
5 VU -> 20 VU -> 50 VU -> 100 VU -> 找到拐点
```

每一级观察 p95、错误率、CPU、连接池等待和数据库利用率。停止条件包括：

- 5xx 超过 1%。
- p95 持续超过 SLO 两倍。
- 数据库连接池持续饱和。
- 容器 OOM 或系统发生严重 swap。

### 11.2 故障演练

- 停止 Redis。
- 增加 PostgreSQL 延迟或降低连接上限。
- 在负载中重启 API。
- 模拟慢客户端和请求取消。
- 验证 readiness、告警、降级和恢复时间。

### 11.3 完成标准

- 得到单实例安全 RPS 和资源配置。
- 明确扩容触发点、降级行为和恢复时间。
- 所有故障演练都有停止条件，不对生产直接执行破坏性操作。

## 12. 阶段 8：CI、发布和持续维护

### 12.1 CI 分层

- 每次提交：单元测试、race、lint、契约测试和微基准。
- 每日或手动：本地规模的 k6 性能回归。
- 发布前：独立性能环境的完整负载和故障测试。
- 上线后：canary 新旧版本指标对比。

### 12.2 性能门禁

门禁应同时约束：

- 正确率和错误率。
- p95/p99。
- 吞吐量。
- CPU 和内存。
- PostgreSQL 查询量和时间。
- Redis 命中率。

只比较延迟可能通过“消耗更多 CPU”制造虚假优化。

### 12.3 回滚条件

- 5xx 或 GraphQL error rate 超过阈值。
- p95 相对基线显著退化并持续多个窗口。
- 数据库负载、连接池等待或缓存错误异常上升。
- 数据正确性或 API 契约发生回归。

## 13. 实验记录模板

每次优化复制以下模板，不覆盖历史记录：

```markdown
### EXP-YYYYMMDD-NN：实验标题

- 日期：
- 负责人：
- Git commit：
- 工作区是否 dirty：
- 环境和资源：
- 数据集 ID/表规模：
- API/参数：
- baseline run ID：
- candidate run ID：
- 假设：
- 修改内容：
- 回滚方式：

| 指标 | 修改前 | 修改后 | 变化 |
|---|---:|---:|---:|
| RPS | | | |
| p50 | | | |
| p95 | | | |
| p99 | | | |
| 5xx | | | |
| CPU | | | |
| 内存 | | | |
| DB mean/max | | | |
| Redis hit rate | | | |

- 正确性验证：
- 结论：接受 / 拒绝 / 需要更多数据
- 遗留风险：
- 下一步：
```

表中的“修改前/修改后”填写至少三次正式运行的中位数；单次运行的原始指标保留在
`tmp/performance/`，不要手工挑选最好的一次。冷缓存测试每次只测一个请求，表中
填写多次独立冷启动耗时的中位数，而不是一次压测内部的 p95。

## 14. 已完成实验日志

### EXP-20260713-01：建立本地性能观测链路

- 环境：Docker Compose 本地开发环境。
- 内容：增加 Prometheus、Grafana、Redis Exporter、PostgreSQL Exporter、
  `pg_stat_statements`、响应大小指标和 k6 场景。
- 结果：API、Redis、PostgreSQL targets 正常；Grafana Dashboard 自动加载；
  k6 JSON 能写入 `tmp/performance/`。
- 验证：API Go 测试、Compose 配置、Dashboard JSON 和 k6 冒烟测试通过。

### EXP-20260713-02：修复 rankings 预热 EOF

- 现象：`make perf-warm` 在 setup 阶段约 17 秒后收到 EOF。
- 假设：服务端崩溃、OOM、数据库异常或 HTTP 超时。
- 诊断：容器持续运行、无 OOM、PostgreSQL 无遗留活动查询；API 日志显示请求
  实际完成并返回 `200`，耗时 `17.100s`。
- 根因：观测 API 的 `WriteTimeout=15s`，冷查询完成时连接已经被关闭。
- 修改：仅将容器化观测 API 的 write timeout 调整为 60 秒；生产默认值不变。
- 额外修改：k6 使用 `phase=warmup|load` 区分预热和正式负载，SLA 只检查 load。

验证结果：

| 场景 | 结果 |
|---|---:|
| Redis 空缓存预热 | 约 13.5 秒，HTTP 200 |
| 热缓存 p50 | 约 1.5 至 3.2 毫秒 |
| 热缓存 p95 | 约 3.5 至 4.7 毫秒 |
| 正式负载错误率 | 0% |
| 响应大小 | 约 17,468 bytes |

结论：EOF 已解决，但 13 至 17 秒的冷聚合查询不可接受。下一项实验是分析
`ListOverallRankings` 的 PostgreSQL 执行计划，而不是继续放宽生产超时。

## 15. 下一步执行清单

1. 固定 rankings 测试参数和数据库表规模。
2. 保存三次热缓存与三次冷缓存基线。
3. 对 `ListOverallRankings` 执行 `EXPLAIN (ANALYZE, BUFFERS, WAL, SETTINGS)`。
4. 把最昂贵执行节点、行数偏差和磁盘读取写入新实验记录。
5. 一次验证一个索引或 SQL 重写方案。
6. 每个候选方案同时跑正确性测试、冷查询和热缓存测试。

## 16. 相关文件

- `tests/performance/k6/api-baseline.js`：k6 压测场景。
- `deploy/observability/prometheus/`：抓取和告警规则。
- `deploy/observability/grafana/`：数据源和 Dashboard provisioning。
- `deploy/observability/postgres/slow-queries.sql`：累计慢 SQL 查询。
- `docs/performance-testing.md`：本地工具使用说明。
- `packages/sqlc/queries/rankings.sql`：当前排名聚合 SQL。
