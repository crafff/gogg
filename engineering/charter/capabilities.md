# 业务能力盘点基线

Baseline ID：`capabilities-bootstrap-20260905-v1`。
状态：`inventoried / runtime_unverified`，不是穷尽清单，也不是新系统验收通过。
来源：保存前工作区（含未提交 TFT 修改）、路由、GraphQL schema 和相关实现。
旧 URL、字段、框架及存储仅定位证据，不构成新系统的技术兼容要求。

每次发布逐 ID 给出保留/替换证据；遗漏阻断整体能力验收，删减或延期需明确决定。
新发现能力通过基线修订加入，不能在重建途中默默丢失。

| ID | 已观察到的业务能力 | 归档证据入口 |
|---|---|---|
| CAP-UI-LOCALE | zh-CN/en-US、持久语言选择、实体名称正确更新 | [语言入口](../../legecy/apps/web/src/shared/i18n/index.ts) |
| CAP-AUTH-ACCOUNT | Google 登录、身份恢复、账户资料/连接身份、退出及失败提示 | [账户页](../../legecy/apps/web/src/features/user-profile/MePage.tsx) |
| CAP-UI-ADDRESSABLE-STATE | 玩家/英雄直达，榜单和分析筛选可通过地址恢复 | [路由](../../legecy/apps/web/src/app/router.tsx) |
| CAP-ASSET-VERSIONED | 本地版本化游戏资源、历史版本展示和缺失处理 | [资源服务](../../legecy/apps/api/internal/transport/rest/assets.go) |
| CAP-LOL-CATALOG | 已发布统计的地区/版本目录，原始历史与统计目录分开 | [目录契约](../../legecy/apps/api/internal/transport/graphql/schema/catalog.graphql) |
| CAP-LOL-RANKINGS | 地区/版本/队列/段位/位置/样本筛选，英雄表现和样本指标 | [榜单契约](../../legecy/apps/api/internal/transport/graphql/schema/rankings.graphql) |
| CAP-LOL-BUILDS | 英雄符文/属性碎片、技能、阶段装备及样本/选择/胜率 | [详情契约](../../legecy/apps/api/internal/transport/graphql/schema/champion.graphql) |
| CAP-LOL-WIN-FACTORS | 有版本/群体/样本门槛的10/15分钟观察指标；现为打野且非因果 | [因素实现](../../legecy/apps/api/internal/service/championinsights/service.go) |
| CAP-LOL-PLAYER-PROFILE | KR/NA1 Riot ID、头像/等级/更新时间/陈旧度/段位资料 | [玩家契约](../../legecy/apps/api/internal/transport/graphql/schema/summoner.graphql) |
| CAP-LOL-MATCH-HISTORY | 400/420/440/480及ALL联集，对局分页与战斗数据 | [历史契约](../../legecy/apps/api/internal/transport/graphql/schema/summoner.graphql) |
| CAP-LOL-MATCH-PARTICIPANTS | 十名玩家展开、装备符文、缺ID替代、近似段位与覆盖 | [历史映射测试](../../legecy/apps/api/internal/service/summoner/service_test.go) |
| CAP-LOL-REFRESH-JOB | 刷新进度、重复复用、重载恢复、限流/终态/部分成功明确 | [刷新服务](../../legecy/apps/api/internal/service/summoner/service.go) |
| CAP-LOL-RECENT-SEARCH | 最近五个地区+Riot ID、重开/清空及地区记忆 | [搜索表单](../../legecy/apps/web/src/features/summoner/components/SummonerSearchForm.tsx) |
| CAP-TFT-PUBLISHED-LINEUPS | 正式目录筛选、阵容/名次/吃鸡/前四/竞争及覆盖 | [TFT 契约](../../legecy/apps/api/internal/transport/graphql/schema/tft.graphql) |
| CAP-TFT-OBSERVED-PREVIEW | 完成批次终局阵容预览，来源明确，未知补丁不伪装 | [预览页面测试](../../legecy/apps/web/src/features/tft/TftAnalysisPage.test.tsx) |
| CAP-TFT-LINEUP-DETAILS | 常用装备、投入、单位星级及完整星级组合，低样本隐藏 | [阵容详情](../../legecy/apps/web/src/features/tft/TftAnalysisPage.tsx) |
| CAP-TFT-FACETS-SIGNALS | 当前实体筛选、详情地址、装备/三星/强化共现及支持度 | [展示逻辑](../../legecy/apps/api/internal/service/tft/presentation.go) |
| CAP-TFT-TEAM-PLANNER | 权威映射完整时生成精确阵容代码，复制和不可用状态明确 | [编码逻辑](../../legecy/apps/api/internal/service/tft/presentation.go) |
| CAP-TFT-PLAYER-HISTORY | 十五平台，ALL/1100/1090/1160/1130，参与者与棋盘分页 | [玩家页面](../../legecy/apps/web/src/features/tft/TftPlayerPage.tsx) |
| CAP-TFT-REFRESH-JOB | 玩家刷新/重连/进度/部分结果，按需数据不污染正式群体 | [历史服务](../../legecy/apps/api/internal/service/tft/history.go) |
| CAP-DATA-REGIONAL-COLLECTION | LoL/TFT区域采集独立控制、共享上游额度、限流/凭据故障 | [TFT 配置](../../legecy/apps/worker/internal/tft/config/config.go) |
| CAP-DATA-RUN-CONTROL | 触发、暂停、恢复、取消、查进度及持久历史结果 | [采集控制](../../legecy/apps/worker/cmd/crawlctl/main.go) |
| CAP-DATA-COHORT-ADMISSION | 明确群体采样、区域目标/不足、到达顺序与缓存不任意偏置 | [采集工作流](../../legecy/apps/worker/internal/tft/workflow/crawl.go) |
| CAP-DATA-RAW-ARCHIVE-TRANSFER | 原始响应校验与重解析；LoL增量包导出/重建/导入 | [包处理](../../legecy/apps/worker/cmd/crawler-lite/bundle.go) |
| CAP-DATA-PUBLISH-REFRESH | 统计/资源独立重建，来源/算法/覆盖/版本，失败保护旧发布 | [静态发布](../../legecy/apps/worker/internal/tft/staticdata/sync.go) |

## 不能遗漏的离线研发资产

[英雄推荐实验](../../legecy/experiments/champion-recommender/README.md)与
[雷克塞预测/SHAP实验](../../legecy/experiments/reksai-win-shap/README.md)保存为研究
参考。没有复跑其报告；外置数据/模型只保存原链接，不声称已在新系统可用。
个性化教练、训练计划和提升效果仍是未来产品目标，不能由实验脚本存在推断上线。

## 待补查与实际验收

所有条目需要固定数据/状态与用户流程验收，尤其并发重复、刷新/服务重启、终态
404、限流、部分失败、旧版本资源、缺字段、分页与样本分母。正式TFT发布和预览
分开验证，代码提供两条路径不证明数据库已有两类结果。

维护/修复命令、健康检查/监控、部署自动化、全部错误状态、移动端和无障碍尚未
完整盘点。新架构开发前补齐适用项，保留明确未知，不能以这25条宣称全面覆盖。
