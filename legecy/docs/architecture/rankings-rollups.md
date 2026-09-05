# Rankings Rollup 设计

## 1. 文档状态

- 状态：当前实现设计
- 更新日期：2026-07-20
- 适用范围：Ranked Solo/Duo（`queue_id = 420`）英雄排行榜
- 目标：让 rankings API 的 Redis-cold 请求只查询窄小的预聚合表，不再同步扫描
  `match_participants` 和 `match_bans` 大表。

本文以当前 migration、重建实现和 API SQL 为准。早期草稿中的
`position_mode`、`total_participants`、`UNCLASSIFIED` 和宽松数据准入设计已经废弃。

实现来源：

- Schema：`packages/sqlc/migrations/019_rankings_rollups.up.sql`
- Worker 通过 `packages/sqlc/migrations/embed.go` 内嵌同一套 canonical migrations；
- 重建逻辑：`apps/worker/internal/storage/rankings_rollup.go`
- 独立命令：`apps/worker/cmd/rankings-rollup/main.go`
- API 查询：`packages/sqlc/queries/rankings.sql`

## 2. 核心决策

1. Rollup 保存最低可组合粒度，不保存最终页面。
2. 只统计严格合格的 queue 420 比赛。
3. Champion、ban、match count 必须共享同一份 eligible match 集合。
4. 每个 eligible match 必须有 10 个完整 participant，五个位置各有双方一人。
5. `totalMatches` 按原子 scope 保存一次，overall 和各 position 共享该比赛数。
6. Overall 和 position pickRate 都使用 `totalMatches` 作为分母，表示英雄出现在多少比例的
   eligible matches 中。
7. 刷新采用单事务全量重建；发布前先在临时表验证不变量。
8. 当前不引入 generation/snapshot 多版本表，也不做逐场增量维护。

## 3. 数据流

```text
matches + match_participants + match_bans
                    │
                    ▼
       rankings_match_quality 临时表
       ├─ eligible
       ├─ metadata
       ├─ tier
       ├─ duration
       ├─ participant_shape
       ├─ participant_facts
       └─ ban_shape
                    │ eligible only
                    ▼
        三张临时 staging rollup 表
                    │
                    ▼
           发布前不变量校验
                    │ success
                    ▼
     单事务替换三张正式 rollup 表
                    │
                    ▼
             rankings API 查询
```

API 在线请求不会访问原始 participant/ban 大表。原始表扫描只发生在离线重建过程。

## 4. 原子 Scope

三张 rollup 数据表共享以下维度：

```text
queue_id
version
region
tier_bucket
```

示例：

```text
420 / 16.13 / KR / MASTER
420 / 16.13 / KR / GRANDMASTER
420 / 16.13 / KR / CHALLENGER
```

`tier_bucket` 保存原子的 `matches.avg_tier`，不保存重叠的 API tier group。

API 在查询时组合：

```text
master             = MASTER
master_plus        = MASTER + GRANDMASTER + CHALLENGER
grandmaster        = GRANDMASTER
grandmaster_plus   = GRANDMASTER + CHALLENGER
challenger         = CHALLENGER
```

当前允许的原子 tier：

```text
IRON, BRONZE, SILVER, GOLD, PLATINUM,
EMERALD, DIAMOND, MASTER, GRANDMASTER, CHALLENGER
```

空值、空字符串和未知 tier 不进入 rollup，不再使用 `UNCLASSIFIED` bucket。

## 5. 比赛准入规则

### 5.1 Source 集合

质量分类只检查：

```sql
fetch_status = 'done'
AND queue_id = 420
```

Pending/error 比赛和其他 queue 不属于 `source_completed_matches`，也不计入排除原因。

### 5.2 排除原因与优先级

每场 source match 只获得一个分类。优先级从上到下：

| 分类 | 条件 |
|---|---|
| `metadata` | version 或 region 为 NULL/空白 |
| `tier` | avg_tier 为 NULL 或不在固定 tier 枚举中 |
| `duration` | game_duration 为 NULL 或小于 600 秒 |
| `participant_shape` | participant 数量、ID、队伍或位置结构不完整 |
| `participant_facts` | champion、胜负或 K/D/A 事实无效 |
| `ban_shape` | ban slot 数量、队伍、pick turn 或 champion ID 完整性无效 |
| `eligible` | 上述检查全部通过 |

例如一场比赛同时缺少 version 且时长不足，按优先级只计入 `metadata`。

### 5.3 Participant Shape

每场 eligible match 必须满足：

- 恰好 10 行 participant；
- `participant_id` 在 1～10 中且恰好有 10 个不同值；
- team 100 恰好 5 人，team 200 恰好 5 人；
- 所有 participant 的 position 都属于：
  `TOP/JUNGLE/MIDDLE/BOTTOM/UTILITY`；
- `(team_id, team_position)` 恰好有 10 种组合。

因此每场 eligible match 必然满足：

```text
overall participant slots = 10
每个 position participant slots = 2
```

### 5.4 Participant Facts

每场 eligible match 还必须满足：

