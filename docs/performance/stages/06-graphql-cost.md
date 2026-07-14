# 阶段 6：GraphQL 成本控制

## 目标

让 GraphQL 查询成本有界、可预测、可观测，并消除关键查询的 N+1。

## 步骤

1. 设置请求体、查询深度、字段数量和复杂度上限。
2. 固定常用 operation，记录 resolver 与数据库调用次数。
3. 识别 N+1；需要时引入 DataLoader 并限制批大小。
4. 所有列表字段分页并限制最大 page size。
5. 生产环境关闭或限制 Playground 和 introspection。
6. 使用 operation name 或 persisted query 建立低基数指标。

## 验收标准

- 高深度或高 fan-out 查询在执行前被拒绝。
- 常用查询的数据库访问次数固定且可预测。
- GraphQL errors 与 HTTP transport errors 分别统计。
- 成本限制有契约测试，不依赖人工检查。
