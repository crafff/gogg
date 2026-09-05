# 个性化英雄推荐实验

目标：从玩家历史对局构建可解释画像，结合英雄适配度、当前版本强度和推荐置信度，为玩家生成适合其位置与风格的英雄列表。

这不是“把玩家聚类后给每类固定英雄”。聚类只用于解释和冷启动；主推荐必须保留玩家级差异，并采用时间正确的训练与评估。

详细设计与实施阶段见 [PLAN.md](PLAN.md)。

Stage 0–2 首轮实现与结果见 [RESULT-MVP-20260721.md](RESULT-MVP-20260721.md)。
Stage 1/3 玩家画像与混合排序结果见 [RESULT-HYBRID-20260721.md](RESULT-HYBRID-20260721.md)。
时间线特征消融与多样性实验见 [RESULT-TIMELINE-ABLATION-20260721.md](RESULT-TIMELINE-ABLATION-20260721.md)。
推荐预计算、槽位与服务契约见 [RESULT-PRECOMPUTE-20260721.md](RESULT-PRECOMPUTE-20260721.md)。

## 核心架构

```text
历史对局/时间线 ──> 玩家时点画像 ─┐
                                  ├─> 全英雄打分 ─> 约束与版本重排 ─> Top-K + 推荐理由
英雄属性/玩法画像 ────────────────┤
玩家×英雄隐式反馈 ──> CF embedding ┘
版本/分段胜率 ────────────────────────────────────────┘
```

英雄总数很小，MVP直接对请求位置的全部英雄评分，不引入ANN或向量数据库。

## 成功标准

推荐成功不能只定义为“猜中玩家下一局会选什么”，否则系统只会推荐他已经常玩的英雄。需要同时衡量：

- 采纳：推荐后是否尝试；
- 留存：7/14天内是否继续使用该英雄；
- 适配：相对玩家水平、英雄基线和对局预期，表现是否改善；
- 多样性：是否发现新的可用英雄，而非重复热门英雄；
- 安全性：不向没有足够样本的组合展示虚假精确推荐。

## 当前运行方式

```bash
cd experiments/champion-recommender
UV_CACHE_DIR=/tmp/gogg-uv-cache uv venv .venv
UV_CACHE_DIR=/tmp/gogg-uv-cache uv pip install --python .venv/bin/python -r requirements.txt

export DATABASE_URL='postgresql://gogg:...@localhost:55433/gogg?sslmode=disable'
.venv/bin/python export_interactions.py --output data/jungle_interactions.csv
MPLCONFIGDIR=/tmp/gogg-matplotlib .venv/bin/python audit.py \
  --input data/jungle_interactions.csv --output-dir artifacts/audit
.venv/bin/python baselines.py \
  --input data/jungle_interactions.csv --output-dir artifacts/baselines
.venv/bin/python profiles.py \
  --input data/jungle_interactions.csv --output-dir artifacts/profiles --min-games 20
.venv/bin/python hybrid_ranker.py \
  --input data/jungle_interactions.csv --output-dir artifacts/hybrid-ranker
.venv/bin/python precompute.py \
  --input data/jungle_timeline_interactions.csv \
  --model artifacts/timeline-hybrid-ranker/model.txt \
  --output-dir artifacts/precomputed
```
