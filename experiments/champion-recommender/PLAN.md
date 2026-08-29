# 个性化英雄推荐系统详细计划

## 1. 产品问题与输出

### 1.1 推荐请求

一次推荐至少包含：

- 玩家 `puuid`；
- 目标位置（TOP/JUNGLE/MIDDLE/BOTTOM/UTILITY）；
- 地区、队列、当前完整版本；
- 可选偏好：想练前期节奏/团战/刷野/操作简单、是否接受新英雄。

不应在未指定位置时把不同位置英雄放在同一榜单；可以先推荐位置，再推荐该位置英雄。

### 1.2 推荐响应

每个英雄返回：

- `fit_score`：与玩家行为画像的适配；
- `patch_score`：该版本、地区、分段和位置的收缩后强度；
- `confidence`：玩家数据量与英雄版本样本量共同决定；
- `novelty`：与现有英雄池的距离；
- `final_score` 与排名；
- 2–3条可审计理由，例如“你使用前期节奏型打野时表现高于自身基线”“该英雄在16.14大师以上打野强度稳定”；
- 风险提示，例如“你没有同类型英雄样本，属于探索推荐”。

严禁生成模型没有证据支持的自然语言理由。

## 2. 最容易犯的建模错误

1. **把选择当适合。** 玩家经常玩某英雄只说明曝光和习惯，不代表它最适合。
2. **把胜率直接当标签。** 5场英雄100%胜率可能不如100场稳定55%；对局双方水平也不同。
3. **随机切分未来数据。** 玩家画像和版本强度必须只使用推荐时点之前的数据。
4. **硬聚类决定推荐。** 同一类玩家内部仍有巨大差异，硬分类会损失个性化。
5. **版本强势覆盖个性化。** 如果最终只是版本榜单乘一个很小的画像权重，系统没有推荐价值。
6. **把未选择当负反馈。** 玩家没有玩过英雄通常是“未知”，不能直接标成不喜欢。
7. **离线猜中率等于产品效果。** 猜中下一次选择会奖励重复推荐，必须增加新英雄采纳和持续使用指标。

## 3. 数据设计

### 3.1 已有数据可用部分

GOGG 已有：

- `matches`：版本、地区、队列、时间、时长；
- `match_participants`：英雄、位置、胜负、KDA、经济、伤害、视野、资源、装备、召唤师技能；
- `match_participant_snapshots`：5/10/15等分钟的经济、补刀、等级、伤害、位置和视野；
- `match_item_events`、`match_skill_events`：购买与加点；
- `player_rank_snapshots`：段位及时间；
- champion/version/position 聚合：当前版本强度。

现阶段缺少推荐曝光、点击、显式“不喜欢”、推荐后尝试等反馈。上线前必须补埋点，否则只能优化历史选择而不能优化推荐价值。

### 3.2 玩家画像快照

新增实验宽表（先放实验产物，验证后再迁移线上表）：

```text
player_profile_snapshot
  puuid, as_of_ts, region, queue_id, target_position
  games_7d/30d/90d, active_days, rank_score
  champion_pool_size, role_entropy, champion_entropy
  early_farm_pct, mid_farm_pct, lane_cs_pct
  early_damage_pct, mid_damage_pct, damage_taken_pct
  vision_pct, cc_pct, objective_pct
  kill_participation_pct, death_rate_pct
  aggression_score, farming_score, utility_score
  tempo_score, risk_score, mechanics_proxy
  preferred_damage_mix, preferred_range/archetype
  feature_confidence_0_1
```

所有连续比赛指标先在 `version × region × tier_group × position × champion` 内标准化，再聚合到玩家，防止玩不同英雄造成画像偏差。使用指数时间衰减，例如30天半衰期；同时保存7/30/90天窗口以区分当前状态与长期风格。

画像必须使用 `as_of_ts` 之前的对局生成，训练样本通过 point-in-time join 取历史画像。

### 3.3 英雄画像

