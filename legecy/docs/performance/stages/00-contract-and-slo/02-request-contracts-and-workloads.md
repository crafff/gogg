# 请求契约、工作负载与测试矩阵

本文档定义排行榜 API 的标准请求、参数空间、性能 workload 和正确性验收规则。
它是阶段 0 的契约产物，也是 k6 场景与自动化契约测试的输入。

参数、operation 或响应契约发生变化时，必须更新本文档并建立新基线；不同契约的
结果不能直接比较。

## 1. 范围

当前最高优先级业务接口是英雄排行榜，提供两种 transport：

- GraphQL：`POST /graphql`，Web 的实际访问路径和主要性能基线。
- REST：`GET /api/v1/rankings/champions`，兼容接口和 GraphQL 对照组。

两种 transport 对等参数必须返回相同的排行榜数据。catalog 的 `versions` 和
`regions` 只作为依赖与轻量基线，不代表排行榜主路径。

## 2. 固定术语

| 术语 | 含义 |
|---|---|
| overall / `ALL` position | 不指定位置，按英雄聚合所有位置 |
| by-position | 指定一个位置，只聚合该位置的数据 |
| `latest` | 请求时由服务解析出的最新具体游戏版本 |
| `master_plus` | MASTER、GRANDMASTER、CHALLENGER |
| `grandmaster_plus` | GRANDMASTER、CHALLENGER |
| complete roster | 返回全部符合过滤条件和 `minGames` 的英雄 |

文档中的北美区域统一使用 API region code `NA1`，不使用简称 `NA`。

## 3. 标准参数空间

| 维度 | 正式值 | 说明 |
|---|---|---|
| queue | `420` | Ranked Solo/Duo |
| region | `KR`、`NA1` | 第一版常用区域 |
| version | `16.13` | 固定数据集的具体版本；性能基线禁止使用 `latest` |
| tier | `master_plus`、`master`、`grandmaster_plus`、`grandmaster`、`challenger` | REST 值 |
| tierGroup | `MASTER_PLUS`、`MASTER`、`GRANDMASTER_PLUS`、`GRANDMASTER`、`CHALLENGER` | GraphQL 值 |
| position | overall、`TOP`、`JUNGLE`、`MIDDLE`、`BOTTOM`、`UTILITY` | overall 使用空字符串/省略参数 |
| minGames | `20` | 固定样本下限 |
| positionThreshold | `5.0` | 百分点；仅 overall 使用 |

排行榜不接受客户端行数控制。服务端固定 500 行安全上限，正常英雄数据不应触发。

## 4. 标准 GraphQL 请求

operation name 固定为 `ChampionRankings`，指标只使用 operation name，禁止把完整
query 文本作为 Prometheus 标签。

```graphql
query ChampionRankings($filter: ChampionRankingsFilter) {
  championRankings(filter: $filter) {
    items {
      championId
      championName
      teamPosition
      games
      wins
      losses
      winRate
      pickRate
      banRate
      kda
    }
    totalMatches
    resolvedVersion
  }
}
```

### 4.1 KR master+ overall

```json
{
  "operationName": "ChampionRankings",
  "variables": {
    "filter": {
      "queueId": 420,
      "region": "KR",
      "version": "16.13",
      "tierGroup": "MASTER_PLUS",
      "position": "",
      "minGames": 20,
      "positionThreshold": 5.0
    }
  }
}
```

### 4.2 KR master+ by-position

下面以 `MIDDLE` 为例；正式位置集合为 `TOP`、`JUNGLE`、`MIDDLE`、`BOTTOM`、
`UTILITY`。

```json
{
  "operationName": "ChampionRankings",
  "variables": {
    "filter": {
      "queueId": 420,
      "region": "KR",
      "version": "16.13",
      "tierGroup": "MASTER_PLUS",
      "position": "MIDDLE",
      "minGames": 20,
      "positionThreshold": 5.0
    }
  }
}
```

`positionThreshold` 在 by-position 查询中被忽略，但标准请求仍显式传入，保证请求
形状稳定。

## 5. 标准 REST 请求

