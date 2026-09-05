# 阶段 1 验收报告

- 状态：已完成
- 验收日期：2026-07-19
- 负责人：项目维护者

## 验收项

| 项目 | 状态 | 证据/说明 |
|---|---|---|
| 结果按 experiment/variant/run 归档 | 已完成 | 运行脚本自动创建目录 |
| 运行元数据和 Git 差异可追溯 | 已完成 | metadata、status、diff 文件 |
| stable dataset 已固定 | 已完成 | `rankings-kr-16.13-current-v1`，六次运行的 dataset ID 均为 `509927be4b03` |
| warm baseline 正式运行三次 | 已完成 | `EXP-20260719-STAGE01-BASELINE/baseline/rankings_graphql-warm/run-01..03` |
| cold baseline 独立运行三次 | 已完成 | `EXP-20260719-STAGE01-BASELINE/baseline/rankings_graphql-cold/run-01..03` |
| 运行波动可解释 | 已完成 | 补验三次 load 请求数为 10,830/10,757/11,006，固定 60 秒折算 180.5/179.3/183.4 req/s，范围约 2.3%；run-02 名义 rate 偏低来自 setup 的 6.91 秒过期缓存重建被计入分母 |
| k6、Prometheus、日志时间可对应 | 已完成 | `EXP-20260719-STAGE01-RECHECK` 每个 run 均保存起止时间、Prometheus range JSON、API 日志、容器资源快照和本次运行的 pg_stat_statements，`observation_exit_code=0` |

## 阶段结论

固定数据集上的重复基线和观测补验均已完成，k6 正确性及本地 SLO 全部通过。补验
显示 warm load 本身稳定；名义 RPS 的异常来自缓存 TTL 到期后 setup 执行一次冷聚合，
而非一分钟 load 的容量下降。API、Redis、PostgreSQL 和日志证据现已与每个 run 的
时间窗口一起归档。Stage 1 验收通过，可以进入 Stage 3 的 rankings SQL 分析。

详细结果和归因见
[EXP-20260719-STAGE01-RECHECK](../../experiments/EXP-20260719-STAGE01-RECHECK.md)。
