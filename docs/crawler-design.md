# Gogg 爬虫模块设计文档

## 1. 概览

本模块负责从 Riot API 采集英雄联盟高端局数据，存入本地 PostgreSQL 数据库，供后续查询分析（胜率、出装、BP 等）使用。整体分为八个阶段（Phase 0–5.5），支持多地区并行爬取、断点续跑与增量/历史模式。

### API 限速约束

| 窗口 | 上限 |
|------|------|
| 每 1 秒 | 20 次请求 |
| 每 2 分钟 | 100 次请求 |

客户端实现**双窗口令牌桶**同时满足两个约束，不依赖 API 返回 429 触发等待。每个地区的 Riot 客户端有独立的限速计数器，多地区并行时互不干扰。

---

## 2. 多地区支持

### 2.1 地区配置

`config.yaml` 通过 `regions:` 列表定义多个地区，每个地区可配置独立的 API key 和 platform URL：

```yaml
riot:
  api_key: "SHARED_API_KEY"

regions:
  - name: KR
    base_url: "https://kr.api.riotgames.com"
  - name: NA1
    base_url: "https://na1.api.riotgames.com"
    api_key: "NA_SPECIFIC_KEY"
```

不配置 `regions:` 时，自动从 `riot.base_url` 合成单地区 `KR`，保持向下兼容。

Platform URL 自动映射到对应 regional URL（账号/比赛 V5 接口使用 regional routing）：

| Platform | Regional URL |
|----------|-------------|
| kr, jp1 | asia.api.riotgames.com |
| euw1, eun1, tr1, ru | europe.api.riotgames.com |
| na1, br1, la1, la2 | americas.api.riotgames.com |
| oc1 | sea.api.riotgames.com |

### 2.2 并行爬取

每个地区对应独立的 `Runner` 实例，拥有独立的 Riot 客户端和各自的 advisory lock。`daemon` 模式下，相同 cron 时间的多个 profile（不同地区）在独立 goroutine 中并发执行，共享同一个 DB 连接池。

### 2.3 Advisory Lock（每地区独立）

每个地区使用独立的 advisory lock，key 由地区名的 FNV-32 哈希派生：

```go
key = int64(fnv32a("gogg:" + region))
```

---

## 3. Run 生命周期

### 3.1 Run 状态变化

| 退出情况 | `status` | `ended_at` |
|---------|---------|------------|
| 所有 phase 完毕 | `completed` | 写入 |
| Ctrl+C / SIGTERM | `running`（不变） | 不写 |
| Phase 执行错误 | `failed` | 写入 |
| 进程 crash / kill -9 | `running`（不变） | 不写，lock 自动释放 |

### 3.2 断点续跑逻辑

`current_phase` 记录最后一次 `SaveCheckpoint` 时的 phase ID。Resume 时根据 `current_phase` 构建 `donePhases` set（基于执行顺序位置，不依赖 phase ID 数值大小），跳过已完成的 phase：

| `current_phase` | 行为 |
|---|---|
| 0 或 1 | 重置到 Phase 0，全部重跑 |
| 2 | 跳过 Phase 0/1，从 Phase 2 继续 |
| 3 | 跳过 Phase 0/1/2，从 Phase 3 继续 |
| 35 | 跳过 Phase 0/1/2/3，从 Phase 3.5 继续 |
| 4 | 跳过 Phase 0–3.5，从 Phase 4 继续 |
| 5 | 跳过 Phase 0–4，从 Phase 5 继续 |
| 55 | 跳过 Phase 0–5，从 Phase 5.5 继续 |

执行顺序：`[0, 1, 2, 3, 35, 4, 5, 55]`

---

## 4. 执行策略

### Sequential 模式

```
Phase 0 → Phase 1 → Phase 2 → Phase 3 → Phase 3.5 → Phase 4 → Phase 5 → Phase 5.5
```

### Pipeline 模式（默认）

```
Phase 0（一次）
→ Phase 1（全部 rank_prefetch_tiers，一次）
→ 对每个 target tier：
    Phase 2 → Phase 3 → Phase 3.5 → Phase 4 → Phase 5 → Phase 5.5
```

Phase 5 / 5.5 是 tier 无关的（按 region + version 过滤所有 pending match），在第一个 tier 循环时处理完全部工作，后续 tier 的 `IsDone` 立刻返回 true 跳过，不产生重复 API 调用。

---

## 5. 各阶段详细设计

### Phase 0 · 版本同步