### 5.1 KR master+ overall

```http
GET /api/v1/rankings/champions?queueId=420&region=KR&version=16.13&tier=master_plus&minGames=20&positionThreshold=5
Accept: application/json
```

### 5.2 KR master+ by-position

```http
GET /api/v1/rankings/champions?queueId=420&region=KR&version=16.13&tier=master_plus&position=MIDDLE&minGames=20&positionThreshold=5
Accept: application/json
```

其他 region、tier 和 position 通过第 3 节的正式值替换，不复制新的请求正文。

### 5.3 可直接执行

REST：

```bash
curl -fsS 'http://localhost:18080/api/v1/rankings/champions?queueId=420&region=KR&version=16.13&tier=master_plus&minGames=20&positionThreshold=5'
```

GraphQL：

```bash
curl -fsS http://localhost:18080/graphql \
  -H 'Content-Type: application/json' \
  --data-binary '{"operationName":"ChampionRankings","query":"query ChampionRankings($filter: ChampionRankingsFilter) { championRankings(filter: $filter) { items { championId championName teamPosition games wins losses winRate pickRate banRate kda } totalMatches resolvedVersion } }","variables":{"filter":{"queueId":420,"region":"KR","version":"16.13","tierGroup":"MASTER_PLUS","position":"","minGames":20,"positionThreshold":5}}}'
```

## 6. 核心性能 workload

一次普通 SQL 优化实验不运行全部组合。baseline 和 candidate 必须使用相同 workload
ID、数据集、解析版本、负载和 cache mode。

| Workload ID | Transport | Region | Tier | Position | 等级 | 用途 |
|---|---|---|---|---|---|---|
| `RKG-GQL-KR-MP-ALL` | GraphQL | KR | master+ | overall | P0 | 默认页面，主要性能基线 |
| `RKG-GQL-KR-MP-MID` | GraphQL | KR | master+ | MIDDLE | P0 | 代表 by-position SQL |
| `RKG-GQL-NA1-MP-ALL` | GraphQL | NA1 | master+ | overall | P0 | 第二常用区域及数据分布 |
| `RKG-GQL-NA1-MP-MID` | GraphQL | NA1 | master+ | MIDDLE | P1 | 第二区域的 position 路径 |
| `RKG-GQL-KR-C-ALL` | GraphQL | KR | challenger | overall | P1 | 小数据量 tier 对照 |
| `RKG-REST-KR-MP-ALL` | REST | KR | master+ | overall | P1 | REST 性能与兼容对照 |
| `RKG-REST-KR-MP-MID` | REST | KR | master+ | MIDDLE | P1 | REST position 对照 |

日常单点优化至少运行两个 P0 KR workload；涉及 region 过滤、缓存 key 或数据分布时
增加 NA1；涉及 tier 过滤时增加 challenger。发布前测试运行全部 P0/P1 workload。

所有性能 workload 都要分别执行：

- warm：一次不计入统计的预热，然后至少三次正式持续负载。
- cold：至少三次独立清缓存单请求。

## 7. 完整正确性矩阵

`SCENARIO=contract_matrix` 自动生成组合，不在本文档手写 120 个请求。

```text
2 regions
× 5 tiers
× 6 position modes（overall + 5 positions）
= 60 个业务组合
```

每个业务组合分别调用 GraphQL 和 REST，共 120 个 transport 请求。k6 会按
`championId` 排序并比较 items 和 `totalMatches`，任一差异使
`gogg_contract_mismatches` 门禁失败。完整矩阵是契约测试，不要求每个组合都执行
一分钟性能负载：

```bash
PERF_SCENARIO=contract_matrix PERF_VUS=1 PERF_DURATION=1m \
  tests/performance/run-api-performance.sh warm
```

映射规则：

| REST tier | GraphQL tierGroup | 数据库 tier 集合 |
|---|---|---|
| `master_plus` | `MASTER_PLUS` | MASTER、GRANDMASTER、CHALLENGER |
| `master` | `MASTER` | MASTER |
| `grandmaster_plus` | `GRANDMASTER_PLUS` | GRANDMASTER、CHALLENGER |
| `grandmaster` | `GRANDMASTER` | GRANDMASTER |
| `challenger` | `CHALLENGER` | CHALLENGER |