```text
champion_profile_snapshot
  champion_id, version, region, tier_group, position
  sample_size, bayesian_win_rate, pick_rate, ban_rate
  early/mid farm, damage, cc, vision, objective distributions
  damage_mix, range, resource_type
  archetype probabilities: gank/farm/tank/carry/utility/assassin/engage
  complexity_proxy, early_power, scaling_proxy
  profile_confidence_0_1
```

英雄属性来自 CDragon 静态数据，玩法画像来自真实对局分布。版本胜率采用 Beta-Binomial 或层级贝叶斯收缩：小样本向该英雄跨版本、同位置总体均值回缩，不能直接使用裸胜率。

### 3.4 玩家—英雄交互

每个推荐时点建立：

```text
player_champion_interaction
  puuid, champion_id, position, as_of_ts
  games_7d/30d/90d, recency_days
  normalized_performance, expected_win_residual
  mastery_proxy, consistency, recent_trend
  explicit_like/dislike, recommendation_impressions
  recommendation_clicks, post_recommend_games
```

隐式反馈置信度建议从以下组合开始，并通过验证调整：

```text
preference = log1p(decayed_games)
           + 0.5 * positive_performance_residual
           + 0.3 * post_recommend_repeat_play
```

胜负残差应基于对局前可知的段位、阵容、位置和版本估计预期胜率，再用实际结果减预期，避免把低质量匹配全部算到英雄适配上。

## 4. 玩家分类

### 4.1 用途

玩家分类用于：

- 给画像起可理解的名称；
- 发现产品人群和冷启动规则；
- 监控不同人群的推荐效果；
- 生成受约束的解释。

不用于把整类玩家强制映射到同一英雄列表。

### 4.2 方法

1. 对标准化、置信度足够的画像做 PCA，保留解释性维度；
2. 先用 K-Means/Gaussian Mixture 建立可复现基线；
3. 用 silhouette、稳定性bootstrap、跨版本迁移率选择簇数，而不是凭肉眼；
4. 若数据明显非球形，再比较 HDBSCAN；
5. 为玩家保存 soft membership，而非只有一个硬标签。

候选标签可能包括“前期交战型”“稳定发育型”“资源控制型”“开团承伤型”“高风险刺客型”，最终名称必须来自各簇真实特征差异。

UMAP/t-SNE只用于可视化，不用于生产距离或聚类真值。

## 5. 推荐算法

### 5.1 必须建立的基线

按时间依次评估：

1. 当前版本同地区/分段/位置热门英雄；
2. 当前版本收缩胜率榜；
3. 玩家历史最高频英雄；
4. Item-KNN：玩过相似英雄的玩家还玩什么；
5. Implicit ALS/BPR：玩家×英雄隐式反馈矩阵。

`implicit` 提供适合隐式反馈的 ALS、BPR 和 Item-Item 模型，CPU多线程即可满足当前约170个英雄的规模。

### 5.2 MVP推荐：混合全量排序

英雄数量很少，直接为目标位置全部候选英雄构造 `(player, champion, context)` 特征并使用 `LightGBM LGBMRanker`：

- 玩家画像及置信度；
- 英雄画像和版本收缩强度；
- 玩家与英雄画像的距离、点积和逐维差；
- ALS用户/英雄embedding点积；
- 是否玩过、距上次使用、历史局数与表现残差；
- 与玩家现有英雄池的相似度/新颖度；
- 玩家簇与英雄的历史适配统计；
- 版本、地区、分段、位置上下文。

训练 group 是一次推荐时点/玩家，候选是该位置英雄，目标优化 NDCG@5。标签采用分级相关度：

- 0：未知或后续无正反馈；
- 1：推荐窗口后尝试1局；
- 2：尝试且表现不低于预期；
- 3：7/14天内重复使用且表现不低于预期。

在没有曝光日志的离线阶段，未玩英雄不能可靠标0。因此第一版训练采用正样本 + 基于可用英雄池的采样负例，并对结果做选择偏差说明；上线记录曝光后改用真实展示未采纳样本，并校正位置偏差。