- **数据源：** CommunityDragon（非 Riot API，无限速）
- **产出：** `game_versions`；将 `state.Profile.Version` 设为 latest version
- **幂等：** `ON CONFLICT (version) DO NOTHING`

---

### Phase 1 · 段位数据同步

遍历 `rank_prefetch_tiers`，写入 `player_rank_snapshots`（`source='phase1'`）和 `players`（含 `region`）。

| API | 用途 |
|-----|------|
| `/challengerleagues/by-queue/{queue}` | 王者 |
| `/grandmasterleagues/by-queue/{queue}` | 宗师 |
| `/masterleagues/by-queue/{queue}` | 大师 |
| `/entries/{queue}/{tier}/{division}` | 翡翠–钻石（含分页，每页最多 205 条）|

---

### Phase 2 · Match ID 采集

**时间窗口：** `game_versions.patch_start_at` → 下一版本 `patch_start_at`（历史版本）或 `run.started_at`（最新版本）。

**产出：** `UpsertMatchID` 同时写入 `match_id`、`region`、`version`（run 绑定版本）。

**Player 去重：** `player_match_sync.(puuid, region)` 记录同步时间，resume 时跳过本轮已处理的玩家。

---

### Phase 3 · 比赛详情抓取

- **过滤：** `fetch_status = 'pending' AND region = ? AND version = ?`，只处理本次 run 版本的 match
- **FK 保障：** 写 `match_participants` 前先批量 `UpsertPlayer`
- **段位推断：** 从 `player_rank_snapshots` 取时间最近的快照（按 region 过滤），无快照则置 null
- **重试：** `retry_count` 满 3 次才变 `error`，未满时保持 `pending` 自动重试

---

### Phase 3.5 · On-demand 段位补全

- **触发：** `tier_at_match IS NULL`（按 region join matches 过滤）
- **API：** `GET /lol/league/v4/entries/by-puuid/{puuid}`
- **批量写回：** `UpdateParticipantTierByPUUID` 一次 SQL 更新该 puuid 所有 NULL 行，`tier_snapshot_delta_h` 通过 JOIN matches 在 DB 侧计算
- **无段位玩家：** API 无数据时写入 `tier_at_match = 'UNRANKED'`，后续不再重复查询
- **终止：** 一批次内所有 puuid 均已查过（`newPUUIDs == 0`）则退出

---

### Phase 4 · 对局平均段位预计算

**Tier Score 编码：**

```
tier_score = tier_base + division_bonus + lp

tier_base:      IRON=0, BRONZE=400, SILVER=800, GOLD=1200,
                PLATINUM=1600, EMERALD=2000, DIAMOND=2400,
                MASTER/GRANDMASTER/CHALLENGER=2800

division_bonus: IV=0, III=100, II=200, I=300
```

**Apex 阈值（每次 Run 动态计算）：** 从本 run 的 `player_rank_snapshots`（`source='phase1'`）中查 Challenger / Grandmaster 玩家的最低 LP：

```
challenger_threshold  = 2800 + min(challenger LP in this run)
grandmaster_threshold = 2800 + min(grandmaster LP in this run)
master_threshold      = 2800
```

**写入字段：** `avg_tier_score`、`avg_tier`、`avg_division`、`tier_coverage`（仅统计有有效段位的参与者，排除 NULL 和 UNRANKED）。

**avg_division：** MASTER / GRANDMASTER / CHALLENGER 均为 `"I"`；其他段位为 `IV/III/II/I`。

---

### Phase 5 · Timeline 抓取

- **API：** `GET /lol/match/v5/matches/{matchId}/timeline`
- **过滤：** `fetch_status = 'done' AND timeline_status = 'pending' AND region = ? AND version = ?`
- **重试：** `timeline_retry_count` 满 3 次才变 `error`，未满时保持 `pending`

**提取内容：**

| 表 | 数据 |
|---|---|
| `match_item_events` | 所有购买事件；`removal_type`: `''`=持有 / `'undo'`=退款 / `'sold'`=售出 |
| `match_skill_events` | 技能加点序列；`skill_slot` 1=Q/2=W/3=E/4=R；`level_up_type` 含 EVOLVE |
| `match_participant_snapshots` | 每 5 分钟一个快照（5/10/15/...直到比赛结束），含金币、CS、刷野、等级、XP、位置、插眼数、累计伤害 |

**championStats 不解析**，节省约 70% 的 timeline JSON 解析开销。

---

### Phase 5.5 · 物品分类

