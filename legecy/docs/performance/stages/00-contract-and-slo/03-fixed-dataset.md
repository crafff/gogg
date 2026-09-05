# 固定性能数据集

## 目标和状态

保证 baseline 和 candidate 使用同一批数据，并能判断两个运行是否具备可比性。

本阶段的正式数据集已经固定并完成恢复验证。性能运行仍会把
`pg_stat_user_tables.n_live_tup` 写入 `dataset.env`，并把其 12 位哈希写入
`metadata.env`；该值只用于快速发现明显变化，不能代替本文件记录的稳定编号和
快照 SHA-256。

## 正式数据集

| 项目 | 值 |
|---|---|
| 稳定编号 | `rankings-kr-16.13-current-v1` |
| 数据来源 | 当前开发/采集 PostgreSQL 数据库的整库逻辑快照 |
| rankings 固定范围 | `region=KR`、`version=16.13`、`queue_id=420` |
| PostgreSQL | `16.14` |
| schema migration | `18`，`dirty=false` |
| 快照格式 | `pg_dump --format=custom --compress=6 --no-owner --no-acl` |
| 快照大小 | 约 2.5 GB |
| 快照 SHA-256 | `a9402acd48c1a5229581dffb41fc72dffea54bc355bf58638741925dd54e9a43` |
| 快照位置 | `/mnt/gogg-perf/snapshots/rankings-kr-16.13-current-v1/database.dump` |
| 专用数据库 | `postgres://gogg:goggpass@localhost:55434/gogg?sslmode=disable` |

快照保留了整库规模和索引环境，但正式 rankings 请求必须显式传递 `KR` 和
`16.13`，不得使用 `latest`。完整机器可读清单位于快照目录的 `manifest.txt`，
`SHA256SUMS` 同时校验 dump 和 manifest。

## 关键规模和分布

| 项目 | 精确行数 |
|---|---:|
| 16.13 matches（所有 region） | 286,332 |
| KR 16.13 matches | 159,644 |
| NA1 16.13 matches | 126,688 |
| KR 16.13 match participants | 1,596,440 |
| KR timeline `done` | 159,444 |
| KR timeline `error` | 200 |

KR 16.13 的 tier 分布：

| tier | matches |
|---|---:|
| CHALLENGER | 4,542 |
| GRANDMASTER | 8,880 |
| MASTER | 127,530 |
| DIAMOND | 15,371 |
| EMERALD | 2,471 |
| PLATINUM | 692 |
| GOLD | 134 |
| SILVER | 22 |
| BRONZE | 2 |

KR 16.13 的 participant position 分布：

| position | participants |
|---|---:|
| TOP | 319,240 |
| JUNGLE | 319,273 |
| MIDDLE | 319,267 |
| BOTTOM | 319,262 |
| UTILITY | 319,250 |
| 空值 | 148 |

比赛时间范围为 `2026-06-24 05:17:14 UTC` 至
`2026-07-15 05:52:19 UTC`。精确水位保存在 `manifest.txt`；上述 200 条 timeline
错误是冻结数据集的一部分，不在 baseline 和 candidate 之间修补。若修补或重新导出，
必须使用新的稳定编号和校验和。

## 移动硬盘约束

快照和恢复后的 PostgreSQL 数据目录均位于移动硬盘。管理脚本要求：

- 挂载点为 `/mnt/gogg-perf`；
- 文件系统 UUID 为 `cca4d6cf-e43e-4a46-be1e-38c73d4dc341`；
- 挂载选项包含 `rw`；
- 当前用户对挂载点可写。

任一条件不满足时脚本立即退出，避免移动硬盘缺失时误写系统盘。设备名可以从
`/dev/sdd` 变成其他值，身份判断不依赖设备名。

## 创建、恢复和使用

创建快照只执行一次；脚本拒绝覆盖已有的 `database.dump`：

```bash
tests/performance/perf-drive.sh snapshot
```

恢复前会验证 `SHA256SUMS`，然后只在专用 PostgreSQL 实例中重建 `gogg` 数据库并
执行 `ANALYZE`，不会修改开发数据库：

```bash
tests/performance/perf-drive.sh restore
```

恢复结果已经验证为 KR 159,644 场、NA1 126,688 场和 KR 1,596,440 条 participant，
与源清单一致。数据库文件长期保存在 `/mnt/gogg-perf/postgres-16`。普通只读实验无需
每次恢复：

```bash
tests/performance/perf-drive.sh up
tests/performance/perf-drive.sh status
```

为了获得严格可比的 baseline/candidate，推荐实验开始前恢复一次，切换 candidate 后
再从同一快照恢复一次；同一 variant 的三次只读运行之间不恢复。

拔盘前必须停止专用 PostgreSQL 并卸载文件系统：

```bash
tests/performance/perf-drive.sh down
sudo umount /mnt/gogg-perf
```

## 数据分类

数据来源已确认为开发/采集库，不含生产数据或正式用户数据。快照可能包含 Riot
采集标识（例如 PUUID、game name 和 tag line），因此仍按内部开发数据管理：仅存放在
受控移动硬盘，不提交 Git，不上传公共制品，也不对外分发。

## 可比性和验收

本数据集已满足以下条件：

- 可通过仓库内固定命令恢复；
- dump 和 manifest 具有完整 SHA-256；
- 恢复后的关键精确行数与源清单一致；
- baseline 与 candidate 使用相同稳定编号、dump 校验和、固定 region/version 和
  恢复流程；
- PostgreSQL、migration、关键分布、时间范围、已知错误和数据分类均已记录。

若 dump SHA-256、manifest、固定请求范围或任何关键精确行数不同，则两个运行不具备
可比性，不能纳入同一组性能结论。
