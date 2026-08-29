#!/usr/bin/env python3
"""Train a grouped LightGBM classifier and explain its held-out predictions."""

import argparse
import json
from pathlib import Path

import lightgbm as lgb
import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
import shap
from sklearn.dummy import DummyClassifier
from sklearn.metrics import average_precision_score, brier_score_loss, roc_auc_score
from sklearn.model_selection import GroupShuffleSplit

from features import FEATURES, add_features


def metrics(y_true, probability):
    return {
        "roc_auc": float(roc_auc_score(y_true, probability)),
        "pr_auc": float(average_precision_score(y_true, probability)),
        "brier": float(brier_score_loss(y_true, probability)),
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", type=Path, required=True)
    parser.add_argument("--output-dir", type=Path, default=Path("artifacts/latest"))
    parser.add_argument("--test-size", type=float, default=0.2)
    parser.add_argument("--seed", type=int, default=421)
    parser.add_argument("--min-rows", type=int, default=200)
    args = parser.parse_args()

    raw = pd.read_csv(args.input)
    required = set(FEATURES) | {"win", "puuid", "match_id"}
    engineered = add_features(raw)
    missing = sorted(required - set(engineered.columns))
    if missing:
        raise SystemExit(f"missing columns: {missing}")
    clean = engineered.dropna(subset=FEATURES + ["win", "puuid"]).copy()
    if len(clean) < args.min_rows:
        raise SystemExit(f"only {len(clean)} complete rows; need at least {args.min_rows}")
    clean["win"] = clean["win"].astype(str).str.lower().map({"true": 1, "false": 0, "1": 1, "0": 0})
    if clean["win"].isna().any() or clean["win"].nunique() != 2:
        raise SystemExit("win must contain both true and false labels")

    splitter = GroupShuffleSplit(n_splits=1, test_size=args.test_size, random_state=args.seed)
    development_idx, test_idx = next(splitter.split(clean, clean["win"], groups=clean["puuid"]))
    development, test = clean.iloc[development_idx], clean.iloc[test_idx]
    validation_splitter = GroupShuffleSplit(n_splits=1, test_size=0.2, random_state=args.seed + 1)
    train_idx, validation_idx = next(validation_splitter.split(
        development, development["win"], groups=development["puuid"]
    ))
    train, validation = development.iloc[train_idx], development.iloc[validation_idx]
    x_train, y_train = train[FEATURES], train["win"]
    x_validation, y_validation = validation[FEATURES], validation["win"]
    x_test, y_test = test[FEATURES], test["win"]
    if min(y_train.nunique(), y_validation.nunique(), y_test.nunique()) != 2:
        raise SystemExit("a grouped split contains only one outcome; collect more data or change --seed")

    model = lgb.LGBMClassifier(
        objective="binary", n_estimators=500, learning_rate=0.03,
        num_leaves=15, max_depth=5, min_child_samples=30,
        subsample=0.8, colsample_bytree=0.8, reg_lambda=1.0,
        random_state=args.seed, n_jobs=-1, verbosity=-1,
    )
    callbacks = [lgb.early_stopping(40, verbose=False)]
    model.fit(x_train, y_train, eval_set=[(x_validation, y_validation)], callbacks=callbacks)
    probability = model.predict_proba(x_test)[:, 1]
    dummy = DummyClassifier(strategy="prior").fit(x_train, y_train)
    dummy_probability = dummy.predict_proba(x_test)[:, 1]

    args.output_dir.mkdir(parents=True, exist_ok=True)
    report = {
        "rows_input": len(raw), "rows_complete": len(clean),
        "train_rows": len(train), "validation_rows": len(validation), "test_rows": len(test),
        "train_players": int(train["puuid"].nunique()),
        "validation_players": int(validation["puuid"].nunique()),
        "test_players": int(test["puuid"].nunique()),
        "train_win_rate": float(y_train.mean()),
        "validation_win_rate": float(y_validation.mean()),
        "test_win_rate": float(y_test.mean()),
        "best_iteration": int(model.best_iteration_),
        "dummy": metrics(y_test, dummy_probability),
        "lightgbm": metrics(y_test, probability),
        "split": "group_shuffle_by_puuid",
        "seed": args.seed,
    }
    (args.output_dir / "metrics.json").write_text(
        json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    model.booster_.save_model(args.output_dir / "model.txt")

    predictions = test[["match_id", "puuid", "win"]].copy()
    predictions["win_probability"] = probability
    predictions.to_csv(args.output_dir / "test_predictions.csv", index=False)

    explainer = shap.TreeExplainer(model)
    explanation = explainer(x_test)
    values = explanation.values
    importance = pd.DataFrame({
        "feature": FEATURES,
        "mean_abs_shap": np.abs(values).mean(axis=0),
        "mean_shap_wins": values[y_test.to_numpy() == 1].mean(axis=0),
        "mean_shap_losses": values[y_test.to_numpy() == 0].mean(axis=0),
    }).sort_values("mean_abs_shap", ascending=False)
    importance.to_csv(args.output_dir / "feature_importance.csv", index=False)

    shap.summary_plot(explanation, x_test, show=False, max_display=15)
    plt.tight_layout()
    plt.savefig(args.output_dir / "shap_summary.png", dpi=180, bbox_inches="tight")
    plt.close()
    for position in np.argsort(np.abs(probability - 0.5))[-3:]:
        shap.plots.waterfall(explanation[position], max_display=12, show=False)
        plt.savefig(args.output_dir / f"shap_waterfall_{test.iloc[position]['match_id']}.png", dpi=180, bbox_inches="tight")
        plt.close()
    print(json.dumps(report, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