- **依赖：** `timeline_status = 'done'` AND `items_status = 'pending'`
- **重试：** `items_retry_count` 满 3 次才变 `error`
- **Item Catalog：** 从 CommunityDragon 拉取（URL: `https://raw.communitydragon.org/{patch}/plugins/rcp-be-lol-game-data/global/default/v1/items.json`），每个 patch（如 `"15.1"`）只拉取一次，缓存在 `item_catalog` 表

**大件判定（`is_completed`）：** `to=[]`（终端物品）AND `from≠[]`（有合成路径）AND `price_total >= 1000` AND `inStore=true` AND 非消耗品/饰品

**出门装判定：** `timestamp_ms < 90000` AND `removal_type != 'undo'` AND `item_id != 3340`（黄色假眼）AND 累计 `price_total ≤ 500` per participant

**鞋子判定：** `is_boots=true` AND `removal_type != 'undo'`，取购买时间**最晚**的那双

**写入表：**

| 表 | 内容 |
|---|---|
| `match_completed_items` | slots 1-6 的大件，含 `is_boots` 标记 |
| `match_starter_items` | 出门装（可以是消耗品，不管后来是否售出） |
| `match_boots` | 每个参与者最终选定的鞋子（每人一行） |

---

## 6. 数据库设计

### `runs`

```sql
id, status, profile, mode, region, target_tiers, rank_prefetch_tiers,
queue, execution, version, current_phase, current_tier,
started_at, ended_at, last_run_end, created_at
```

### `game_versions`

```sql
id, version, fetched_at, patch_start_at, is_latest
```

### `players`

```sql
puuid (PK), region, game_name, tag_line, created_at, updated_at
```

### `player_rank_snapshots`

```sql
id, run_id, puuid, region, source,  -- phase1 / on_demand
league_id, queue, tier, division, league_points,
wins, losses, veteran, inactive, fresh_blood, hot_streak,
rank_status, created_at
```

### `player_match_sync`

```sql
puuid, region  -- 复合主键
last_synced_at
```

### `matches`

```sql
match_id (PK), region, version,         -- phase2 写入
data_version, platform_id, queue_id, game_version,
game_mode, game_type, game_start_ts, game_end_ts,
game_duration, end_of_game_result,
fetch_status, retry_count,              -- phase3 管理
timeline_status, timeline_retry_count,  -- phase5 管理
items_status, items_retry_count,        -- phase5.5 管理
avg_tier_score, avg_tier, avg_division, tier_coverage,  -- phase4 写入
created_at
```

### `match_participants`

关键字段（与 Riot DTO 一一对应，约 100 列）：

```sql
id, match_id, puuid, participant_id,
tier_at_match,          -- phase3 推断；'UNRANKED' = 无段位哨兵值
division_at_match, lp_at_match, tier_snapshot_delta_h,
team_id, team_position, win, champion_id, champion_name,
-- KDA、伤害、经济、视野、建筑、Ping 等...
```

### `match_perks` / `match_bans` / `match_teams`

结构与 Riot API DTO 对应，主键含 `match_id`。

### `match_timelines`（遗留）

Migration 001 里建立的旧表，固定槽位设计（`boots_item_id`, `first_item_id` ...），已被 Phase 5/5.5 的新表体系取代，当前无任何代码写入，可在未来 migration 中删除。

### Timeline 相关表

```sql
-- Phase 5 写入
match_item_events    (match_id, participant_id, timestamp_ms, item_id, removal_type)
match_skill_events   (match_id, participant_id, timestamp_ms, skill_slot, level_up_type)
match_participant_snapshots (match_id, participant_id, minute PK,
    total_gold, current_gold, cs, jungle_cs, level, xp,
    pos_x, pos_y, time_enemy_cc, wards_placed, wards_killed,
    dmg_total, dmg_to_champs, dmg_magic_champs, dmg_phys_champs,
    dmg_true_champs, dmg_taken)

-- Phase 5.5 写入
item_catalog         (item_id, patch PK, name, price_total,
    is_completed, is_boots, is_skippable, from_ids, to_ids)
match_completed_items (match_id, participant_id, slot PK,
    item_id, timestamp_ms, is_boots)
match_starter_items  (match_id, participant_id, item_id, timestamp_ms)
match_boots          (match_id, participant_id PK, item_id, timestamp_ms)
```

---

## 7. 代码架构

### 包结构