## 8. 每个请求的验收断言

### 8.1 Transport

- REST 返回 HTTP `200` 和 JSON Content-Type。
- GraphQL 返回 HTTP `200`，且 `errors` 不存在或为空。
- 请求超时和客户端失败率满足对应实验阈值。

### 8.2 数据形状

- `items` 是数组且长度不超过 500。
- `championId` 为正数，同一响应内不重复。
- `championName` 非空。
- `games >= minGames`。
- `wins + losses = games`。
- `winRate`、`pickRate`、`banRate` 均在 `[0, 100]`，`kda` 为非负有限值。
- overall 的 `teamPosition` 可包含多个位置；by-position 必须只包含请求位置。
- `totalMatches >= 0`。
- 固定版本请求必须使用 `16.13`；REST `meta.version` 必须为 `16.13`。GraphQL 的
  `resolvedVersion` 只在请求 `latest` 时非空，因此固定版本请求允许其为 null。

### 8.3 REST/GraphQL 对等性

对同一业务组合，将结果按 `championId` 排序后比较：

- 英雄集合一致。
- games、wins、losses 和 teamPosition 一致。
- winRate、pickRate、banRate、kda 一致。
- totalMatches 一致；双方实际查询版本均为固定的 `16.13`。

### 8.4 完整结果与安全上限

- 请求不包含 `limit`。
- 返回所有满足过滤条件和 `minGames` 的英雄，而不是固定前 40/100 条。
- 正常数据下结果应显著少于 500；如果达到 500，测试失败并报告数据完整性风险，
  不能把截断结果当作成功。

## 9. Catalog 辅助请求

```http
GET /api/v1/versions
GET /api/v1/regions
```

```graphql
query Versions {
  versions
}

query Regions {
  regions
}
```

这些请求用于验证可用 version 和 region 选项，也可测量轻量 API/GraphQL
框架开销，但不能替代 `ChampionRankings` 性能基线。

## 10. 最大合法请求和保护边界

REST 与 GraphQL 共用以下 rankings filter 合法边界：

| 字段 | 合法范围 |
|---|---|
| `queueId` | 整数 `[0, 9999]` |
| `minGames` | 整数 `[1, 20000]` |
| `positionThreshold` | 有限数值 `[0, 100]` |
| `version` | 空、`latest` 或最长 16 字符的数字 patch（如 `16.13`、`16.13.1`） |
| `region` | 空、`KR`、`NA1` |
| `tier` / `tierGroup` | ALL/空或第 3 节列出的五组 |
| `position` | overall/空或五个正式位置 |

REST query string 最大 4096 bytes，超过返回 `414`；GraphQL request body 最大
64 KiB，超过返回 `413`；GraphQL calculated complexity 最大 300。排行榜返回固定最多
500 行，不接受客户端 limit。

数值最大的合法边界请求用于验证解析与保护契约：

```http
GET /api/v1/rankings/champions?queueId=9999&region=KR&version=16.13&tier=challenger&position=UTILITY&minGames=20000&positionThreshold=100
```

计算成本最大的代表性合法请求采用全量 overall 过滤，而不是数值最大值：

```http
GET /api/v1/rankings/champions?queueId=420&region=KR&version=16.13&tier=master_plus&minGames=1&positionThreshold=0
```

非法数字、越界数值、未知 region/tier/position 和非法 version：REST 返回 `400`，
GraphQL 返回 HTTP `200` 且 error extension `code=BAD_USER_INPUT`。

## 11. 实施状态

- k6 已提供 `rankings_graphql`、`rankings_rest` 和全部 workload ID。
- `contract_matrix` 已实现 60 个业务组合、120 个 transport 请求的自动对等性检查。
- 正式版本固定为 `16.13`，稳定数据集为 `rankings-kr-16.13-current-v1`。
- 参数边界由 REST 和 GraphQL 共用的验证器执行。
