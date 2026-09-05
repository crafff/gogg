# 性能实验记录

每项实验创建独立的 `EXP-YYYYMMDD-NN.md`，并在
[`../README.md`](../README.md) 的实验索引中登记。原始输出保存在同 ID 的
`tmp/performance/<experiment>/` 下。

## 模板

```markdown
# EXP-YYYYMMDD-NN：实验标题

- 所属阶段：
- 状态：计划中 / 进行中 / 接受 / 拒绝 / 需要更多数据
- 日期：
- 负责人：
- 假设：
- 成功条件：
- 停止条件：
- 回滚方式：

## 环境

- baseline commit / image / dirty：
- candidate commit / image / dirty：
- dirty 差异说明：
- dataset ID / 表规模：
- API 场景和参数：
- 机器和资源：

## 修改内容

## 执行步骤

## 原始运行

- baseline：run-01、run-02、run-03
- candidate：run-01、run-02、run-03

## 结果

| 指标 | baseline 中位数 | candidate 中位数 | 变化 | 判定 |
|---|---:|---:|---:|---|
| RPS | | | | |
| p50 | | | | |
| p95 | | | | |
| p99 | | | | |
| 5xx | | | | |
| CPU | | | | |
| 内存 | | | | |
| DB mean/max | | | | |
| Redis hit rate | | | | |

## 正确性验证

## 结论

## 遗留风险与下一步
```
