# 阶段 3：优化 PostgreSQL 查询

本文件是阶段入口；查询分析和方案文档在开展后添加到本目录。

## 目标

降低 rankings 冷聚合耗时，并量化索引或 SQL 重写对 Worker 写入和存储的影响。

## 前置条件与非目标

必须先固定 rankings 参数、数据集和三次基线。本阶段一次只验证一个 SQL 或索引
假设，不同时改变缓存 TTL、API 超时或容器资源。

## 步骤

1. 提取 `ListOverallRankings` 和 `ListRankingsByPosition` 的实际 SQL 与参数。
2. 执行 `EXPLAIN (ANALYZE, BUFFERS, WAL, SETTINGS)`。
3. 记录顺序/索引扫描、估算与实际行数、落盘 sort、shared hit/read、临时块及
   最昂贵执行节点。
4. 更新统计信息并确认数据分布，再检查现有索引是否使用。
5. 依次验证复合/部分索引、减少重复扫描、SQL 重写、预聚合或物化视图。
6. 每个候选方案运行正确性测试、cold/warm 性能测试和 Worker 写入测试。

## 风险与停止条件

- 不对修改语句裸跑 `EXPLAIN ANALYZE`；必须放入最终回滚的事务。
- 测试导致数据库持续饱和、磁盘异常增长或影响共享环境时立即停止。
- 索引提升读取但显著恶化 Worker 写入时不能直接接受。

## 验收标准

- 冷查询达到阶段 0 确定的目标，执行计划在目标数据规模下稳定。
- 新索引空间、维护成本和 Worker 写入代价已量化。
- REST/GraphQL 契约测试和结果正确性通过。
