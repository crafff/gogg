# EXP-20260719-STAGE01-BASELINE：固定数据集首次重复基线

- 所属阶段：1（可重复基线）
- 状态：需要更多数据
- 日期：2026-07-19
- 负责人：项目维护者
- 假设：固定数据集、镜像、负载和机器后，rankings GraphQL 的 warm/cold 结果可重复。
- 成功条件：三次正式 warm/cold 全部通过契约与本地 SLO，运行间波动可由同窗口资源数据解释。
- 停止条件：数据集、镜像或负载不一致；出现请求/正确性失败；波动无法解释。
- 回滚方式：本实验未修改运行时代码，无需回滚。

## 环境

- commit：`e456d75d10b183d1d4a28e5fdcd56ed33455ab9f`
- API image：`sha256:ad41a7b9457195e28dc73e586cf2c1644809d8653ec37d400e204fe08214cc8b`
- dirty：`true`；六次正式运行的 status 和 diff 校验和完全一致。差异为 worker/Riot API
  错误处理、crawler 文档、Stage 00 文档和本地加密配置；未改动 rankings API 镜像。
  正式运行全部使用同一不可变 image digest，因此这些差异不改变本次被测 API 代码。
- 数据集：`rankings-kr-16.13-current-v1`，dataset ID `509927be4b03`，dump SHA-256
  `a9402acd48c1a5229581dffb41fc72dffea54bc355bf58638741925dd54e9a43`。
- 场景：`rankings_graphql`，KR / 16.13 / master_plus / overall / minGames 20 / positionThreshold 5。
- 负载：warm 20 VU、1 分钟；cold 每次 FLUSHDB 后单请求。
- 主机：`RuitaoZhou`，Linux 5.15.167.4-microsoft-standard-WSL2 x86_64，Docker 29.1.3。

## 执行和原始运行

- 不计入统计的预热：`warmup/rankings_graphql-warm/run-01`
- warm：`baseline/rankings_graphql-warm/run-01`、`run-02`、`run-03`
- cold：`baseline/rankings_graphql-cold/run-01`、`run-02`、`run-03`
- 原始根目录：`tmp/performance/EXP-20260719-STAGE01-BASELINE/`（不提交 Git）

六次正式运行的 commit、image、dataset ID、snapshot SHA-256、主机和负载一致，
`exit_code=0`。每次响应大小均为 30,068 bytes。

## 结果

### Warm

| run | RPS | p50 | p95 | p99 | max | 失败率 | checks |
|---|---:|---:|---:|---:|---:|---:|---:|
| run-01 | 157.98 | 5.79 ms | 24.03 ms | 57.24 ms | 171.75 ms | 0% | 100% |
| run-02 | 160.78 | 4.44 ms | 18.47 ms | 32.88 ms | 87.50 ms | 0% | 100% |
| run-03 | 186.77 | 5.49 ms | 15.45 ms | 23.25 ms | 39.29 ms | 0% | 100% |
| **中位数** | **160.78** | **5.49 ms** | **18.47 ms** | **32.88 ms** | **87.50 ms** | **0%** | **100%** |

warm 中位数满足 rankings 热缓存的 p95 `<100 ms`、p99 `<250 ms`、失败率
`<0.1%`、吞吐 `>20 req/s` 和响应 `<256 KiB` 本地目标。但 RPS 最大/最小相差
18.2%，p99 最大值是最小值的 2.46 倍；绝对延迟虽低，仍需资源窗口解释。

### Cold

| run | 单请求耗时 | 失败率 | checks |
|---|---:|---:|---:|
| run-01 | 8.158 s | 0% | 100% |
| run-02 | 7.155 s | 0% | 100% |
| run-03 | 9.460 s | 0% | 100% |
| **中位数** | **8.158 s** | **0%** | **100%** |

cold 中位数及三次单请求均满足 `<20 s`/`<25 s` 本地延迟目标。最大/最小耗时
相差 32.2%，同样需要 PostgreSQL 查询和磁盘读取数据辅助解释。

## 正确性验证

- 六次正式运行均无 HTTP 失败，k6 checks 成功率为 100%。
- 响应大小在六次运行中完全一致，且低于 256 KiB 安全上限。
- Stage 00 已在同一固定数据集上通过 120 请求、60 业务组合、零 mismatch 的
  REST/GraphQL 契约矩阵。

## 结论

本次建立了可追溯的 k6 warm/cold 数值基线，正确性和本地 SLO 通过。但实验归档仅有
k6 summary、元数据、数据集和 Git 差异，没有保存 API CPU/内存、goroutine、在途请求、
Redis hit/miss、PostgreSQL 查询/读取以及 API 日志的同窗口证据。在没有这些数据时无法
解释运行间波动，因此状态为“需要更多数据”，不将 Stage 1 标记为完成。

## 遗留风险与下一步

1. 让运行脚本按 `started_at`/`finished_at` 保存 Prometheus 查询结果和对应 API 日志，
   至少覆盖 Stage 1 列出的 API、Redis 和 PostgreSQL 必采指标。
2. 在同 commit、image、dataset、机器和负载下重跑三次 warm，核对 RPS 和 p99 波动是否
   可由 CPU、GC、Redis 命中或主机调度解释。
3. 补验通过后更新 Stage 1 验收报告，再开始 Stage 3 rankings SQL candidate 实验。
