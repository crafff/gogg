# 基线执行计划

## 固定条件

- 实验 ID：`EXP-20260719-STAGE01-BASELINE`。
- 数据集：`rankings-kr-16.13-current-v1`，dump SHA-256 见
  [数据快照记录](dataset-snapshot.md)。
- 场景：`rankings_graphql`（`RKG-GQL-KR-MP-ALL`）。
- 典型参数：KR、`16.13`、`master_plus`、overall、`minGames=20`、
  `positionThreshold=5`，返回全部符合条件的英雄。正式基线禁止使用 `latest`。
- warm 负载：20 VU，1 分钟。
- cold：每次 FLUSHDB 后单请求。

## 执行顺序

```bash
export PERF_EXPERIMENT=EXP-YYYYMMDD-NN

make perf-warm PERF_VARIANT=warmup

make perf-warm PERF_VARIANT=baseline
make perf-warm PERF_VARIANT=baseline
make perf-warm PERF_VARIANT=baseline

make perf-cold PERF_VARIANT=baseline
make perf-cold PERF_VARIANT=baseline
make perf-cold PERF_VARIANT=baseline
```

candidate 必须在相同机器、数据集和负载下重复 warm/cold 各三次。

2026-07-19 的首次正式 baseline 已按上述顺序执行，原始目录为
`tmp/performance/EXP-20260719-STAGE01-BASELINE/`，结论见
[实验记录](../../experiments/EXP-20260719-STAGE01-BASELINE.md)。

## 运行前检查

- observed API、PostgreSQL、Redis、Prometheus 和 Grafana healthy。
- 机器没有其他明显 CPU、内存或磁盘负载。
- Git dirty 差异已确认并可以还原。
- `metadata.env` 中 image、dataset ID 和参数与计划一致。

## 结果处理

- warm 对三次正式运行的 RPS、p50、p95、p99、错误率取中位数。
- cold 对三次独立单请求耗时取中位数。
- 不使用 warmup，不删除异常值；异常运行应说明原因并整体重跑。
