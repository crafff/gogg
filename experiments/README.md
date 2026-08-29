# 数据分析与预测实验

这个目录存放不属于线上服务的数据科学实验。每个实验都应当能够独立复现，并至少包含：研究问题、数据口径、避免数据泄漏的方法、运行方式、评估指标、产物说明和结论边界。

## 实验列表

| 实验 | 状态 | 目标 |
|---|---|---|
| [reksai-win-shap](reksai-win-shap/README.md) | 已完成首次运行 | 用 15 分钟前信息预测雷克塞胜负，并用 SHAP 找出与胜利最相关的可行动特征 |
| [champion-recommender](champion-recommender/README.md) | 设计完成 | 基于玩家画像、隐式反馈和版本强度生成个性化英雄推荐 |

全体打野位置的英雄内百分位实验见 [RESULT-JUNGLE-ROLE-20260721.md](reksai-win-shap/RESULT-JUNGLE-ROLE-20260721.md)。

真实数据和模型产物不提交到 Git。实验得到稳定结论后，再把带日期、版本、样本口径和指标的报告提交到对应实验目录。
