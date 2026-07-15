# 测试数据定义

## 目标

保证 baseline 和 candidate 使用同一批数据，并能判断两个运行是否具备可比性。

## 当前采集方式

每次运行把 `pg_stat_user_tables.n_live_tup` 按表名排序写入 `dataset.env`，并计算
12 位 `dataset_id` 写入 `metadata.env`。

该值适合快速发现明显的数据规模变化，但 `n_live_tup` 是统计估算，不是严格的
数据快照标识。因此当前阶段仍未完成。

## 正式数据集需要记录

- 数据来源和生成/恢复命令。
- PostgreSQL 版本、schema migration 版本。
- 关键表精确行数。
- rankings 相关 region、version、tier、position 分布。
- 关键时间范围以及 `MAX(updated_at)` 等水位。
- 数据导出文件的校验和，或可重复的数据生成 seed。
- 数据集中是否含敏感或生产数据及其处理方式。

## 数据集编号

建议使用稳定编号，例如 `rankings-kr-v1`，并同时记录快照校验和。统计估算生成的
临时 `dataset_id` 不能替代稳定编号。

## 验收条件

- 另一个工程师可以通过文档命令恢复相同数据集。
- 同一数据集重复恢复得到相同校验和和关键分布。
- baseline 与 candidate 的稳定数据集编号和校验和一致。
