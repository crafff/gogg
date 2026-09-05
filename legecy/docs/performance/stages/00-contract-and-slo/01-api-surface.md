# API 表面与路由清单

本文件固定阶段 0 覆盖的公开 API 表面。清单已与
`apps/api/cmd/api/main.go`、`transport/rest/v1` 和 Auth router 核对。

## 路由清单

| 类型 | Method | 路径 | 调用方 | 认证 | 主要依赖 | 优先级 | 代表性请求 |
|---|---|---|---|---|---|---|---|
| REST | GET | `/api/v1/rankings/champions` | Web、兼容客户端 | 可选 Bearer，不要求登录 | PostgreSQL、Redis | P0 | `RKG-REST-*` |
| GraphQL | POST | `/graphql`（`ChampionRankings`） | Web | 可选 Bearer，不要求登录 | PostgreSQL、Redis | P0 | `RKG-GQL-*` |
| REST | GET | `/api/v1/versions` | Web | 不要求 | PostgreSQL | P1 | [Catalog versions](02-request-contracts-and-workloads.md#9-catalog-辅助请求) |
| REST | GET | `/api/v1/regions` | Web | 不要求 | PostgreSQL | P1 | [Catalog regions](02-request-contracts-and-workloads.md#9-catalog-辅助请求) |
| GraphQL | POST | `/graphql`（`Versions`） | Web/开发者 | 不要求 | PostgreSQL | P1 | [GraphQL Versions](02-request-contracts-and-workloads.md#9-catalog-辅助请求) |
| GraphQL | POST | `/graphql`（`Regions`） | Web/开发者 | 不要求 | PostgreSQL | P1 | [GraphQL Regions](02-request-contracts-and-workloads.md#9-catalog-辅助请求) |
| Operations | GET | `/healthz` | 平台探针 | 不要求；部署层限制访问 | 进程 | P1 | liveness |
| Operations | GET | `/readyz` | 平台探针 | 不要求；部署层限制访问 | PostgreSQL、可选 Redis | P1 | readiness |
| Operations | GET | `/metrics` | Prometheus | 不要求；部署层限制访问 | 指标 registry | P1 | scrape |
| OAuth | GET | `/oauth/start/google` | Web | 不要求 | Google OAuth、PostgreSQL | P1 | 外部集成，不纳入本地延迟门禁 |
| OAuth | GET | `/oauth/callback/google` | Google/Web | state + browser-binding cookie + PKCE | Google OAuth、PostgreSQL | P1 | 外部集成，不纳入本地延迟门禁 |
| Auth | POST | `/auth/logout` | Web | session cookie + CSRF header | PostgreSQL | P1 | Auth 专项测试 |
| GraphQL | POST | `/graphql`（`Me`/`AuthProviders`） | Web | `Me` 可选 session cookie | PostgreSQL | P1 | Auth 专项测试 |

`/graphql` handler 还支持 GET 和 OPTIONS transport；Web 的正式业务路径固定为 POST，
OPTIONS 由 CORS 浏览器预检覆盖，GET 不作为排行榜性能基线。开发配置开启时还存在
`GET /graphql/playground`，它不是生产 API 或性能场景。

OAuth/Auth 路由独立于可选的 JWT issuer 挂载；当前只注册配置完整的 Google provider。
它们依赖外部网络和用户会话，不与数据库 rankings 基线混测。

## 调用频率和风险

| 路径组 | 预期频率 | 主要风险 |
|---|---|---|
| ChampionRankings | 页面加载及筛选时调用，最高业务成本 | 聚合 SQL、缓存击穿、超时、响应大小 |
| versions/regions | 页面初始化，低成本高复用 | 重复查询、空结果契约 |
| health/ready/metrics | 平台周期调用 | 探针语义、指标基数 |
| OAuth/Auth | 登录、续期、退出时调用 | 外部延迟、token 安全、限流 |

阶段 0 的本地性能门禁覆盖 rankings、catalog 和常用 GraphQL。Operations 只验证可用性；
OAuth/Auth 的外部延迟不纳入本地 SLO，后续以 mock provider 或独立集成环境测试。

代表性请求、workload ID 和参数矩阵见
[请求契约与代表性 workload](02-request-contracts-and-workloads.md)。