- 所有 `champion_id > 0`；
- champion name 非空；
- `win` 非 NULL；
- kills、deaths、assists 非 NULL 且非负；
- 恰好 10 个不同 champion；
- 恰好一支队伍五人获胜，另一支队伍无人获胜。

### 5.5 Ban Shape

每场 eligible match 必须包含完整的 10 个 ban slot：

- 恰好 10 行 ban；
- team 100 恰好 5 行，team 200 恰好 5 行；
- team 100 的 `pick_turn` 必须为 1～5，team 200 必须为 6～10；
- `(team_id, pick_turn)` 恰好有 10 种组合；
- `champion_id` 非 NULL。

`champion_id <= 0` 表示该 slot 没有实际禁用英雄，例如玩家主动 skip ban。这种 slot 不会
导致比赛被排除，也不进入任何英雄的 `ban_matches`。构建 ban rollup 时只统计
`champion_id > 0` 的 ban，并按 champion 对 `match_id` 去重。

## 6. Rollup 表

### 6.1 `rankings_champion_position_rollup`

粒度：

```text
scope × champion_id × team_position
```

字段：

| 字段 | 含义 |
|---|---|
| `queue_id/version/region/tier_bucket` | 原子 scope |
| `champion_id` | 英雄 ID |
| `champion_name` | 当前来源数据中的冗余名称 |
| `team_position` | 五个标准位置之一 |
| `games` | 该英雄在该位置出现的 participant 数量 |
| `wins` | 其中胜利 participant 数量 |
| `kda_contribution_sum` | 每局 `(kills + assists) / max(deaths, 1)` 的总和 |

不保存：

- losses；
- winRate、pickRate、banRate、KDA 最终值；
- kills/deaths/assists 总和；
- KDA contribution count。

严格事实校验保证每个 game 都有一个有效 KDA contribution，因此：

```text
KDA = SUM(kda_contribution_sum) / SUM(games)
```

`champion_name` 当前用于兼容既有 API。多语言名称应最终迁移到独立 champion catalog，
不应通过复制 rollup 支持 locale。

### 6.2 `rankings_champion_ban_rollup`

粒度：

```text
scope × champion_id
```

指标：

```text
ban_matches = COUNT(DISTINCT match_id)
```

Ban 没有 position，必须和 champion-position rollup 分开。即使英雄在某个原子 tier
中没有被选用，该 tier 的 ban 仍可在组合 tier group 时参与 banRate。

### 6.3 `rankings_match_count_rollup`

粒度：

```text
scope
```

指标：

```text
total_matches = eligible match 数量
```

不再按 position 重复保存 `totalMatches`。原因是 eligible match 已保证每个位置都有双方
各一人，所以同一 scope 下：

```text
overall totalMatches
= TOP totalMatches
= JUNGLE totalMatches
= MIDDLE totalMatches
= BOTTOM totalMatches
= UTILITY totalMatches
```

以上各行用等号连接，表示这些 `totalMatches` 数值相等，不表示相加。

### 6.4 `rankings_rollup_state`

这是单行刷新审计表，不是排行榜业务数据。

记录：

- `refreshed_at`；
- eligible 数据的 `data_through`；
- source completed match 数；
- eligible match 数；
- 六类互斥排除数量；
- 三张 rollup 的发布行数。

数据库约束保证：

```text
source_completed_matches
= eligible_matches
 + excluded_metadata_matches
 + excluded_tier_matches
 + excluded_duration_matches
 + excluded_participant_shape_matches
 + excluded_participant_facts_matches
 + excluded_ban_shape_matches
```

## 7. API 计算公式

### 7.1 Overall

对选中的原子 tier、version 和 region 按 champion 汇总所有 position：

```text
games    = SUM(games)
wins     = SUM(wins)
losses   = games - wins
winRate  = wins / games × 100
pickRate = games / totalMatches × 100
banRate  = banMatches / totalMatches × 100
KDA      = SUM(kda_contribution_sum) / games
```

英雄的 `teamPosition` 根据 position games 占该英雄 overall games 的比例动态计算：

```text
positionGames / championGames × 100 >= positionThreshold
```

### 7.2 By Position

先过滤一个标准 position，再按 champion 汇总：

```text
games    = SUM(position games)
wins     = SUM(position wins)
losses   = games - wins
winRate  = wins / games × 100
pickRate = games / totalMatches × 100
banRate  = banMatches / totalMatches × 100
KDA      = SUM(kda_contribution_sum) / games
```

该口径表示英雄在多少比例的 eligible matches 中以指定位置出场。由于每场比赛有双方
两名该位置 participant，该位置所有英雄的 pickRate 合计通常为 200%；overall 的合计通常
为 1000%。这些合计不需要归一到 100%。严格准入保证一场比赛内英雄不重复，因此单个英雄
的 overall 或单位置 pickRate 都不会超过 100%。

### 7.3 动态参数

以下参数只影响 rollup 查询，不要求重建：

- `minGames`：最终 champion 汇总后的过滤条件；
- `positionThreshold`：只影响 overall 返回的位置列表；
- tier group：组合不同原子 tier；
- version、region、position：过滤对应原子维度。

