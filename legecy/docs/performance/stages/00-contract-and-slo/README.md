# Stage 00：API 契约与 SLO

## 阶段目标

固定 API 表面、合法参数、代表性 workload、测试数据和可计算的本地性能目标，为后续
baseline 与 candidate 建立共同口径。

## 阅读顺序

| 顺序 | 文档 | 负责内容 |
|---:|---|---|
| 1 | [API 表面与路由清单](01-api-surface.md) | method、path、调用方、认证、依赖和范围 |
| 2 | [请求契约、工作负载与测试矩阵](02-request-contracts-and-workloads.md) | 标准参数、可执行请求、最大合法请求、k6 workload 和对等性矩阵 |
| 3 | [固定性能数据集](03-fixed-dataset.md) | stable ID、快照、精确分布、恢复和移动硬盘约束 |
| 4 | [SLO、容量哨兵与测量口径](04-slo-and-measurement.md) | 每项指标、数值目标、测量实现和生产边界 |
| 5 | [Stage 00 验收报告](05-stage-acceptance.md) | 验收证据、固定基线契约和阶段结论 |

## 完成标准

- 每个关键 API 都能从 [API 清单](01-api-surface.md)跳转到
  [可执行代表性请求](02-request-contracts-and-workloads.md#53-可直接执行)。
- 正常参数和最大合法参数均由
  [请求保护边界](02-request-contracts-and-workloads.md#10-最大合法请求和保护边界)定义。
- 另一位工程师可按 [数据集恢复命令](03-fixed-dataset.md#创建恢复和使用)重建相同数据。
- 延迟、失败率、吞吐、响应大小、陈旧时间和可比性都有
  [指标注册项与实现链接](04-slo-and-measurement.md#指标注册表)。
- 所有验收项均有 [证据链接](05-stage-acceptance.md#验收项)。

## 当前结论

Stage 00 已通过，详情见 [验收报告](05-stage-acceptance.md)。任何 API 参数、固定数据集、
game version、缓存语义或 SLO 变化都必须更新对应编号文档并建立新基线。
