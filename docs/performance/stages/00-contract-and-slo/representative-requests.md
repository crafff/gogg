# 代表性请求

本文件固定性能测试使用的典型请求和最大合法请求。参数发生变化时必须更新文档，
并创建新的基线，不能与旧参数直接比较。

## Rankings

典型请求：

```http
GET /api/v1/rankings/champions?region=KR&version=latest&tier=master_plus
```

当前 k6 场景名：`rankings`。

待补充：

- 验证接口返回全部符合条件的英雄，客户端不能控制行数上限。
- 验证服务端固定 500 行安全上限不会在正常英雄数据下触发。
- position、tier、version 的边界组合。
- 预期状态码、Content-Type、响应 schema 和关键正确性断言。
- `latest` 是否会随数据发布变化；必要时改用固定 version。

## Catalog

```http
GET /api/v1/versions
GET /api/v1/regions
```

当前 k6 场景名分别为 `versions` 和 `regions`。待补充预期响应规模及正确性断言。

## GraphQL

```graphql
query Versions {
  versions
}
```

当前 k6 场景名：`graphql`，operation name：`Versions`。后续还需补充一个真实常用
查询和一个最坏合法查询，简单的 `versions` 查询不能代表 GraphQL 主路径成本。
