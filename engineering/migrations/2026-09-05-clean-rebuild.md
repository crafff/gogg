# 保存与全新工程切换

用户于2026-09-04授权先保存全部工作，再彻底重构当前目录；旧文件按指定拼写
放入 `legecy/`，仅作业务参考。实际操作时间为2026-09-05 UTC。

## 保存证据

私有备份：`.local/bootstrap/20260905T001550Z/`，目录权限700，不进入Git或索引。

- 原 HEAD：`303a450ac4bfe8c40c4930a484fb32c57a722fed`，根Git历史与索引保留未改。
- 原树：57,654条，其中51,403普通文件、6,195目录、56符号链接。
- 包含未提交TFT修改、未跟踪源码、旧设计、ignored依赖/产物、Git元数据。
- `workspace.tar.gz`：444,481,392字节，SHA-256：
  `0665b91ca1b70df7842ff29c1122e2b66eb254b3304a3fc6ee7f6d6ab9cf1708`。
- `history.bundle` 通过Git验证；另保存原始status、refs、staged/unstaged二进制diff
  和逐文件内容/权限/链接清单。bundle不单独承担工作树恢复。
- 全量流式校验和迁移前源一致性通过。首次恢复检查发现两个旧venv锁文件的权限
  被默认umask收窄，已改用保留权限恢复；后续完整恢复逐项一致，Git status完全匹配。
- 归档到legecy的51,533条原件一致性通过；旧`.codex`复制归档后才更新根配置，
  根`.git`保留。额外添加的legecy/AGENTS.override.md阻止旧指导被误用。

JSON记录分别为 `verified.json`、`restore-verified.json`、
`relocation-verified.json`、`native-config-verified.json`、`ignore-verified.json`。
这些是本机私有证据，不把完整轨迹或凭据复制到公开文档。

## 忽略和数据边界

原44,959个ignored路径迁入legecy后逐一验证仍被忽略，包含依赖、构建产物、
实验数据链接和依赖全局规则的旧Claude本地设置。新根显式忽略.local和.claude。
所有旧Git tracked和原非ignored未跟踪源码仍保存在归档，未创建提交或推送。

业务数据链接仍指向原外部挂载，未跟随复制、未移动或删除数据库、原始响应、
游戏资产、实验数据和模型制品。工作区备份不能替代外部业务数据备份。
当前备份与工作区同一文件系统，防误操作与代码回退，不提供独立磁盘灾备保证。

## 新入口与已知边界

新根使用AGENTS、engineering、原生专家配置/技能及离线知识工具；旧apps/packages、
Go工作区、Make/CI和部署文件均进入legecy，不提供兼容入口。
`codex doctor --json`实际加载 `gpt-6-astra`，配置解析通过；网络/MCP探测在当前
沙箱受限，不能据此宣称推理链路已验证。当前会话不会因文件改变热切换配置。

模型迁移保持xhigh推理、workspace-write、on-request、auto_review；实验性上下文
功能未擅自启用。[官方模型说明](https://learn.chatgpt.com/docs/models)、
[自定义Agent配置](https://learn.chatgpt.com/docs/agent-configuration/subagents)。

网站运行时、持久工作流/执行账本、独立评分服务、长期制品库和自动学习尚未实现。
新根基础检查和工具单元测试不构成网站功能、性能或长期无人值守的完成证据。

独立审查发现根Git仍有可执行的旧Lefthook入口。旧hooks保留在原Git及完整备份，
本地 `core.hooksPath` 改指新 `.githooks`，仅改变本地配置，HEAD/refs/index不变。
新pre-commit使用当前make check和staged-only秘密扫描，不运行旧Go/Web工具。
Gitleaks固定8.30.1，下载包核对官方SHA-256；不继承旧扫描豁免文件。
[官方发布及校验和](https://github.com/gitleaks/gitleaks/releases/expanded_assets/v8.30.1)。

## 恢复方法

先校验压缩包摘要，再选择一个新建的私有空目录。使用保留权限、不跟随链接的
tar恢复，并核对inventory及Git状态；不要直接解压覆盖当前根，不自动启动旧服务。
精确复原步骤与已用脚本保存在本机私有备份旁。旧venv或依赖里的绝对路径不保证
在新位置可执行；需要运行历史环境时另外构建隔离环境。

后续清理只适用于已授权的具体副本或可重建缓存，绝不删除唯一备份及外部数据。
本轮两份临时恢复演练副本在核验后已清理；完整压缩备份、恢复记录和legecy原件
保留。需要再次演练时从校验过的压缩包恢复到新的私有目录。
