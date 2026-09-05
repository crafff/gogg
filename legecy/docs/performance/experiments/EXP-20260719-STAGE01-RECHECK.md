# EXP-20260719-STAGE01-RECHECK：Stage 1 观测补验

- 所属阶段：1（可重复基线）
- 状态：接受
- 日期：2026-07-19
- 假设：保存每次运行的资源和日志窗口后，可以解释 warm 基线的运行间波动。
- 成功条件：三次 warm 均通过契约与 SLO，观测归档完整，吞吐和尾延迟波动可解释。
- 回滚方式：本实验未修改 API 运行时代码；归档脚本可独立回滚。

## 环境与运行

- commit：`e456d75d10b183d1d4a28e5fdcd56ed33455ab9f`
- API image：`sha256:ad41a7b9457195e28dc73e586cf2c1644809d8653ec37d400e204fe08214cc8b`
- 数据集：`rankings-kr-16.13-current-v1`，dataset ID `509927be4b03`
- 场景：`rankings_graphql`，KR / 16.13 / master_plus / overall
- 负载：20 VU、1 分钟，先执行一次不计入统计的 warmup
- 原始目录：`tmp/performance/EXP-20260719-STAGE01-RECHECK/`（不提交 Git）

每个正式 run 的 `dataset_id`、API image、负载和主机一致，`exit_code=0`、
`observation_exit_code=0`。工作区为 dirty，但被测 API 使用与首次实验一致的不可变
image digest；差异快照随各 run 归档。

## 结果

| run | k6 rate | load 请求数 | 请求数 / 60s | p50 | p95 | p99 | max |
|---|---:|---:|---:|---:|---:|---:|---:|
| run-01 | 185.1 req/s | 10,830 | 180.5 req/s | 6.50 ms | 14.95 ms | 22.42 ms | 45.14 ms |
| run-02 | 164.3 req/s | 10,757 | 179.3 req/s | 6.70 ms | 17.68 ms | 29.69 ms | 124.89 ms |
| run-03 | 188.1 req/s | 11,006 | 183.4 req/s | 5.34 ms | 13.38 ms | 21.63 ms | 42.37 ms |
| **中位数** | **185.1 req/s** | **10,830** | **180.5 req/s** | **6.50 ms** | **14.95 ms** | **22.42 ms** | **45.14 ms** |

三次 checks 均为 100%，HTTP 失败率为 0%，响应均为 30,068 bytes。所有正式运行
满足 warm rankings 的 p95、p99、失败率、吞吐量和响应大小目标。

## 观测与归因

每次运行保存了 5 秒步长的 API CPU/内存/heap/goroutine/在途请求、Redis hit/miss
和内存、PostgreSQL 连接/块读取/事务，另保存 API 日志、容器前后资源快照以及在运行
前重置后的 `pg_stat_statements`。

run-01 和 run-03 在整个窗口没有 PostgreSQL 块读取或 Redis miss。run-02 的 setup
遇到排行榜缓存 TTL 到期，记录到一次 `ListOverallRankings`，耗时 6,912.46 ms；同期
Redis 出现一次 miss，PostgreSQL 块读取率短暂上升。setup 完成后的 load 请求仍由
Redis 服务，没有持续数据库查询。

k6 导出的 `http_reqs{phase:load}` rate 使用包含 setup 的进程墙钟时间作为分母，因而
run-02 的 6.91 秒冷聚合将名义 rate 压低到 164.3 req/s。用固定的一分钟 load 请求数
计算，三次为 180.5、179.3、183.4 req/s，最大与最小相差约 2.3%。p99 均低于 30 ms，
绝对值和波动都远低于 250 ms 目标。

## 结论

假设成立。warm load 在固定数据集、镜像和负载下可重复，原先无法解释的主要吞吐
波动是 setup 缓存过期与 k6 rate 分母共同造成的测量效应。资源和日志窗口现已自动
归档，Stage 1 验收通过，可以开始 Stage 3 rankings SQL candidate 分析。
