# 知识、文件与存储布局

状态：长期目标设计。部分仓库布局和离线知识查询已进入实施，外部状态/制品服务尚未
落地；以[研发入口](../README.md)为当前状态。入口：[总览](README.md)。

## 1. 一个权威源，多个可重建视图

| 类别 | 权威记录 | 可派生内容 |
|---|---|---|
| 工程规则、产品目标、知识与决策 | Git 指导、契约、带 ID 的知识、ADR | 上下文、全文/向量/关系索引 |
| 流程进度 | 持久工作流 checkpoint | 任务看板、阶段摘要 |
| 认领与动作事实 | 执行账本的事务记录和追加事件 | 审计视图、运行统计 |
| 候选与实验结果 | 不可变对象与 manifest | 报告、截图索引、评分聚合 |
| 私人偏好 | 用户授权的原生 Memories | 有限的当前会话提示 |
| 业务历史与玩家数据 | 网站数据与模型系统 | Agent 获授权后的受限样本引用 |

Git 保存为什么改变，checkpoint 保存流程到了哪里，账本保存动作事实，制品
保存当时的输入和结果。禁止让索引、聊天摘要或两份 Markdown 同时拥有执行状态。

## 2. 目标仓库布局

用 `engineering/` 归集长期研发资产，`.codex/` 和 `.agents/` 保持原生入口。
产品代码布局由后续业务架构决定；旧 `apps/`、`packages/` 现归档在 `legecy/`。

```text
AGENTS.md                             最小启动指导与路由
.codex/
  config.toml                         Codex 配置唯一版本化来源
  agents/                             原生角色；不另复制竞争性定义
  hooks.json                          经验证的生命周期桥接（如采用）
.agents/skills/<skill-id>/             评测后的流程、脚本和必要引用
engineering/
  README.md                           人与 Agent 的统一导航
  charter/
    product.md                        产品目标、区域与保留能力
    quality.md                        验收原则、测量口径、质量优先策略
  decisions/                          Agent/知识系统决策，不重复产品 ADR
  knowledge/
    index.yaml                        从元数据生成的轻量目录
  facts/<domain>.md                 规模化目标；引导阶段先用 facts.json
    incidents/<id>.md                 失败条件、原因、修复和回归证据
    environment/<id>.md               可分享的环境约定，无主机私密值
  research/
    sources.yaml                      原始来源、版本、日期、核实状态
    questions/<id>.md                 未决问题、假设与实验引用
  tasks/<task-id>/
    contract.yaml                     需求、验收、输入和依赖
    outcome.md                        收尾摘要，不存第二份实时进度
  evals/
    cases/                            可见回归任务和输入定义
    protocols/                        评分、消融、隔离和划分协议
    reports/                          结果摘要与制品引用
  schemas/                            知识、契约、事件、manifest 结构
  releases/<release-id>.yaml           被接受的模型/规则/Skill/工具组合
tools/agent-system/
  adapters/                           Codex 等执行接口
  workflows/                          持久图和 GOGG 领域关卡
  execution/                          资源认领、运行账本和受控工具桥
  knowledge/                          索引、校验和上下文组装
  maintenance/                        备份、恢复验证与清理计划
engineering/decisions/                 当前接受决策；旧产品 ADR 留在 legecy 作参考
```

知识的原子性是能独立验证、失效和回退，不强制每条事实单独一个文件。
同领域条目可共存，文件过长或经常发生独立修改冲突时再拆。稳定 ID 不依赖源码
路径，引用改名只更新证据定位。标签与索引从唯一元数据生成。

AGENTS 只含必须遵守的行为、权威顺序、入口和关键命令。Skill 是操作流程的
权威源，知识仅引用其版本。主机的真实目录和身份放 Git 外配置，仓库提供无敏感
值模板。学习者不能自行覆盖这些强制规则。

## 3. 单机物理存放

以下为 Linux/WSL 主机建议映射，本轮不创建这些路径：

| 逻辑根 | 建议位置或配置方式 | 内容 |
|---|---|---|
| 代码/知识 | 当前仓库及专用 worktree | 可审查的版本化文件 |
| 持久控制状态 | `/home/zrt/.local/state/gogg-agent/` | 主机配置、恢复记录、试点本地库；生产库由明确 PostgreSQL 存储配置管理 |
| 可丢缓存 | `/home/zrt/.cache/gogg-agent/` | 检索、解析、下载暂存 |
| 活跃工作区 | 显式 `GOGG_AGENT_WORK_ROOT` | 隔离任务、构建与运行目录 |
| 不可变制品 | 显式 `GOGG_AGENT_ARTIFACT_ROOT` | 快照、日志、浏览器与实验原始结果 |
| 独立备份 | 显式跨故障域备份目标 | 一致性状态备份、知识、被固定的制品 |
| Codex 私有状态 | Codex 自己的状态目录 | 原生会话、认证与个人记忆，由原生机制管理 |

