# 雷克塞胜负预测与 SHAP 解释

> 方向更新：首次实验验证了局势预测，但经验/经济差不够可操作。面向玩家训练建议的新方案见 [ACTIONABLE-DESIGN.md](ACTIONABLE-DESIGN.md)。

使用当前已有数据完成的首轮可操作指标结果见 [RESULT-ACTIONABLE-20260721.md](RESULT-ACTIONABLE-20260721.md)。

## 研究问题

使用排位赛中雷克塞打野在 10、15 分钟时可观察到的数据预测最终胜负，并回答：哪些早期、可由玩家影响的指标与胜利预测关系最强？目标是形成训练方向，而不是做赛后“数据好看所以赢了”的同义反复。

默认不使用终局击杀、KDA、总经济、推塔、龙等字段。这些字段发生在预测时点之后，会造成目标泄漏。SHAP 解释模型学到的关联，不证明某项操作会导致胜利；最终建议仍需要分段、版本复验或因果实验支持。

## 数据口径

- 英雄：雷克塞（champion_id = 421）
- 位置：JUNGLE
- 队列：召唤师峡谷单双排（queue_id = 420）
- 时点：第 15 分钟；对局必须至少持续 15 分钟
- 特征：个人 10/15 分钟经济、野怪、等级、经验、伤害构成、承伤、控制、视野和位置变化；10→15 分钟增量；相对敌方打野的差值；15 分钟前的装备购买行为与技能加点
- 标签：雷克塞所在方最终是否胜利
- 切分：按 `puuid` 分组为训练/验证/测试集，避免同一个玩家跨集合；验证集只用于早停，最终测试集不参与训练决策

SQL 会要求雷克塞和敌方打野都具有 10、15 分钟快照。缺失快照的对局不会静默填零，而是排除并在训练日志中报告。

## 运行

需要 Python 3.11+、可访问本项目 PostgreSQL 的 `DATABASE_URL`，以及已经抓取的时间线数据。

```bash
cd experiments/reksai-win-shap
python -m venv .venv
. .venv/bin/activate
pip install -r requirements.txt

export DATABASE_URL='postgresql://gogg:...@localhost:5432/gogg'
python export_data.py --output data/reksai_15m.csv
python train.py --input data/reksai_15m.csv --output-dir artifacts/latest
```

也可以用 `psql` 直接执行 `extract.sql`。导出 SQL 支持变量 `queue_id`、`cutoff_minute`、`min_game_start` 和 `max_game_start`；`export_data.py` 会安全地绑定这些参数。

快速验证特征工程：

```bash
python -m unittest discover -s tests -v
```

## 产物

`artifacts/latest/` 中会生成：

- `metrics.json`：样本口径、类别比例、Dummy 基线与 LightGBM 指标
- `feature_importance.csv`：按平均绝对 SHAP 值排序，并包含胜/负样本的平均 SHAP 方向
- `shap_summary.png`：全局重要性及高低取值方向
- `shap_waterfall_*.png`：测试集若干单局解释
- `model.txt`：LightGBM 模型
- `test_predictions.csv`：留出集预测，方便误差分析

首次扩展特征实验的对比与结论见 [RESULT-EXPANDED-20260721.md](RESULT-EXPANDED-20260721.md)。

至少 200 局才允许运行，正式形成玩家建议前建议满足：多个版本均有样本、测试集至少数百局，并针对不同分段分别复验。不要仅凭 SHAP 排名写成“提升 X 就一定能赢”。
