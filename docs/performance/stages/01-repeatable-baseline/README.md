# 阶段 1：建立可重复基线

本目录集中管理基线执行计划、数据快照说明和阶段验收材料。

## 目标

在相同提交、数据和环境下得到可解释、不可覆盖且可追溯的基线结果。

## 前置条件

- 阶段 0 已固定场景参数和数据集。
- observed API、PostgreSQL、Redis、Prometheus 和 Grafana 正常。
- 正式结果的 Git 工作区应为 clean；例外必须在实验文档说明。

## 步骤

1. 设置一个实验 ID，例如 `EXP-20260714-01`。
2. 执行一次 `PERF_VARIANT=warmup`，不纳入统计。
3. 分别执行三次 baseline warm 和三次 baseline cold。
4. 修改后以相同条件执行三次 candidate warm/cold。
5. 核对每次运行的 commit、image、dataset ID、负载、退出码和 Git 差异快照。
6. 在实验文档填写中位数及所有参与统计的 run ID。

命令和目录结构见 [../../../performance-testing.md](../../../performance-testing.md)。统计规则见
[../../methodology.md](../../methodology.md)。

## 必须采集

- RPS、p50、p95、p99、最大延迟、失败率和响应大小。
- API CPU、内存、goroutine 和在途请求。
- Redis hit/miss、内存；PostgreSQL 连接、查询时间和磁盘读取。

## 验收标准

- 运行结果不会覆盖，且可以从结论追溯到原始 run。
- dirty 运行的相关差异已保存并解释；无法还原的结果不作为正式基线。
- 三次正式运行的差异能够解释；波动过大时不进入下一阶段。
- k6、Prometheus 和服务日志的时间窗口能够对应。
- 能区分客户端、API、Redis 和 PostgreSQL 故障。

## 实施文档

- [基线执行计划](baseline-plan.md)
- [数据快照记录](dataset-snapshot.md)
- [阶段验收报告](acceptance-report.md)
