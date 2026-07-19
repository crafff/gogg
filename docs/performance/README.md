# GOGG API 性能优化

本目录是 API 性能优化工作的总入口。阶段文档描述计划、统一步骤和验收标准；
实验文档记录一次具体变更的假设、数据和结论；原始机器数据保存在
`tmp/performance/`，不提交 Git。

## 工作原则

```text
定义目标 -> 建立基线 -> 定位瓶颈 -> 提出假设 -> 单点修改
        -> 重复测试 -> 对比结果 -> 正确性验证 -> 记录结论
```

- 不凭感觉修改 SQL、缓存或资源配置。
- 一次实验只验证一个主要变量。
- 修改前后使用相同数据集、负载和机器环境。
- 先进行一次不计入结果的预热，再正式运行至少三次并比较中位数。
- 延迟、吞吐量、错误率、CPU、内存和依赖成本必须一起判断。

完整规则见 [methodology.md](methodology.md)，本地命令见
[../performance-testing.md](../performance-testing.md)。

## 阶段状态

| 阶段 | 状态 | 文档 | 当前下一步 |
|---|---|---|---|
| 0 契约和 SLO | 已完成 | [阶段 0](stages/00-contract-and-slo/README.md) | 契约变更时重新验收 |
| 1 可重复基线 | 已完成 | [阶段 1](stages/01-repeatable-baseline/README.md) | 保持运行归档格式稳定 |
| 2 可观测性 | 第一版完成 | [阶段 2](stages/02-observability/README.md) | 增加业务缓存与连接池指标 |
| 3 PostgreSQL | 待执行 | [阶段 3](stages/03-postgresql/README.md) | 分析 rankings 执行计划 |
| 4 缓存 | 待执行 | [阶段 4](stages/04-cache/README.md) | 等待数据库基线完成 |
| 5 API 保护 | 待执行 | [阶段 5](stages/05-api-protection/README.md) | 盘点资源上限 |
| 6 GraphQL 成本 | 待执行 | [阶段 6](stages/06-graphql-cost/README.md) | 固定代表性 operation |
| 7 容量与故障 | 待执行 | [阶段 7](stages/07-capacity-and-failure/README.md) | 等待关键路径稳定 |
| 8 CI 与发布 | 待执行 | [阶段 8](stages/08-ci-and-release/README.md) | 定义回归阈值 |

## 当前事实

- rankings 冷聚合曾观测到约 13–17 秒。
- rankings 热缓存 p50 约 1.5–3.2 毫秒，p95 约 3.5–4.7 毫秒。
- 本地 Prometheus、Grafana、Redis Exporter、PostgreSQL Exporter 和
  `pg_stat_statements` 已接入。
- 阶段 1 已取得固定数据集上的三次 warm/cold 基线；warm 中位数为
  160.8 req/s、p95 18.5 ms、p99 32.9 ms，cold 单请求中位数为 8.16 s。
- Stage 1 补验已归档同窗口 Prometheus/日志/pg_stat_statements；固定 60 秒 load 的
  三次吞吐为 180.5、179.3、183.4 req/s，范围约 2.3%。

## 实验索引

| 实验 | 阶段 | 状态 | 结论 |
|---|---|---|---|
| [EXP-20260713-01](experiments/EXP-20260713-01.md) | 1/2 | 完成 | 本地性能观测链路可用 |
| [EXP-20260713-02](experiments/EXP-20260713-02.md) | 1 | 完成 | EOF 来自观测 API 写超时；冷聚合仍需优化 |
| [EXP-20260719-STAGE01-BASELINE](experiments/EXP-20260719-STAGE01-BASELINE.md) | 1 | 需要更多数据 | k6/SLO 通过；需补资源窗口并解释 warm 波动 |
| [EXP-20260719-STAGE01-RECHECK](experiments/EXP-20260719-STAGE01-RECHECK.md) | 1 | 接受 | 同窗口观测归档完成；解释波动并通过 Stage 1 验收 |

新增实验时复制 [实验模板](experiments/README.md)，并在此表追加一行。
