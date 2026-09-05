# GOGG

面向长期演进的游戏数据与玩家成长产品，现处于全新工程基础建设阶段。

目标：保全已有 LoL/TFT 用户能力，先做到单机稳定、高性能、低资源占用，再验证
多机；逐步建设可靠采集、可重现历史数据、个性化推荐、取胜因素分析与提升反馈。
技术栈与旧实现没有兼容约束，选择依据是可验证的质量和维护收益。

## 从这里开始

- [研发入口](engineering/README.md)：当前状态、工作协议与知识导航。
- [产品目标](engineering/charter/product.md)：保全范围和新的产品目标。
- [设计总览](engineering/design/README.md)：Agent、存储、学习及后续落地设计。
- [迁移记录](engineering/migrations/2026-09-05-clean-rebuild.md)：保存、归档与恢复边界。

```sh
make check
make test
```

当前工具要求 Linux/WSL、Python 3.11+（SQLite含FTS5）、Git和GNU Make；本机验证环境
为Python 3.12.3。离线知识工具无第三方Python依赖。

当前提供研发指导、知识工具与验证基础；尚未提供新网站、LangGraph 运行服务或
自动学习服务。不要从 `legecy/` 启动旧服务来冒充新系统已经运行。

## 目录

```text
.codex/                 生效于新会话的模型和专家配置
.agents/skills/         可复用的研发流程
engineering/            目标、知识、契约、决策、评测和设计
tools/                  当前工程工具
tests/                  当前工程工具的行为测试
legecy/                 原工作区，业务逻辑和历史研究参考
.local/                 私有备份/恢复记录，不进入 Git 或检索
```

Git 历史保留在根 `.git/`。旧代码移到 `legecy/`，旧构建、CI 和配置不再是新工程
的入口。保存点和归档边界见迁移记录，外部数据库及历史数据保持原地。
