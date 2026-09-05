# 基线数据快照记录

本文件记录阶段 1 正式基线采用的数据集。数据集定义和恢复方法见
[Stage 00 固定数据集](../00-contract-and-slo/03-fixed-dataset.md)。

## 当前状态

- 稳定数据集编号：`rankings-kr-16.13-current-v1`
- 快照校验和：`a9402acd48c1a5229581dffb41fc72dffea54bc355bf58638741925dd54e9a43`
- schema migration 版本：`18`（`dirty=false`）
- PostgreSQL 版本：`16.14`
- 快照位置：移动硬盘
  `/mnt/gogg-perf/snapshots/rankings-kr-16.13-current-v1/database.dump`
- 恢复命令：`tests/performance/perf-drive.sh restore`

## 关键规模和分布

| 项目 | 值 |
|---|---:|
| 16.13 matches | 286,332 |
| KR matches | 159,644 |
| KR match participants | 1,596,440 |
| KR timeline done / error | 159,444 / 200 |
| 固定 game version | 16.13 |
| KR 数据时间范围 | 2026-06-24 05:17:14 UTC 至 2026-07-15 05:52:19 UTC |

快照是当前整库的逻辑备份；正式 rankings 场景必须固定使用 `region=KR` 和
`version=16.13`。当前运行自动生成的 `dataset.env` 只作为辅助检查，不能代替本记录。

移动硬盘必须以 UUID `cca4d6cf-e43e-4a46-be1e-38c73d4dc341` 读写挂载在
`/mnt/gogg-perf`。管理脚本会在 UUID 不符、只读或未挂载时拒绝操作，避免数据落到
系统盘。专用 PostgreSQL 监听 `localhost:55434`；拔盘前先运行：

```bash
tests/performance/perf-drive.sh down
sudo umount /mnt/gogg-perf
```