不将 `HOME`、`CODEX_HOME` 重定义为自建系统数据根。控制状态不依附 worktree
生命周期。活跃工作区含未归档修改时不可丢，不能按缓存清理。

大型制品可放专用 ext4/NVMe 或已核验数据盘的独立子目录。项目记录了
`/mnt/gogg-db` 数据盘；本轮仅观察到该路径在本会话命名空间中为 ext4、只读。
这不证明主机文件系统损坏，也不构成迁移许可。未来使用前核实挂载身份、可写性、
容量和数据库 I/O 竞争，配置专用子目录。挂载不符时停止写入，不能静默回落到
系统盘形成两份数据。

## 4. 控制状态与制品结构

```text
STATE_ROOT/repos/<repo-id>/
  pilot/workflow.sqlite               仅本地试点的框架 checkpoint
  pilot/execution.sqlite              仅本地试点的动作/租约账本
  host.json                           私有路径与宿主能力
  recovery/                           恢复计划与最近演练
CACHE_ROOT/<repo-id>/<index-version>/
  fulltext.sqlite                     可重建 FTS
  semantic/                           可选向量索引及模型版本
  symbols/                            语法/符号缓存
ARTIFACT_ROOT/
  objects/sha256/<prefix>/<digest>     完成后不可变的内容对象
  manifests/sha256/<digest>.json       不可变关系、来源、完整性与访问策略
  staging/<run-id>/                    未完成写入，不是有效证据
  quarantine/<gc-plan-id>/             可回收对象的延迟删除区
```

长期无人值守优先使用 PostgreSQL 持久后端，框架 checkpoint 与工程执行账本按
独立逻辑 schema/账号管理，不写网站业务表；不要求控制库文件位于 STATE_ROOT。
FTS 仍可用可重建的本地 SQLite。生产后端、权限和备份的采用需通过试点，不能仅
凭数据库名字推断可靠性或资源成本。

流程阶段只由 workflow 库拥有；execution 库拥有认领、进程、操作意图/结果与
资源代次，追加事件和对应投影同事务更新。JSONL 导出只作备份/可读视图。
框架表的 schema 由框架管理，不自行改内部表结构。两个库之间通过稳定 ID、
幂等动作和恢复核对协调，不假设跨库原子事务。

对象先写 staging，完成后核对哈希并发布；manifest 需要的对象持久化成功后才
能标为完整，跨文件系统不假设 rename 原子。gate 只引用完整 manifest。
崩溃后检查缺失/已存在对象和操作账本，不能保留悬空的通过状态。
每个候选、评测和修订产生新的 manifest digest；gate 固定 digest，不以可变
“最新结果”作依据。run ID 是关联键，不是可覆盖的唯一制品版本。

记录原始内容哈希和存储对象哈希，压缩格式升级不改变原始证据身份。逻辑对象
引用独立于物理路径。路径不由不可信文本直接拼接。

manifest/object 必含数据分类、允许读取的主体及保留策略；截图、HAR/trace、
请求响应、数据库样本和日志均在不可变发布前检查并脱敏。摘要、索引、导出及
来源追溯继承源对象限制，知道 digest 不代表获准读取。受限原件如确有保存授权，
存独立权限域；隐藏评测原件与普通工程证据分离。学习器只能追溯其授权证据包，
不能顺着引用绕过权限读取原件。去重跨权限域时仍逐次鉴权，不能泄露对象存在性。

## 5. 知识条目契约

```yaml
id: stable-domain-fact-id
schema_version: 1
kind: fact # decision | incident | procedure-reference | environment | hypothesis
status: candidate # verified | stale | superseded | rejected
scope: [repo-id, domain]
claim: 一项可核验主张
sources: [文件符号引用、证据manifest或原始来源ID]
observed_at: null
valid_from: null
valid_to: null
verified_revision: null # commit 或含未提交文件的候选快照
last_checked_at: null
revalidate_on: [dependency-change]
valid_until: null
depends_on: [] # 文件/符号/契约/外部来源的版本与哈希
supersedes: []
rejection_reason: null
owner: responsible-role
sensitivity: project-shareable
```

`valid_from/to` 表示主张何时成立，`observed_at/last_checked_at` 表示何时获知或
核实；代码适用性仍需内容/接口依赖。日期本身不能证明分支兼容。
`verified` 需证据与范围。外部技术事实可由官方来源核实；本项目运行行为需要
代码、测试或运行证据。决策需要责任者接受记录。

