#!/usr/bin/env python3
"""Discover actionable 10/15-minute candidate thresholds with additive EBMs."""

import argparse
import json
from pathlib import Path

import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
from interpret.glassbox import ExplainableBoostingClassifier
from sklearn.metrics import accuracy_score, average_precision_score, brier_score_loss, roc_auc_score
from sklearn.model_selection import GroupShuffleSplit

from features import add_features


FEATURE_SETS = {
    "10m": [
        "jungle_cs_10", "lane_cs_10", "dmg_to_champs_10", "dmg_taken_10",
        "time_enemy_cc_10", "wards_placed_10", "wards_killed_10",
    ],
    "15m": [
        "jungle_cs_10", "lane_cs_10", "dmg_to_champs_10", "dmg_taken_10",
        "time_enemy_cc_10", "wards_placed_10", "wards_killed_10",
        "jungle_cs_gain_10_15", "lane_cs_gain_10_15", "dmg_to_champs_gain_10_15",
        "dmg_taken_gain_10_15", "time_enemy_cc_gain_10_15",
        "wards_placed_gain_10_15", "wards_killed_gain_10_15",
        "item_purchases_15", "distinct_items_15", "item_undos_15", "items_sold_15",
        "first_purchase_minute", "last_purchase_minute",
        "e_priority_over_q_15", "w_extra_points_15",
    ],
}


def grouped_split(frame, seed):
    outer = GroupShuffleSplit(n_splits=1, test_size=0.2, random_state=seed)
    development_idx, test_idx = next(outer.split(frame, frame.win, frame.puuid))
    development, test = frame.iloc[development_idx], frame.iloc[test_idx]
    inner = GroupShuffleSplit(n_splits=1, test_size=0.2, random_state=seed + 1)
    train_idx, validation_idx = next(inner.split(development, development.win, development.puuid))
    return development.iloc[train_idx], development.iloc[validation_idx], test


def threshold_table(frame, feature, thresholds):
    rows = []
    for threshold in thresholds:
        low, high = frame[frame[feature] < threshold], frame[frame[feature] >= threshold]
        if min(len(low), len(high)) < 100:
            continue
        rows.append({
            "feature": feature, "threshold": threshold,
            "below_n": len(low), "above_n": len(high),
            "below_win_rate": low.win.mean(), "above_win_rate": high.win.mean(),
            "raw_win_rate_difference_pp": 100 * (high.win.mean() - low.win.mean()),
        })
    return rows


def run_model(frame, name, features, output, seed):
    columns = features + ["region", "game_version", "win", "puuid", "match_id"]
    data = frame[columns].dropna().copy()
    data["region"] = data.region.astype("category")
    data["game_version"] = data.game_version.astype("category")
    train, validation, test = grouped_split(data, seed)
    model_features = features + ["region", "game_version"]
    feature_types = ["continuous"] * len(features) + ["nominal", "nominal"]
    model = ExplainableBoostingClassifier(
        feature_names=model_features, feature_types=feature_types,
        interactions=0, max_bins=64, max_rounds=5000,
        learning_rate=0.03, validation_size=0.15, early_stopping_rounds=100,
        outer_bags=8, random_state=seed, n_jobs=-1,
    )
    # EBM owns an internal early-stopping split. The external validation set is
    # retained for threshold selection; test remains untouched for reporting.
    model.fit(train[model_features], train.win)
    probability = model.predict_proba(test[model_features])[:, 1]
    metrics = {
        "rows": len(data), "train_rows": len(train),
        "validation_rows": len(validation), "test_rows": len(test),
        "accuracy_at_0_5": float(accuracy_score(test.win, probability >= 0.5)),
        "roc_auc": float(roc_auc_score(test.win, probability)),
        "pr_auc": float(average_precision_score(test.win, probability)),
        "brier": float(brier_score_loss(test.win, probability)),
    }
    output.mkdir(parents=True, exist_ok=True)
    (output / "metrics.json").write_text(json.dumps(metrics, indent=2) + "\n")

    global_explanation = model.explain_global()
    importance = pd.DataFrame({
        "feature": global_explanation.data()["names"],
        "mean_abs_score": global_explanation.data()["scores"],
    }).sort_values("mean_abs_score", ascending=False)
    importance.to_csv(output / "importance.csv", index=False)

    plot_features = [f for f in features if f in set(importance.head(10).feature)]
    fig, axes = plt.subplots((len(plot_features) + 1) // 2, 2, figsize=(13, 3.4 * ((len(plot_features) + 1) // 2)))
    axes = np.atleast_1d(axes).ravel()
    for axis, feature in zip(axes, plot_features):
        term = global_explanation.data(model_features.index(feature))
        names, scores = np.asarray(term["names"]), np.asarray(term["scores"])
        # Continuous term names are cut points; scores can include an extra bin.
        size = min(len(names), len(scores))
        axis.plot(names[:size], scores[:size], marker=".", linewidth=1)
        axis.axhline(0, color="grey", linewidth=0.8)
        axis.set(title=feature, ylabel="EBM log-odds contribution")
    for axis in axes[len(plot_features):]:
        axis.set_visible(False)
    fig.tight_layout()
    fig.savefig(output / "action_curves.png", dpi=180)
    plt.close(fig)

    candidates = {
        "jungle_cs_10": range(40, 81, 5),
        "lane_cs_10": range(0, 21, 5),
        "wards_placed_10": range(1, 9),
        "wards_killed_10": range(1, 6),
        "jungle_cs_gain_10_15": range(15, 51, 5),
        "lane_cs_gain_10_15": range(0, 21, 5),
        "item_purchases_15": range(8, 17),
        "last_purchase_minute": range(8, 16),
    }
    rows = []
    # Candidate discovery uses validation only, never the final test set.
    for feature, thresholds in candidates.items():
        if feature in features:
            rows.extend(threshold_table(validation, feature, thresholds))
    pd.DataFrame(rows).to_csv(output / "candidate_thresholds.csv", index=False)
    return metrics, importance


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", type=Path, required=True)
    parser.add_argument("--output-dir", type=Path, default=Path("artifacts/actionable-ebm"))
    parser.add_argument("--seed", type=int, default=421)
    args = parser.parse_args()
    frame = add_features(pd.read_csv(args.input))
    frame["win"] = frame.win.astype(str).str.lower().map({"true": 1, "false": 0, "1": 1, "0": 0})
    reports = {}
    for name, features in FEATURE_SETS.items():
        metrics, importance = run_model(frame, name, features, args.output_dir / name, args.seed)
        reports[name] = metrics
        print(name, json.dumps(metrics))
        print(importance.head(10).to_string(index=False))
    (args.output_dir / "metrics.json").write_text(json.dumps(reports, indent=2) + "\n")


if __name__ == "__main__":
    main()
