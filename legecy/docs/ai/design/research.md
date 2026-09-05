# 一手研究、开源项目与工具取舍

核实日期：2026-09-04。研究事实、作者工程经验与本项目设计判断分开陈述。
本轮未安装、训练、运行或压测下列项目；来源说明不构成 GOGG 效果证明。

## 1. 长期执行与复杂任务

| 来源 | 时间与已核实内容 | 对设计的影响及边界 |
|---|---|---|
| [Astra 官方指南](https://developers.openai.com/api/docs/guides/latest-model) | 当前指南：复杂工具工作、异步工具与中途引导；更敏感的指令遵循 | 重要角色统一 Astra；审计指导冲突，明确委派；API 能力不等于每个 Codex 入口都已支持 |
| [Anthropic 长期 harness](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents) | 2025-11-26：初始化清单、启动脚本、增量任务和交接 | 持久交接与功能验收；不是多 Agent 优势的严格对照实验 |
| [Anthropic 长应用开发](https://www.anthropic.com/engineering/harness-design-long-running-apps) | 2026-03-24：规划/生成/评估分工，QA 需校准；模型升级后移除强制 reset/固定 sprint | 验收独立、按漏检改进；旧流程随模型升级做消融；展示案例不能等同生产可靠性 |
| [METR 时间跨度局限](https://metr.org/notes/2026-01-22-time-horizon-limitations/) | 2026-01-22：time horizon 是指定成功率对应的人类工时 | 不能理解为 Agent 可连续稳定工作时长，使用 GOGG 自己的验收和介入指标 |
| [Recursive Language Models v3](https://arxiv.org/html/2512.24601v3) | 首发 2025-12-31，更新 2026-05-11：外置输入、程序化读取和递归模型调用 | 按需检索大型材料；代码理解并非完整编码，递归不总有益，有延迟长尾 |
| [ProgramBench](https://arxiv.org/html/2605.03546v1) | 2026-05-05：整程序行为验收，论文实验完整通过极低 | issue 修复能力不能替代整项目维护；有限测试仍不证明完全正确 |

[ProgramBench 官方扩展榜](https://programbench.com/extended/) 标记 2026-08-16
更新，已有非零完整通过；不能继续把早期论文的零完成描述为当前所有模型结果。
本设计引用的是整项目行为验收仍困难，不据排行榜预测 GOGG 成功率。

## 2. 记忆与经验学习

| 来源 | 时间与机制 | 借鉴与局限 |
|---|---|---|
| [ACE v3](https://arxiv.org/html/2510.04618v3) | 首发 2025-10-06，更新 2026-03-29：生成、反思、整理与带 ID 的增量补丁 | 借鉴增量更新；有害反思实验表明可能劣于无记忆，不自动晋升 |
| [DGM v3](https://arxiv.org/html/2505.22954v3) | 首发 2025-05-29，更新 2026-03-12：冻结模型，进化工具/流程 | 借鉴候选档案、离线搜索和迁移评估；有指标投机案例，不能无限在线自改 |
| [GEPA](https://arxiv.org/pdf/2507.19457) | 首发 2025-07-25，v2 2026-02-14：执行反馈与文本候选搜索 | 可选离线技能优化器；需要独立可信 evaluator |
| [SkillRL](https://arxiv.org/html/2602.08234v1) | 2026-02-09：经历到一般/特定技能，并结合参数训练 | 借鉴技能抽象；收益不能归因于几个 SKILL.md，不能直接用于闭源模型权重 |
| [Letta Context Repositories](https://www.letta.com/blog/context-repositories/) | 2026-02-12：Git 管理记忆与按需上下文 | 借鉴分层文件、版本和审计，不为此整体替换 Codex |
| [LongMemEval-V2](https://arxiv.org/abs/2605.12493) | 2026-05-12：历史轨迹文件加 coding agent 收集证据 | 支持文件检索基线；Web 环境预印本、延迟较高，不能外推工程效果 |
| [Mem0](https://arxiv.org/html/2504.19413) | 2025-04-28：提取并决定增删改/不变 | 主要是长对话回忆；LoCoMo 数据量小且排除 adversarial 类别，不证明工程抗污染 |
| [Zep/Graphiti](https://arxiv.org/abs/2501.13956) | 2025-01-20：时序知识关系 | 借鉴何时成立/何时获知、失效保留历史，暂不先引入图数据库 |

当前 [Letta MemFS 文档](https://raw.githubusercontent.com/letta-ai/letta-docs-md/main/concepts/memfs/index.md)
说明常驻和按需文件分层、每 Agent 的记忆，以及默认不包含向量索引。
[共享记忆文档](https://raw.githubusercontent.com/letta-ai/letta-docs-md/main/concepts/shared-memory/index.md)
将共享仓库能力限定于云托管 Agent；不可误认为本地代码具备相同服务能力。

同样，[Zep CE 的维护方向已改变](https://blog.getzep.com/announcing-a-new-direction-for-zeps-open-source-strategy/)，
[Graphiti 与 Zep 托管产品不同](https://help.getzep.com/zep-vs-graphiti)。云产品的模型、
治理和成绩不能全部归到开源本地库。

## 3. 执行框架选择

| 选项 | 判断 | 理由 |
|---|---|---|
| 原生 Codex | 立即用于交互试点 | 已有工具、子 Agent 和会话；初期验证工作协议 |
| Codex SDK + LangGraph | 长期无人值守优先试点 | SDK 执行代码阶段，持久图管理阶段/暂停/恢复；避免自建通用流程引擎 |
| 纯 SDK + 自建控制器 | 仅限边界明确原型 | 队列、取消、分支恢复、幂等和迁移会迅速扩大自建范围 |
| DeepAgents | 对照执行器 | 已有规划、压缩、文件系统和子 Agent，不再与 Codex 重复管理同一循环 |
| OpenHands SDK | 远端工作区或自控 harness 的对照 | 有类型化事件、持久化、容器和远程执行，仍需领域验收 |
| Letta Code | 借鉴记忆布局 | 整体换运行时的收益尚无本项目证据 |
| GEPA | 评价器成熟后的可选依赖 | 搜索技能/文本候选，接受外部 evaluator |
| Mem0 / Graphiti | 暂不作为基础依赖 | 先建立项目事实与版本语义；关系/语义漏检成为瓶颈再评估 |

官方能力来源：[Codex SDK](https://learn.chatgpt.com/docs/codex-sdk)、
[LangGraph Persistence](https://docs.langchain.com/oss/python/langgraph/persistence)、
[LangGraph Interrupts](https://docs.langchain.com/oss/python/langgraph/interrupts)、
[DeepAgents](https://docs.langchain.com/oss/python/deepagents/overview)、
[OpenHands 架构](https://docs.openhands.dev/sdk/arch/overview)、
[OpenHands 持久化](https://docs.openhands.dev/sdk/guides/convo-persistence)。

LangGraph 的节点恢复会重放部分代码，副作用需要幂等或核对；它不自动管理 GOGG
工作区的进程终止、资源租约与候选哈希。若采用 Temporal 作为外层唯一耐久引擎，
则不再叠加 LangGraph 统管同一生命周期。选型是维护风险判断，尚无本地性能对照。
官方 Persistence 文档将 SQLite 定位本地开发，PostgreSQL 定位生产持久化；
据此将 SQLite 限于单宿主试点，长期无人值守需验证生产后端和恢复语义。

## 4. 许可证和采用边界

已核对的官方代码许可：[LangGraph MIT](https://raw.githubusercontent.com/langchain-ai/langgraph/main/LICENSE)、
[Codex Apache-2.0](https://raw.githubusercontent.com/openai/codex/main/LICENSE)、
[ACE Apache-2.0](https://raw.githubusercontent.com/ace-agent/ace/main/LICENSE.txt)、
[GEPA MIT](https://raw.githubusercontent.com/gepa-ai/gepa/main/LICENSE)、
[SkillRL MIT](https://raw.githubusercontent.com/aiming-lab/SkillRL/main/LICENSE)、
[Mem0 Apache-2.0](https://raw.githubusercontent.com/mem0ai/mem0/main/LICENSE)、
[Graphiti Apache-2.0](https://raw.githubusercontent.com/getzep/graphiti/main/LICENSE)。
[Letta Code](https://raw.githubusercontent.com/letta-ai/letta-code/main/LICENSE)
代码采用 Apache-2.0，并明确排除品牌名称、Logo、图片和 ASCII art 等资产。

许可范围不自动覆盖托管服务、模型权重、数据集、后端数据库或所有间接依赖。
真正安装时固定版本并核查该版本 LICENSE 与锁文件；本轮未做全依赖审计。

## 5. 文献维护规则

来源记录包含 URL/DOI、版本、发布日期、核实日期、支持哪项主张及适用边界。
网页功能声明、作者案例、受控实验和本地验证分别标记。
新文献先进入研究候选；涉及决策时复核原文，接受的变化进入 ADR 或知识条目。
维护检查重点在影响当前选择的来源，不每日机械全网扫描。
