# 知识维护契约

当前唯一结构化来源是 `facts.json`，不是整个仓库全文扫描。每条知识以稳定 ID
维护；将来规模扩大可按领域拆分，当前不创建空的知识图或向量数据库。

```json
{
  "schema_version": 1,
  "facts": [{
    "id": "FACT-EXAMPLE",
    "status": "candidate",
    "scope": "current",
    "claim": "一项有明确范围的主张",
    "sources": [],
    "verified_at": null
  }]
}
```

状态：`candidate / verified / stale / superseded / rejected`。
scope：`current` 为当前工程，`legacy` 为明确的归档行为参考，不混用。
本地 source 使用 `{"path":"相对路径","sha256":"实际文件摘要"}`，公共网络来源
使用 `{"url":"https://..."}`，工具不会访问或自动核实网站。
`verified_at` 和可选 `valid_until` 使用带时区的 ISO 8601 时间。

verified 必须有本地哈希证据及核实时间。current 不可直接引用 legecy；legacy
必须有 legecy 来源。把文件放入新工程或给它正确哈希，并不能证明主张为真，语义
核实及接受仍由任务证据和审查负责。

禁止私有/隐藏路径、凭据扩展名、运行数据/备份目录、非公开配置、路径逃逸和
符号链接读取；Git仓库还按实际ignore规则拒绝来源，无Git目录仍执行命名保护。
公开的 `*.example.yaml` 等配置模板可作为证据；来源只读普通文件。
即便引用归档源码也仅核对哈希，不把原文件全文送入索引。研究和失效条目可保留，
默认查询不召回 candidate、失效、过期或另一范围的条目。

```sh
python3 -B -m tools.agent_system check
python3 -B -m tools.agent_system search "Astra"
python3 -B -m tools.agent_system search "打野" --scope legacy
```

工具默认以当前目录为仓库根，也可显式 `--root`。每次查询直接核对权威事实与
来源，并建立临时内存 FTS；无需预建或重建磁盘索引，不读写或删除以前的缓存。
修改并核实事实后，下次查询立即采用。中文按字面子串补充，英文用 FTS，中英文
边界统一处理；不是语义搜索，也未证明适合大规模知识库。

未过期 verified 或 candidate 的来源变化、缺失会阻断并要求重新核实；不能为
消除报错机械刷新哈希。已过期的 verified 或标记 stale/superseded/rejected 的条目仍检查
来源安全边界，来源变化或缺失作为警告返回，并从召回中排除。查询结果的 warnings
供维护者处理，不让这些历史来源阻断无关有效知识。查询前后核对知识与来源，
期间变化则拒绝本次结果；路径、隐私和 schema 错误始终阻断。

维护时依次检查原始来源与范围，修改小条目，运行 check 和实际查询，再独立审查。
工具不会自动修改 Git 知识、原生 Memories、权限或 Skill，不进行垃圾回收。
学习候选另按[评测协议](../evals/protocols/learning.md)证明可泛化改善。