冲突显示双方并阻止默认晋升，不能仅以更新日期或模型信心覆盖。实现存在不证明
满足用户要求，测试名称存在不证明执行通过，记忆重复一句话不算独立证据。
否决条目保留反证，避免反复提出相同错误。

## 6. 索引、快照与失效

读取顺序：当前用户请求与强制指导 → 核对任务契约 → 模块路由 → 精确路径、
符号、FTS → 必要时语义召回 → 必要时关系遍历 → 原始证据。
版本/状态过滤先于相关性排序。模型生成摘要保留适用条件、未知项和源指针。

索引身份含 repo ID、commit、dirty digest、文档哈希、schema/解析器版本；向量
索引另含 embedding 模型、切块及归一化版本。另一 worktree 的最新索引不得
自动用于本任务。各角色及 gate 固定 `release_id + knowledge_snapshot_id`；知识
快照包含条目内容、状态、来源版本和检索配置。候选变更由维护者串行整合；
用户当前纠正即时优先，必要时显式更新任务及知识快照引用。

变化事件只重建受影响索引。重复 ID、缺失证据、失效路径、冲突替代关系和过期
环境事实做机械校验。FTS/向量/图均可重建；重建不触碰 workflow/execution 库。
是否加语义或图索引由真实问题集效果决定。

知识遗忘通常是不再默认召回、标记过时/替代；历史保持可查。低频高代价教训
不能仅按访问次数清理。个人删除要求需要处理派生缓存和备份政策，共享 Git
知识从一开始不写原始玩家资料、私人聊天与凭据。

## 7. 维护与保留

| 时机 | 机械维护 | Agent 判断 |
|---|---|---|
| 任务开始 | 版本、候选、索引和资源自检 | 选择知识、暴露冲突 |
| 候选冻结 | 输入快照、manifest、gate 版本 | 核查覆盖原始目标 |
| 阶段结束 | checkpoint、增量索引、备份触发 | 提出少量知识/技能候选 |
| 空闲窗口 | 链接/哈希/容量/备份检查 | 审核失效知识与重复失败 |
| 模型或工具升级 | 固定版本、能力探测、回放 | 决定是否晋升 |
| 定期或重大变更后 | 恢复演练、全索引重建 | 调整维护和保留政策 |

维护作业入队，不抢占性能测量。当前没有自动删除政策。实施时先测容量，再为
普通调试轨迹、成功运行、失败复现分别定义窗口；已被知识、发布、回归或用户
指定引用的证据固定保留，活跃任务及依赖对象全部 pin。业务数据集不在 Agent GC
范围内，不能把可重建索引的清理规则用于它们。

GC 遍历 manifest、任务、发布及知识引用，生成 dry-run 清单，协调并发新引用，
确认未被 pin 才放入延迟回收区；真正删除按已授权政策执行。哈希去重不赋予删除
权限。未完成写入在恢复核对后才可判断为垃圾。
仍有效的备份恢复点也是 GC 根，闭合 manifest 固定其全部依赖。只有确认对象已
独立备份且恢复可用，或恢复点按授权政策过期，才可解除原存储的相应保护。
工作流 checkpoint 使用框架支持的保留/删除接口，活跃或可恢复任务必须保留所需
历史；不能把制品 GC 直接应用于框架内部表。

## 8. 备份、恢复与多机

Git 备份保存已提交知识，未提交任务变更要有独立候选快照。试点 SQLite 使用
一致性备份，不能运行中仅复制主文件而忽略 WAL；PostgreSQL 使用其支持的一致性
备份/恢复机制。备份记录两类存储的逻辑关联、事件水位、闭合制品集合、知识
快照及可取得的工作流/适配器运行时版本；恢复允许重放核对不同时刻落盘的记录。

恢复顺序：仓库与已发布组合 → workflow/execution 与对象 → 哈希和水位核对 →
生成新 controller incarnation、拒绝旧 token 并隔离旧写者 → 核对外部动作 →
恢复工作区 → 重建索引 → 运行验收。共享可写资源未隔离旧进程前不重新派发。
测试至少包含一个未完成任务和一份有未提交文件的候选。备份成功不等于恢复成功。

多机时一个控制服务拥有流程与账本，执行者各有工作区/缓存，通过内容寻址制品
交换，知识按明确 revision 分发。不要共享网络目录中的 SQLite/WAL 或工作区。
高可用阶段另外验证控制器选主、epoch/fencing、故障转移和恢复点，不能仅增加
多个控制器实例。逻辑 ID 与契约保持稳定。
