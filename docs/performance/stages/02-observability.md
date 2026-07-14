# 阶段 2：补齐可观测性

## 目标

让一个慢请求能够被定位到 API、缓存、连接池或具体 SQL，同时控制指标基数。

## 已完成

- HTTP 请求数、延迟、在途请求和响应大小指标。
- Redis 和 PostgreSQL Exporter。
- `pg_stat_statements`、Grafana API Dashboard 和第一版告警。

## 后续步骤

1. 增加应用缓存 `hit/miss/error` 与 loader duration。
2. 增加 pgx 连接池 acquire、等待时间和饱和度。
3. 接入 OpenTelemetry Trace，串联 HTTP、service、Redis 和 PostgreSQL。
4. GraphQL 指标仅按受控 operation name 聚合。
5. 审计标签，禁止原始 URL、用户 ID 和完整 GraphQL query。

## 验收标准

- 慢请求能够归因到具体依赖和 SQL。
- Dashboard 不存在无限增长的高基数标签。
- 告警能通过受控故障触发，并在恢复后自动解除。
- 观测本身没有造成明显延迟或资源回归。