### 5.3 冷启动

- 0场：用户选择位置和2–3个喜欢的英雄/风格，使用内容相似度 + 版本收缩强度；
- 1–5场：内容画像为主，CF低权重；
- 6–20场：混合模型，置信度随有效场次增长；
- 20场以上：完整个性化，但保留10%探索位；
- 新版本：英雄画像从上一版本迁移，版本强度高收缩，随样本增加更新。

### 5.4 重排约束

最终分数不要手工永久固定，初始可解释形式为：

```text
final = ranker_score
      + patch_strength_guardrail
      + confidence_adjustment
      + calibrated_exploration
      - redundancy_penalty
```

约束：目标位置可用、版本样本门槛、Top-5不过度同质、至少一个熟悉英雄和一个探索英雄、低置信结果明确标注。

不要把英雄胜率直接线性加到个性化分数；它应同时作为ranker特征和安全护栏，并经过样本收缩。

## 6. 开源工具选择

| 任务 | MVP选择 | 用途与取舍 |
|---|---|---|
| 特征SQL/快照 | PostgreSQL + SQL + pandas/Polars | 复用现有基础设施，先避免引入Feature Store |
| 标准化/聚类 | scikit-learn | PCA、KMeans、GMM、指标齐全 |
| 协同过滤 | `implicit` ALS/BPR | 快速隐式反馈基线、CPU足够 |
| 混合冷启动基线 | LightFM | 可联合用户/英雄metadata；作为对照，不一定上线 |
| 最终重排 | LightGBM `LGBMRanker` | LambdaRank、表格特征和解释工具成熟 |
| 统一算法benchmark | RecBole | 比较BPR、NeuMF、LightGCN等并统一NDCG/Recall评估 |
| 实验追踪 | MLflow | 参数、数据版本、指标、模型产物 |
| 数据质量 | Pandera 或 Great Expectations | 快照schema、范围、空值和漂移检查 |
| 调度 | 现有Temporal worker或批处理job | 日/版本画像与模型更新 |
| 在线缓存 | 现有Redis | 缓存Top-K和画像，不新增向量库 |
| 特征平台 | Feast（后期可选） | 需要多模型共享和严格在线/离线一致时再引入 |

RecBole适合研究比较，不建议直接嵌入Go线上服务。模型训练用Python离线完成，产出版本化模型/推荐表；Go API继续负责鉴权、业务约束和返回结果。

双塔/ANN只在候选规模远大于英雄集合、加入装备/符文/内容等百万级item后考虑。当前全量打分更简单、可解释且足够快。

## 7. 离线评估

### 7.1 严格时间回放

以每个完整版本为单位：

- 用 `T` 之前数据构建画像；
- 在 `T` 时点生成推荐；
- 用之后7/14天行为评估；
- 最终做“旧版本训练 → 新版本测试”；
- 同一场对局和未来信息绝不能进入画像。

### 7.2 指标

主指标：

- NDCG@5、Recall@5、HitRate@5、MRR；
- 新英雄 Recall@5：仅评估此前少于N场的后续采用；
- post-recommend normalized performance；
- 7/14天重复使用率。

护栏指标：

- catalog coverage、英雄曝光Gini、平均热门度；
- 推荐列表内相似度、novelty、serendipity；
- KR/NA1、段位、位置、活跃度、冷启动人群分组结果；
- 推荐置信度校准；
- 各版本性能与特征漂移。

必须同时超过“版本热门榜”“历史最高频”和“收缩胜率榜”，否则复杂模型不进入下一阶段。RecBole原生支持时间排序切分和NDCG、Recall、MRR、覆盖率、Gini等指标，可用于统一benchmark。

## 8. 在线实验与反馈闭环

### 8.1 埋点

新增：

```text
recommendation_request(id, puuid, context, model_version, created_at)
recommendation_impression(request_id, champion_id, rank, scores, reason_codes)
recommendation_action(request_id, champion_id, action_type, created_at)
```

