# SLO、容量哨兵与测量口径

## 适用边界

以下目标是固定机器和固定数据集上的本地回归门禁，不是生产 SLO。正式运行使用
[固定数据集](03-fixed-dataset.md)、[标准 workload](02-request-contracts-and-workloads.md)
和 [k6 执行脚本](../../../../tests/performance/k6/api-baseline.js)。生产容量和 SLO 需要
结合真实流量、部署资源、跨区网络与业务影响另行批准，方法边界见
[性能测试方法](../../methodology.md#结论规则)。

## 场景目标

表中每个数值都链接到其可执行测量或契约来源。

| 场景 | p95 | p99 | 失败率 | 最低吞吐 | p99 响应大小 |
|---|---:|---:|---:|---:|---:|
| versions、regions | [`<100ms`](../../../../tests/performance/k6/api-baseline.js) | [`<250ms`](../../../../tests/performance/k6/api-baseline.js) | [`<0.1%`](../../../../tests/performance/k6/api-baseline.js) | [`>50 req/s`](../../../../tests/performance/k6/api-baseline.js) | [`<32 KiB`](02-request-contracts-and-workloads.md#9-catalog-辅助请求) |
| rankings 热缓存 | [`<100ms`](../../../../tests/performance/k6/api-baseline.js) | [`<250ms`](../../../../tests/performance/k6/api-baseline.js) | [`<0.1%`](../../../../tests/performance/k6/api-baseline.js) | [`>20 req/s`](../../../../tests/performance/k6/api-baseline.js) | [`<256 KiB`](02-request-contracts-and-workloads.md#84-完整结果与安全上限) |
| rankings 冷缓存 | [`<20s`](../../../../tests/performance/k6/api-baseline.js) | [`<25s`](../../../../tests/performance/k6/api-baseline.js) | [`<0.1%`](../../../../tests/performance/k6/api-baseline.js) | [单请求，不设持续吞吐](../../../performance-testing.md#run-a-baseline) | [`<256 KiB`](02-request-contracts-and-workloads.md#84-完整结果与安全上限) |
| GraphQL Versions | [`<300ms`](../../../../tests/performance/k6/api-baseline.js) | [`<750ms`](../../../../tests/performance/k6/api-baseline.js) | [`<0.1%`](../../../../tests/performance/k6/api-baseline.js) | [`>20 req/s`](../../../../tests/performance/k6/api-baseline.js) | [`<32 KiB`](02-request-contracts-and-workloads.md#9-catalog-辅助请求) |

k6 对 rankings 使用统一的 `<256 KiB` 自动门禁；catalog 的 `<32 KiB` 是更严格的
契约审查值，通过归档 summary 和响应大小指标检查。指标采集实现见
[HTTP metrics middleware](../../../../apps/api/internal/transport/middleware/metrics.go)。

## 指标注册表

| 指标 | 定义 | 测量与证据 |
|---|---|---|
| p95 / p99 延迟 | `phase=load` 请求的客户端耗时分位数 | [k6 thresholds](../../../../tests/performance/k6/api-baseline.js)、[运行归档脚本](../../../../tests/performance/run-api-performance.sh) |
| 失败率 | HTTP 失败率 `<0.1%`，checks 成功率 `>99.9%` | [k6 thresholds](../../../../tests/performance/k6/api-baseline.js)、[响应断言](02-request-contracts-and-workloads.md#8-每个请求的验收断言) |
| 吞吐量 | warm 阶段 `http_reqs{phase:load}` 每秒请求数 | [k6 workload options](../../../../tests/performance/k6/api-baseline.js)、[warm 运行方法](../../../performance-testing.md#run-a-baseline) |
| 响应大小 | k6 `gogg_response_bytes` 与 API response-size histogram | [k6 Trend](../../../../tests/performance/k6/api-baseline.js)、[metrics middleware](../../../../apps/api/internal/transport/middleware/metrics.go) |
| 数据陈旧时间 | rankings 最多 5 分钟；catalog 下一请求读取数据库提交 | [rankings cache TTL](../../../../apps/api/cmd/api/main.go)、[固定版本规则](02-request-contracts-and-workloads.md#3-标准参数空间) |
| REST/GraphQL 对等性 | 60 业务组合、120 transport 请求，mismatch 必须为 0 | [contract matrix](../../../../tests/performance/k6/api-baseline.js)、[矩阵契约](02-request-contracts-and-workloads.md#7-完整正确性矩阵) |
| 数据集一致性 | stable ID、dump SHA-256、KR 16.13 精确行数必须一致 | [数据集定义](03-fixed-dataset.md)、[运行前校验](../../../../tests/performance/run-api-performance.sh) |
| 缓存模式 | warm 先预热；cold 每次清 Redis 后执行一个请求 | [runner 实现](../../../../tests/performance/run-api-performance.sh)、[运行方法](../../../performance-testing.md#run-a-baseline) |
| 负载口径 | warm 20 VUs、1 分钟；每种至少三次并比较中位数 | [workload 契约](02-request-contracts-and-workloads.md#6-核心性能-workload)、[方法文档](../../methodology.md) |
| 本地/生产边界 | 本地结果只用于回归和容量哨兵，不直接形成生产承诺 | [性能测试解释原则](../../../performance-testing.md#interpretation)、[阶段验收](05-stage-acceptance.md) |

## 可比性规则

任一运行若数据集、版本、workload ID、cache mode、VUs、duration、机器或 API image
不同，不得直接比较。归档字段与判定方法见
[运行归档脚本](../../../../tests/performance/run-api-performance.sh)和
[性能测试方法](../../methodology.md)。