`queue_id` 当前固定为 420。其他 queue 请求不会命中数据；如需支持 Ranked Flex，应先扩展
schema 约束、eligibility 和测试，而不是仅开放 API 参数。

## 8. 重建与发布流程

`RebuildRankingsRollups` 执行以下步骤：

1. 开启数据库事务；
2. 获取事务级 PostgreSQL advisory lock，禁止并发 rebuild；
3. 建立 `rankings_match_quality` 临时表，一次性分类 source matches；
4. 为质量表建立唯一索引并 `ANALYZE`；
5. 从同一 eligible 集合构建三张临时 staging 表；
6. 校验发布前不变量；
7. 校验通过后删除旧正式 rollup；
8. 将三张 staging 表写入正式表；
9. 写入 `rankings_rollup_state`；
10. `ANALYZE` 正式 rollup 并提交事务。

发布前必须满足：

```text
scope total games = totalMatches × 10
scope total wins  = totalMatches × 5
每个 position games = totalMatches × 2
```

任何分类、构建、约束或不变量失败都会回滚整个事务。旧的已提交 rollup 保持可读，不会
发布部分更新。

当前采用 in-place 原子替换，没有多 generation 保留和快速回滚指针。如果未来需要保留
历史快照、跨库发布或刷新后人工验收，再引入 generation。

## 9. 运行方式

```bash
go run ./apps/worker/cmd/rankings-rollup \
  --database-dsn 'postgres://user:password@host:5432/gogg?sslmode=disable' \
  --timeout 30m
```

也可以通过环境变量提供 DSN：

```bash
GOGG_DATABASE_DSN='postgres://...' \
go run ./apps/worker/cmd/rankings-rollup
```

命令会先运行 `InitSchema`，再重建 rollup，并输出刷新时间、数据截止时间、eligible 数量、
各排除原因、rollup 行数和耗时。

## 10. 测试与正确性门禁

### 10.1 PostgreSQL 集成测试

`rankings_rollup_integration_test.go` 在独立临时数据库中验证：

- 所有 eligibility 条件和分类优先级；
- source/eligible/exclusion 数量对账；
- champion、ban 和 match count 聚合；
- overall/position API 公式；
- schema CHECK 约束；
- 重复 rebuild 的幂等性；
- 原始数据变化后的全量替换；
- migration down 删除全部 rollup 表。

本地运行：

```bash
GOGG_INTTEST=1 \
GOGG_TEST_DATABASE_DSN='postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable' \
go test -tags=integration -count=1 ./apps/worker/internal/storage
```

CI 使用 PostgreSQL 16 service 运行相同的 race integration test，并检查 sqlc 生成代码
没有漂移。

## 11. 与 Redis 的边界

Rollup 的第一目标是保证 Redis miss/不可用时，API 仍然能在小表上快速查询。

当前 Redis 行为仍由 API 的 read-through cache 控制；rollup rebuild 本身：

- 不生成最终 120 个页面；
- 不主动预热 Redis；
- 不主动删除或切换 Redis key；
- 不负责 Redis generation。

因此 refresh 后最多仍可能在现有 Redis TTL 内读取旧页面。Redis 常驻页面和原子发布是
后续独立设计，不影响 rollup schema 的原子粒度。

## 12. 当前限制与后续工作

1. **全量重建**：当前扫描所有完成的 queue 420 历史数据，没有只限定最新两个版本。
2. **刷新触发未接入 crawler**：建议在 avg_tier 已完成的 phase4 或完整 crawl 批次后触发，
   当前仍需独立执行命令。
3. **Redis 未失效**：重建完成后现有缓存可能继续服务旧数据直到 TTL 到期。
4. **没有 generation**：事务保证原子发布，但不保留上一代快照供快速回切。
5. **Champion 多语言未拆分**：当前仍依赖冗余的 `champion_name`。
6. **只支持 queue 420**：其他 queue 必须通过新设计显式扩展。
7. **最新严格版本尚需重新做固定数据集性能实验**：
   `EXP-20260719-ROLLUP-01` 的 16.325 ms cold 中位数来自更早的宽松 rollup prototype；
   当时 schema 仍有 `position_mode` 和 `total_participants`。该结果证明方向可行，但不能
   直接作为当前严格 eligibility 版本的最终验收数据。

## 13. Migration 019 环境注意事项

Migration 019 仍处于未提交开发阶段，因此当前源文件直接反映最新 schema。全新数据库会
正确应用最新定义。

如果某个本地或性能数据库已经应用过早期版本的 migration 019，migration 工具只会看到
版本号 19，不会自动重新执行被改写后的 SQL。这样的数据库可能仍保留旧字段和旧约束，
不能视为符合本文设计。

在合并前应选择一种处理方式：

- 对可丢弃的本地/性能数据库，从固定快照重新创建并应用最新 migration；或
- 保留已有环境时，新增后续 migration 将旧 019 schema 显式迁移到最新定义。

不要仅凭 `schema_migrations.version = 19` 判断 rollup schema 已经是最新版本。