`action_type` 至少包括 view、save、dismiss、queue_intent、played、played_again。真实对局通过玩家、英雄和时间窗口归因，不只依赖前端点击。

### 8.2 A/B指标

主指标不是CTR，而是：

- 推荐英雄7天内首次使用率；
- 首次使用后再次使用率；
- 标准化表现不劣于玩家自身基线；
- 玩家主动收藏/反馈；
- 负向指标：秒退、明确不喜欢、推荐集中度上升。

先做shadow mode，再做小流量A/B。探索位使用epsilon-greedy或Thompson Sampling；只有记录每次展示倾向概率后，才能做可靠的离线策略评估。

## 9. 服务架构

MVP推荐离线预计算：

```text
每日/版本批任务
  -> 生成point-in-time画像与交互
  -> 训练/验证
  -> 为活跃玩家×常用位置预计算Top-20
  -> 写入player_champion_recommendations
  -> Redis缓存

GraphQL/API
  -> 查询Top-K
  -> 应用可用性/版本护栏
  -> 返回reason_codes
  -> 记录impression
```

推荐表建议字段：`puuid, position, champion_id, rank, final_score, fit_score, patch_score, confidence, novelty, reason_codes, model_version, feature_as_of, expires_at`。

不在Go请求链路中实时调用Python模型。对刚完成对局的玩家，可异步刷新画像和推荐缓存。

## 10. 分阶段实施

### Stage 0：问题与数据审计（2–3天）

- 确定推荐位置、Top-K、成功定义和反馈窗口；
- 审计每位玩家历史长度、英雄覆盖、版本跨度；
- 量化冷启动比例与流行度偏差；
- 建立point-in-time数据契约。

验收：数据覆盖报告、无未来泄漏测试、三条简单基线可运行。

### Stage 1：画像与分群（3–5天）

- 生成玩家/英雄画像快照；
- 完成标准化、时间衰减和置信度；
- PCA + KMeans/GMM稳定性分析；
- 产出画像解释页面样例。

验收：画像跨重复运行稳定；每个画像字段可追溯到源数据；低样本置信度合理下降。

### Stage 2：推荐基线（3–5天）

- 热门、收缩胜率、历史频率、Item-KNN；
- `implicit` ALS/BPR；
- 时间回放评估与分组指标。

验收：结果可复现；明确CF是否超过简单基线；没有把未知英雄全部当负样本。

### Stage 3：混合Ranker（5–8天）

- 玩家×英雄候选特征；
- LightFM混合基线；
- LightGBM LambdaRank、置信度与多样性重排；
- reason code生成和逐项消融实验。

验收：NDCG@5、新英雄采用代理指标、coverage至少两项超过最佳简单基线；删除任一关键特征组的影响有记录。

### Stage 4：版本外验证（3–5天）

- 旧版本训练、新版本完整测试；
- KR/NA1、段位、位置、冷启动分层；
- 漂移、校准、失败案例和英雄曝光审计。

验收：主要人群不出现系统性退化；版本强度变化时仍保留个性化；形成go/no-go报告。

### Stage 5：产品闭环（5–10天）

- 推荐表、API、缓存、过期策略；
- impression/action埋点；
- shadow与小流量A/B；
- 监控模型版本、延迟、覆盖、反馈和漂移。

验收：可回滚；推荐理由可审计；线上采用/复玩指标定义与数据链路完整。

## 11. 首个实验建议

先限定为“为有至少20场历史、目标为JUNGLE的玩家推荐3个新英雄”：

- 候选排除最近30天已玩超过10场的主力英雄；
- 标签为未来14天首次采用并至少再玩1次；
- 使用16.13训练/验证，16.14测试；
- 比较版本热门、Item-KNN、ALS、LightFM、LambdaRank；
- Top-3必须包含一个与现有英雄池相似的低风险候选和一个风格相邻但更新颖的探索候选。

选择打野作为首个位置，是因为当前已经完成打野行为百分位实验，可以直接复用刷野、战斗、承伤、视野和节奏画像；验证成功后再扩展到其他位置。