```
gogg/
├── cmd/
│   ├── crawl/
│   │   ├── root.go       # loadDeps、newRiotClientForRegion、buildRunner
│   │   ├── run.go        # gogg crawl run（含 --region 等 flags）
│   │   ├── daemon.go     # gogg crawl daemon（多 profile 并行）
│   │   ├── cancel.go     # gogg crawl cancel <id>
│   │   ├── runs_cmd.go   # gogg crawl runs
│   │   └── status.go     # gogg crawl status
│   └── inttest-cleanup/
│       └── main.go       # 清理集成测试 schema
└── internal/
    ├── config/
    │   ├── config.go     # Config、RegionConfig、resolvedRegions、RegionByName
    │   └── profile.go    # RunProfile、Validate、MergeFlags
    ├── crawler/
    │   ├── runner.go     # Advisory Lock（per-region）、graceful shutdown
    │   ├── strategy.go   # PipelineStrategy / SequentialStrategy（runPhase 统一保存 checkpoint）
    │   ├── phase.go      # Phase 接口
    │   ├── run_state.go  # RunState（含 donePhases、Region()）
    │   ├── phase0/       # 版本同步
    │   ├── phase1/       # 段位快照
    │   ├── phase2/       # Match ID 采集
    │   ├── phase3/       # 比赛详情（含 phase_test.go、phase_e2e_test.go）
    │   ├── phase35/      # On-demand 段位补全
    │   ├── phase4/       # avg_tier 预计算
    │   ├── phase5/       # Timeline 抓取与解析
    │   ├── phase55/      # 物品分类（大件/出门装/鞋子）
    │   └── pipeline_inttest_test.go  # 全流程集成测试
    ├── riotapi/
    │   ├── client.go     # HTTP 客户端、双窗口限速
    │   ├── cdragon.go    # CommunityDragon item catalog 拉取与分类
    │   └── ...           # league、match、account、version、timeline endpoint
    └── storage/
        ├── schema.go     # golang-migrate InitSchema（每次启动自动应用 pending migrations）
        ├── runs.go       # Run CRUD、GetActiveRun(region)、GetLastCompletedRunEnd
        ├── matches.go    # UpsertMatchID(region,version)、GetPendingMatchIDs(region,version)、GetApexThresholds
        ├── snapshots.go  # InsertSnapshot、GetClosestSnapshot(region)
        ├── players.go    # UpsertPlayer(region)、GetPlayerSyncTime(region)
        ├── participants.go # InsertParticipants、UpdateParticipantTierByPUUID、MarkParticipantUnranked
        ├── versions.go   # GetVersionBoundaries
        ├── timeline.go   # ItemEvent、SkillEvent、ParticipantSnapshot CRUD
        ├── items.go      # ItemSets、CompletedItem、StarterItem、BootsItem CRUD
        └── testutil/
            └── testutil.go  # NewTestStore / NewPersistentStore
```

### 关键机制

**Graceful Shutdown**

```
runErr == nil              → CompleteRun()（status=completed）
errors.Is(err, Canceled)   → 不写（status 保持 running，可 resume）
其他错误                   → FailRun()（status=failed，可 resume）
```

**Match 重试（fetch/timeline/items 统一策略）**

```
retry_count 0→1、1→2：status 保持 pending（自动重试）
retry_count 2→3：        status = error（不再重试）
```

**无段位哨兵值**

Phase 3.5 查不到段位时写 `tier_at_match = 'UNRANKED'`（非 NULL），后续 `GetParticipantsMissingTier` 过滤 `IS NULL`，不再重复查询。Phase 4 的 avg_tier 计算排除 UNRANKED。

---

## 8. 测试

### 单元测试

```bash
go test ./internal/crawler/phase3/
```

### E2E 测试（真实 Riot API）

```bash
RIOT_API_KEY=<key> go test -tags e2e -v ./internal/crawler/phase3/ -run TestRun_realMatch
```

### 全流程集成测试

```bash
RIOT_API_KEY=<key> go test -tags integration -v -timeout 20m \
  ./internal/crawler/ -run TestFullPipelineReduced
```

Phase1 保留每段位 3 个玩家，Phase2 正常运行后裁剪至 9 场，Phase3–5.5 正常执行。Schema `crawl_inttest` 不自动清理：

```bash
go run ./cmd/inttest-cleanup/
```

---

## 9. 待定事项

- [ ] Timeline 出门装时间阈值（90s）是否需要根据 champion/role 差异化
- [ ] `match_teams.feats` 字段结构待确认是否展开为独立列
- [ ] 低段位（翡翠–铂金）是否纳入采集范围
