# Stage 00 验收报告

- 状态：已完成
- 验收日期：2026-07-18
- 负责人：项目维护者

## 验收项

| 项目 | 状态 | 证据/说明 |
|---|---|---|
| 关键 API 清单完整 | 已完成 | [API 表面清单](01-api-surface.md)、[router wiring](../../../../apps/api/cmd/api/main.go) |
| 典型请求可直接执行 | 已完成 | [REST/GraphQL curl](02-request-contracts-and-workloads.md#53-可直接执行)、[k6 workloads](../../../../tests/performance/k6/api-baseline.js) |
| 最大合法请求已固定 | 已完成 | [保护边界](02-request-contracts-and-workloads.md#10-最大合法请求和保护边界)、[共享验证器](../../../../apps/api/internal/service/rankings/validation.go) |
| 非法请求结果稳定 | 已完成 | [REST tests](../../../../apps/api/internal/transport/rest/v1/v1_test.go)、[GraphQL tests](../../../../apps/api/internal/transport/graphql/server_test.go) |
| 稳定测试数据可恢复 | 已完成 | [固定数据集](03-fixed-dataset.md)、[移动盘管理脚本](../../../../tests/performance/perf-drive.sh) |
| REST/GraphQL 契约可比 | 已完成 | [矩阵契约](02-request-contracts-and-workloads.md#7-完整正确性矩阵)、[contract matrix 实现](../../../../tests/performance/k6/api-baseline.js) |
| 本地回归目标可计算 | 已完成 | [SLO 场景目标](04-slo-and-measurement.md#场景目标)、[指标注册表](04-slo-and-measurement.md#指标注册表) |
| 生产 SLO 边界明确 | 已完成 | [适用边界](04-slo-and-measurement.md#适用边界)、[性能测试解释原则](../../../performance-testing.md#interpretation) |

端到端契约矩阵已在移动硬盘固定数据集上执行通过：120 个 HTTP 请求、60 个业务组合、
零 mismatch。归档位置：
`tmp/performance/EXP-20260718-STAGE00/stage00-acceptance-fixed/contract_matrix-warm/run-01`。

## 固定基线契约

- 数据集：[`rankings-kr-16.13-current-v1`](03-fixed-dataset.md#正式数据集)
- game version：[`16.13`](02-request-contracts-and-workloads.md#3-标准参数空间)，性能基线禁止使用 `latest`
- 主路径：[GraphQL `ChampionRankings`](02-request-contracts-and-workloads.md#4-标准-graphql-请求)
- 对照路径：[REST `/api/v1/rankings/champions`](02-request-contracts-and-workloads.md#5-标准-rest-请求)
- 默认负载：[warm/cold 口径](04-slo-and-measurement.md#指标注册表)
- 结果归档字段：[运行归档脚本](../../../../tests/performance/run-api-performance.sh)

## 阶段结论

阶段 0 通过。API 表面、合法参数、代表性请求、可恢复数据集和本地 SLO 已形成可执行
契约，可以进入正式可重复 baseline。后续若修改 API 参数、数据集、固定版本、缓存
语义或 SLO，必须更新本目录并建立新基线，不能与旧结果直接比较。
