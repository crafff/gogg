# API 清单

本文件是阶段 0 的实施产物，用于固定性能测试覆盖的 API 表面、调用方和依赖。

## 清单

| 类型 | 接口 | 调用方 | 认证 | 主要依赖 | 主要风险 | 优先级 |
|---|---|---|---|---|---|---|
| REST | `GET /api/v1/rankings/champions` | Web | 待确认 | PostgreSQL、Redis | 聚合 SQL、缓存、超时 | P0 |
| REST | `GET /api/v1/versions` | Web | 待确认 | PostgreSQL、Redis | 重复查询、缓存 | P1 |
| REST | `GET /api/v1/regions` | Web | 待确认 | PostgreSQL、Redis | 重复查询、缓存 | P1 |
| GraphQL | `POST /graphql` | Web/开发者 | 待确认 | PostgreSQL、Redis | 查询复杂度、N+1、响应大小 | P1 |
| Auth | `/oauth/*`、`/auth/*` | Web | 按端点 | 外部 OAuth、PostgreSQL | 外部延迟、限流、安全 | P1 |
| Operations | `/healthz`、`/readyz`、`/metrics` | 平台 | 内部访问 | 服务依赖 | 探针语义、指标基数 | P1 |

## 待完成

- 从实际路由生成完整端点清单，避免只记录已知端点。
- 确认每个端点的调用方、认证和访问频率。
- 按用户影响、请求量、错误率和资源成本确认优先级。
- 为每个 P0/P1 端点链接代表性请求。
