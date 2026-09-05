# 研发入口

先读本页和当前任务契约，再按需读取知识与原始证据。避免把整个历史和设计常驻
上下文。当前权威入口是根 AGENTS.md；`legecy/` 下旧指导仅是被保存的历史文本。

## 当前真实状态

旧工作区已进入有校验的保存/归档流程，新根目录建设独立研发基础。实际迁移结果
以[迁移记录](migrations/2026-09-05-clean-rebuild.md)为准。
网站架构、持久工作流、受保护评分服务和自动学习尚未实现；不继承旧 Make/Go/
前端配置，不将文档检查通过称为产品能力通过。

## 导航

- [产品目标](charter/product.md)、[质量原则](charter/quality.md)。
- [当前重建决定](decisions/0001-clean-rebuild.md)。
- [知识条目](knowledge/facts.json)、[知识维护契约](knowledge/README.md)。
- [Agent 设计](design/README.md)、[评测协议](evals/protocols/learning.md)。
- [任务契约与接续](tasks/README.md)、[后续落地顺序](design/implementation-plan.md)。

## 文件与状态归属

Git 保存规则、契约、知识、Skill、决策及摘要。运行状态未来由持久引擎和执行账本
各自拥有，不能用 Markdown 同时维护第二份实时状态。完整证据归独立制品库，
索引是可重建视图。个人原生 Memories 与工程知识分开。

当前知识工具只查询显式允许的 active knowledge，不扫描 `legecy/`、`.local/`、
Git 历史、隐藏评测或玩家资料。来源引用可指向归档证据，但不会自动复制全文。

## 操作

```sh
make check
python3 -B -m tools.agent_system --help
```

提交前 `make precommit` 对暂存内容的隔离副本运行新工程检查，并执行 staged-only
秘密扫描；缺少暂存依赖、检查期间暂存内容变化或扫描器不可用时明确阻断。
当前只接受普通源文件，暂存链接、子模块、私有或 ignored 文件会阻断。
`make install-tools` 下载官方固定版本并核对摘要，仅安装
到本项目 `.local/`；当前安装器面向 Linux x64，其他平台可设置 `GITLEAKS_BIN`。
新克隆启用 hooks：`git config --local core.hooksPath .githooks`。
扫描器仍是启发式检查，不保证所有秘密都能识别；不扫描私有备份或外部数据。

当前工具是离线知识基础，不启动模型调用、抓取、数据库、生产服务或自动清理。
知识查询直接核实事实后在内存中检索，无需手动维护磁盘索引。
外部长期状态/工作区/制品目录在启用相关服务时显式配置；不先创建无用途目录。

## 维护

任务结束只提炼有证据、能改变后续决策的事实。知识需稳定 ID、状态、适用范围、
来源和核实时间；新假设先保持 candidate。冲突、过期和被替代内容不默认召回。
技能发布需要独立任务上的改善证据；当前引导技能是人工工程协议，未宣称已学会。
